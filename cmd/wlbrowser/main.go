package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	gdk "github.com/diamondburned/gotk4/pkg/gdk/v4"
	gio "github.com/diamondburned/gotk4/pkg/gio/v2"
	glib "github.com/diamondburned/gotk4/pkg/glib/v2"
	gtk "github.com/diamondburned/gotk4/pkg/gtk/v4"

	webkit "github.com/diamondburned/gotk4-webkitgtk/pkg/webkit/v6"

	"github.com/dnslee/wlbrowser/internal/actions"
	"github.com/dnslee/wlbrowser/internal/cmdbar"
	"github.com/dnslee/wlbrowser/internal/config"
	"github.com/dnslee/wlbrowser/internal/dom"
	"github.com/dnslee/wlbrowser/internal/find"
	"github.com/dnslee/wlbrowser/internal/history"
	"github.com/dnslee/wlbrowser/internal/keys"
	"github.com/dnslee/wlbrowser/internal/mode"
)

func main() {
	fmt.Fprintln(os.Stderr, "DEBUG: main starting")
	writeCfg := flag.Bool("write-config", false, "write default config to the standard location and exit")
	setDefault := flag.Bool("set-default", false, "set as default browser and exit")
	cfgPath := flag.String("config", "", "path to config.toml (default: $XDG_CONFIG_HOME/wlbrowser/config.toml)")
	flag.Parse()

	if *writeCfg {
		if err := config.WriteDefault(*cfgPath); err != nil {
			log.Fatal(err)
		}
		path := *cfgPath
		if path == "" {
			path, _ = config.DefaultPath()
		}
		fmt.Printf("wrote default config to %s\n", path)
		return
	}

	if *setDefault {
		if err := config.EnsureDefaultBrowser(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("successfully set wlbrowser as default browser")
		return
	}

	var globalCfg *config.Config
	cfgReady := make(chan struct{})
	histChan := make(chan *history.Store, 1)

	go func() {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			log.Printf("failed to load config: %v", err)
			cfg, _ = config.Load("") // try default if path failed, though Load handles it
		}
		globalCfg = cfg
		close(cfgReady)
	}()

	go func() {
		histStore, err := history.New()
		if err != nil {
			log.Printf("failed to open history db: %v", err)
			histChan <- nil
		} else {
			histChan <- histStore
		}
	}()

	app := gtk.NewApplication("dev.wlbrowser", gio.ApplicationHandlesCommandLine)

	app.ConnectStartup(setupApp)

	var globalHist *history.Store
	histReady := make(chan struct{})

	// Monitor histChan to capture the store for shutdown and subsequent activations
	go func() {
		globalHist = <-histChan
		close(histReady)
	}()

	app.ConnectCommandLine(func(cmd *gio.ApplicationCommandLine) int {
		args := cmd.Arguments()

		<-cfgReady
		cfg := globalCfg

		if cfg.SetDefault {
			go func() {
				if err := config.EnsureDefaultBrowser(); err != nil {
					log.Printf("failed to set as default browser: %v", err)
				}
			}()
		}

		target := cfg.Home
		fs := flag.NewFlagSet("wlbrowser", flag.ContinueOnError)
		fs.Bool("write-config", false, "")
		fs.Bool("set-default", false, "")
		fs.String("config", "", "")
		if err := fs.Parse(args[1:]); err == nil {
			if fs.NArg() > 0 {
				target = fs.Arg(0)
			}
		}

		activate(app, cfg, histReady, &globalHist, target)
		return 0
	})

	app.ConnectShutdown(func() {
		if globalHist != nil {
			globalHist.Close()
		}
	})

	if code := app.Run(os.Args); code != 0 {
		os.Exit(code)
	}
}

func setupApp() {
	// Configure session persistence
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".local", "share", "wlbrowser")
	os.MkdirAll(dataDir, 0755)

	session := webkit.NetworkSessionGetDefault()
	cookieMgr := session.CookieManager()
	cookieMgr.SetPersistentStorage(filepath.Join(dataDir, "cookies.db"), webkit.CookiePersistentStorageSqlite)
	cookieMgr.SetAcceptPolicy(webkit.CookiePolicyAcceptAlways)

	css := gtk.NewCSSProvider()
	css.LoadFromData(`
		.rich-list { 
			background: #ffffff;
			color: #000000;
			border: 1px solid #cccccc;
			border-top: none;
			border-radius: 0 0 8px 8px;
			box-shadow: 0 4px 12px rgba(0,0,0,0.4);
		}
		/* Force white background on the internal list widget */
		.rich-list list {
			background: #ffffff;
			background-color: #ffffff;
		}
		/* Style the rows explicitly with white background and black text */
		.rich-list row {
			padding: 6px;
			background: #ffffff;
			background-color: #ffffff;
			color: #000000;
		}
		.rich-list row label {
			color: #000000;
		}
		.rich-list row:hover, .rich-list row:selected {
			background: #3584e4;
			color: #ffffff;
		}
		.rich-list row:hover label, .rich-list row:selected label {
			color: #ffffff;
		}
		.dim-label { opacity: 0.7; font-size: 0.85em; color: #555555; }
		.domain-header { 
			background: #f0f0f0; 
			padding: 4px 8px; 
			border-radius: 4px;
			margin-bottom: 4px;
		}
		.domain-title { font-weight: bold; color: #333333; }
	`)
	gtk.StyleContextAddProviderForDisplay(gdk.DisplayGetDefault(), css, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

func activate(app *gtk.Application, cfg *config.Config, histReady <-chan struct{}, globalHist **history.Store, targetURL string) {
	fmt.Fprintln(os.Stderr, "DEBUG: activating")
	win := gtk.NewApplicationWindow(app)
	win.SetTitle(cfg.Title)
	win.SetDefaultSize(cfg.Width, cfg.Height)

	view := webkit.NewWebView()
	view.SetVExpand(true)
	view.SetHExpand(true)

	settings := view.Settings()
	settings.SetEnableHtml5LocalStorage(true)
	settings.SetEnableHtml5Database(true)
	view.SetSettings(settings)

	bar := cmdbar.New()
	bar.AttachEscape()

	overlay := gtk.NewOverlay()

	// Suggestions box will contain the suggestions list, positioned below the entry
	bar.Scrolled.SetVAlign(gtk.AlignStart)
	bar.Scrolled.AddCSSClass("rich-list")
	
	overlay.SetChild(view)
	
	// Create a container for the bar and suggestions that sits on top of the view
	barBox := gtk.NewBox(gtk.OrientationVertical, 0)
	barBox.SetVAlign(gtk.AlignStart)
	barBox.Append(bar.Entry)
	barBox.Append(bar.Scrolled)
	
	overlay.AddOverlay(barBox)

	win.SetChild(overlay)

	findCtrl := find.New(view)

	var inInsertMode atomic.Bool

	sched := mode.RealScheduler{
		RunOnMain: func(f func()) { glib.IdleAdd(func() bool { f(); return false }) },
	}

	disp := &actions.Dispatcher{
		View:   view,
		Window: &win.Window,
		Bar:    bar,
		Find:   findCtrl,
		Home:   cfg.Home,
		Config: cfg,
		Hist:   nil, // Loaded asynchronously
	}

	// Load history in the background and update dispatcher when ready
	go func() {
		<-histReady
		h := *globalHist
		glib.IdleAdd(func() bool {
			disp.Hist = h
			return false
		})
	}()

	machine, err := mode.New(
		cfg.Keys["normal"],
		time.Duration(cfg.SeqTimeoutMs)*time.Millisecond,
		sched,
		disp,
	)
	if err != nil {
		log.Fatal(err)
	}
	disp.Machine = machine

	if err := dom.RegisterFocusTracker(view.UserContentManager(), func(insert bool) {
		inInsertMode.Store(insert)
		// Don't override Command mode; the cmdbar callbacks restore mode on close.
		if bar.Entry.HasFocus() {
			return
		}
		if insert {
			machine.SetMode(mode.ModeInsert)
		} else {
			machine.SetMode(mode.ModeNormal)
			bar.Hide()
		}
	}); err != nil {
		log.Fatal(err)
	}

	bar.OnChange = func(text string) {
		if strings.HasPrefix(text, "/") {
			bar.Scrolled.SetVisible(false)
			findCtrl.Start(text[1:])
		} else if disp.Hist != nil && text != "" && !strings.HasPrefix(text, ":") {
			results := disp.Hist.Search(text)
			var suggestions []cmdbar.Suggestion
			for _, r := range results {
				suggestions = append(suggestions, cmdbar.Suggestion{
					Text: r.URL,
					Desc: r.Title,
				})
			}
			bar.UpdateSuggestions(suggestions)
		} else {
			bar.Scrolled.SetVisible(false)
		}
	}
	bar.OnSubmit = func(text string) {
		if strings.HasPrefix(text, "/") {
			// For '/', findCtrl.Start was already called via OnChange; submit
			// just keeps the highlight and returns to Normal.
		} else {
			// Treat as URL or search
			if !strings.Contains(text, "://") && !strings.HasPrefix(text, "localhost:") && (strings.Contains(text, " ") || !strings.Contains(text, ".")) {
				view.LoadURI("https://duckduckgo.com/?q=" + url.QueryEscape(text))
			} else if !strings.Contains(text, "://") {
				view.LoadURI("https://" + text)
			} else {
				view.LoadURI(text)
			}
		}
		view.GrabFocus()
		restoreMode(machine, &inInsertMode)
	}
	bar.OnCancel = func() {
		findCtrl.Cancel()
		view.GrabFocus()
		restoreMode(machine, &inInsertMode)
	}

	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.SetPropagationPhase(gtk.PhaseCapture)
	keyCtrl.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		if bar.Entry.HasFocus() || bar.List.HasFocus() {
			return false
		}
		return machine.Feed(keys.Normalize(state, keyval))
	})
	win.AddController(keyCtrl)

	view.ConnectLoadChanged(func(loadEvent webkit.LoadEvent) {
		switch loadEvent {
		case webkit.LoadStarted:
			bar.Hide()
		case webkit.LoadCommitted:
			uri := view.URI()
			if !bar.Entry.HasFocus() {
				bar.SetText(uri)
			}
			if disp.Hist != nil && uri != "" {
				disp.Hist.Add(uri, view.Title())
			}
		}
	})

	view.LoadURI(targetURL)
	win.SetVisible(true)
}

func restoreMode(m *mode.Machine, insert *atomic.Bool) {
	if insert.Load() {
		m.SetMode(mode.ModeInsert)
	} else {
		m.SetMode(mode.ModeNormal)
	}
}
