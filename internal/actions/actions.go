// Package actions implements every concrete side effect a key binding can
// trigger. The Dispatcher is the bridge between the mode machine (which
// produces config.Action values) and the live WebView/cmdbar/find/process
// surface.
package actions

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"strings"

	gtk "github.com/diamondburned/gotk4/pkg/gtk/v4"

	webkit "github.com/diamondburned/gotk4-webkitgtk/pkg/webkit/v6"
	"github.com/diamondburned/gotk4/pkg/pango"

	"github.com/dnslee/wlbrowser/internal/cmdbar"
	"github.com/dnslee/wlbrowser/internal/config"
	"github.com/dnslee/wlbrowser/internal/find"
	"github.com/dnslee/wlbrowser/internal/history"
	"github.com/dnslee/wlbrowser/internal/mode"
)

// Dispatcher carries every reference an action might need. It is the single
// thing main.go hands to the mode machine.
type Dispatcher struct {
	View    *webkit.WebView
	Window  *gtk.Window
	Bar     *cmdbar.Bar
	Find    *find.Controller
	Machine *mode.Machine
	Home    string
	Config  *config.Config
	Hist    *history.Store
}

// Run is the entry point invoked by mode.Machine when a binding fires.
func (d *Dispatcher) Run(a config.Action, count int) {
	if count <= 0 {
		count = 1
	}
	switch a.Name {
	// ─── Navigation ────────────────────────────────────────────────────
	case "back":
		for i := 0; i < count && d.View.CanGoBack(); i++ {
			d.View.GoBack()
		}
	case "forward":
		for i := 0; i < count && d.View.CanGoForward(); i++ {
			d.View.GoForward()
		}
	case "reload":
		d.View.Reload()
	case "reload-hard":
		d.View.ReloadBypassCache()
	case "stop":
		d.View.StopLoading()
	case "home":
		if d.Home != "" {
			d.View.LoadURI(d.Home)
		}
	case "open-url":
		if a.URL != "" {
			d.View.LoadURI(a.URL)
		}

	// ─── Scrolling / zoom (delegated to JS — WebKit has no native API) ─
	case "scroll-down":
		evalJS(d.View, fmt.Sprintf("window.scrollBy(0, %d * 60)", count))
	case "scroll-up":
		evalJS(d.View, fmt.Sprintf("window.scrollBy(0, -%d * 60)", count))
	case "scroll-left":
		evalJS(d.View, fmt.Sprintf("window.scrollBy(-%d * 60, 0)", count))
	case "scroll-right":
		evalJS(d.View, fmt.Sprintf("window.scrollBy(%d * 60, 0)", count))
	case "scroll-half-page-down":
		evalJS(d.View, fmt.Sprintf("window.scrollBy(0, %d * window.innerHeight / 2)", count))
	case "scroll-half-page-up":
		evalJS(d.View, fmt.Sprintf("window.scrollBy(0, -%d * window.innerHeight / 2)", count))
	case "scroll-page-down":
		evalJS(d.View, fmt.Sprintf("window.scrollBy(0, %d * window.innerHeight)", count))
	case "scroll-page-up":
		evalJS(d.View, fmt.Sprintf("window.scrollBy(0, -%d * window.innerHeight)", count))
	case "scroll-top":
		evalJS(d.View, "window.scrollTo(0, 0)")
	case "scroll-bottom":
		evalJS(d.View, "window.scrollTo(0, document.body.scrollHeight)")

	case "open-settings":
		d.openSettings()

	case "zoom-in":
		d.View.SetZoomLevel(d.View.ZoomLevel() * 1.1)
	case "zoom-out":
		d.View.SetZoomLevel(d.View.ZoomLevel() / 1.1)
	case "zoom-reset":
		d.View.SetZoomLevel(1.0)

	// ─── Modes ────────────────────────────────────────────────────────
	case "cmd-mode":
		d.openBar(0, "")
	case "cmd-open":
		d.openBar(0, "")
	case "cmd-open-current":
		d.openBar(0, d.View.URI())
	case "find-start":
		d.openBar('/', "")
	case "find-next":
		for i := 0; i < count; i++ {
			d.Find.Next()
		}
	case "find-prev":
		for i := 0; i < count; i++ {
			d.Find.Prev()
		}
	case "escape":
		d.Find.Cancel()
		d.Bar.Hide()
		evalJS(d.View, `
			(function(){
				if(document.activeElement) document.activeElement.blur();
				var c = document.getElementById('wlbrowser-hint-container');
				if(c && c.parentNode) {
					c.parentNode.removeChild(c);
				}
			})();
		`)
		// mode machine already reset its state when called; nothing else.
	case "unfocus":
		evalJS(d.View, "if(document.activeElement) document.activeElement.blur();")
	case "quit":
		d.Window.Close()
	case "copy-url":
		d.View.Display().Clipboard().SetText(d.View.URI())
	case "hint-links":
		log.Println("wlbrowser: executing hint-links action")
		evalJS(d.View, hintScript)
	case "leave-insert":
		evalJS(d.View, "if(document.activeElement) document.activeElement.blur();")
		d.Bar.Hide()
		// focus tracker will report focusout and flip mode back to Normal.

	case "delete-last-word":
		evalJS(d.View, `
			(function() {
				var el = document.activeElement;
				if (!el || (el.tagName !== 'INPUT' && el.tagName !== 'TEXTAREA')) return;
				var start = el.selectionStart;
				var end = el.selectionEnd;
				if (start !== end) {
					el.setRangeText('', start, end, 'end');
					return;
				}
				var text = el.value;
				var i = start - 1;
				while (i >= 0 && /\s/.test(text[i])) i--;
				while (i >= 0 && !/\s/.test(text[i]) && !/[\.,;:!\?\(\)\[\]\{\}"']/.test(text[i]) && text[i] !== '/') i--;
				el.setRangeText('', i + 1, start, 'end');
			})();
		`)

	// ─── Free-form ────────────────────────────────────────────────────
	case "exec":
		d.runExec(a)
	case "js":
		if a.Script != "" {
			evalJS(d.View, a.Script)
		}

	default:
		log.Printf("wlbrowser: unknown action %q", a.Name)
	}
}

func (d *Dispatcher) openSettings() {
	dialog := gtk.NewDialog()
	dialog.SetTitle("Settings")
	dialog.SetTransientFor(d.Window)
	dialog.SetModal(true)
	dialog.AddButton("_Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("_Save", int(gtk.ResponseAccept))

	content := dialog.ContentArea()
	content.SetMarginTop(12)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)

	grid := gtk.NewGrid()
	grid.SetColumnSpacing(10)
	grid.SetRowSpacing(10)

	lblHome := gtk.NewLabel("Home Page:")
	lblHome.SetHAlign(gtk.AlignStart)
	entryHome := gtk.NewEntry()
	entryHome.SetText(d.Config.Home)
	entryHome.SetHExpand(true)

	grid.Attach(lblHome, 0, 0, 1, 1)
	grid.Attach(entryHome, 1, 0, 1, 1)

	lblHist := gtk.NewLabel("History:")
	lblHist.SetHAlign(gtk.AlignStart)
	lblHist.SetMarginTop(12)
	grid.Attach(lblHist, 0, 1, 2, 1)

	histList := gtk.NewListBox()
	histList.SetSelectionMode(gtk.SelectionNone)
	histList.AddCSSClass("boxed-list")

	refreshHistory := func() {
		// Clear existing
		for child := histList.FirstChild(); child != nil; child = histList.FirstChild() {
			histList.Remove(child)
		}

		if d.Hist == nil {
			histList.Append(gtk.NewLabel("History unavailable"))
			return
		}

		entries, err := d.Hist.List(200) // Fetch more to allow grouping
		if err != nil {
			log.Printf("failed to list history: %v", err)
			return
		}

		// Group by domain
		groups := make(map[string][]history.Entry)
		var domains []string
		for _, e := range entries {
			u, err := url.Parse(e.URL)
			domain := "unknown"
			if err == nil && u.Host != "" {
				domain = u.Host
			}
			if _, ok := groups[domain]; !ok {
				domains = append(domains, domain)
			}
			groups[domain] = append(groups[domain], e)
		}

		for _, domain := range domains {
			domainBox := gtk.NewBox(gtk.OrientationVertical, 0)
			
			header := gtk.NewBox(gtk.OrientationHorizontal, 6)
			header.SetMarginTop(8)
			header.SetMarginBottom(4)
			header.AddCSSClass("domain-header")

			domainLabel := gtk.NewLabel(domain)
			domainLabel.SetHExpand(true)
			domainLabel.SetHAlign(gtk.AlignStart)
			domainLabel.AddCSSClass("domain-title")
			
			delDomainBtn := gtk.NewButtonFromIconName("user-trash-symbolic")
			delDomainBtn.SetTooltipText("Delete all history for " + domain)
			
			header.Append(domainLabel)
			header.Append(delDomainBtn)
			domainBox.Append(header)

			urlsBox := gtk.NewBox(gtk.OrientationVertical, 0)
			urlsBox.SetMarginStart(16) // Indent URLs under domain
			
			for _, e := range groups[domain] {
				row := gtk.NewBox(gtk.OrientationHorizontal, 6)
				row.SetMarginTop(2)
				row.SetMarginBottom(2)
				row.SetMarginStart(8)
				row.SetMarginEnd(8)

				title := e.Title
				if title == "" {
					title = e.URL
				}
				label := gtk.NewLabel(title)
				label.SetHExpand(true)
				label.SetHAlign(gtk.AlignStart)
				label.SetEllipsize(pango.EllipsizeEnd)
				label.SetTooltipText(e.URL)

				delBtn := gtk.NewButtonFromIconName("edit-delete-symbolic")
				delBtn.SetHasFrame(false)
				
				row.Append(label)
				row.Append(delBtn)

				listBoxRow := gtk.NewListBoxRow()
				listBoxRow.SetChild(row)
				urlsBox.Append(listBoxRow)

				// Main list box row for the domain group (pre-declare for closure)
				domainRow := gtk.NewListBoxRow()

				url := e.URL
				delBtn.ConnectClicked(func() {
					if err := d.Hist.Delete(url); err == nil {
						urlsBox.Remove(listBoxRow)
						// If domain box is now empty, we could remove it too
						if urlsBox.FirstChild() == nil {
							histList.Remove(domainRow)
						}
					}
				})
			}

			domainBox.Append(urlsBox)
			
			// Main list box row for the domain group
			domainRow := gtk.NewListBoxRow()
			domainRow.SetChild(domainBox)
			domainRow.SetActivatable(false)
			domainRow.SetSelectable(false)
			histList.Append(domainRow)

			targetDomain := domain
			delDomainBtn.ConnectClicked(func() {
				if err := d.Hist.DeleteDomain(targetDomain); err == nil {
					histList.Remove(domainRow)
				}
			})
		}
	}

	refreshHistory()

	scroll := gtk.NewScrolledWindow()
	scroll.SetChild(histList)
	scroll.SetVExpand(true)
	scroll.SetMinContentHeight(200)
	scroll.SetPropagateNaturalHeight(true)
	grid.Attach(scroll, 0, 2, 2, 1)

	content.Append(grid)

	dialog.ConnectResponse(func(responseID int) {
		if responseID == int(gtk.ResponseAccept) {
			newHome := entryHome.Text()
			if newHome != "" {
				d.Config.Home = newHome
				d.Home = newHome
				if err := d.Config.Save(""); err != nil {
					log.Printf("failed to save settings: %v", err)
				}
			}
		}
		dialog.Destroy()
	})

	dialog.SetDefaultSize(400, -1)
	dialog.Show()
}

func (d *Dispatcher) openBar(prefix rune, initial string) {
	if prefix == 0 {
		d.Bar.SetText(initial)
	} else {
		d.Bar.SetText(string(prefix) + initial)
	}
	d.Bar.GrabFocus()
	d.Machine.SetMode(mode.ModeCommand)
}

func (d *Dispatcher) runExec(a config.Action) {
	if len(a.Command) == 0 {
		return
	}
	args := make([]string, len(a.Command))
	url := d.View.URI()
	for i, s := range a.Command {
		args[i] = strings.ReplaceAll(s, "{url}", url)
	}
	cmd := exec.Command(args[0], args[1:]...)
	if a.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(strings.ReplaceAll(a.Stdin, "{url}", url))
	}
	go func() {
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("wlbrowser: exec %v failed: %v: %s", args, err, out)
		}
	}()
}

const hintScript = `
try {
  (() => {
    if (document.getElementById('wlbrowser-hint-container')) {
      document.getElementById('wlbrowser-hint-container').remove();
      return; // Toggle off
    }

    const elements = document.querySelectorAll('a, button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"], [tabindex]');
    const visible = [];
    for (let el of elements) {
      const rect = el.getBoundingClientRect();
      const style = window.getComputedStyle(el);
      // be very permissive for visibility
      if (rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0') {
        // check if inside viewport
        if (rect.bottom > 0 && rect.top < window.innerHeight && rect.right > 0 && rect.left < window.innerWidth) {
           visible.push(el);
        }
      }
    }

    if (visible.length === 0) return;

    const chars = 'abcdefghijklmnopqrstuvwxyz';
    const depth = Math.ceil(Math.log(visible.length) / Math.log(chars.length)) || 1;
    const getHint = (index) => {
      let hint = '';
      let num = index;
      for (let i = 0; i < depth; i++) {
        hint = chars[num % chars.length] + hint;
        num = Math.floor(num / chars.length);
      }
      return hint;
    };

    const container = document.createElement('div');
    container.id = 'wlbrowser-hint-container';
    container.style.cssText = 'position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 2147483647; pointer-events: none;';
    
    const input = document.createElement('input');
    input.type = 'text';
    input.id = 'wlbrowser-hint-input';
    // Visible but offscreen/transparent so it can receive focus without fail
    input.style.cssText = 'position: absolute; top: 0; left: 0; width: 1px; height: 1px; opacity: 0; border: none; padding: 0; outline: none; pointer-events: auto;';
    container.appendChild(input);

    const hints = [];
    visible.forEach((el, i) => {
      const hint = getHint(i);
      const rect = el.getBoundingClientRect();
      const label = document.createElement('div');
      label.textContent = hint;
      label.style.cssText = 'position: absolute; top: ' + Math.max(0, rect.top) + 'px; left: ' + Math.max(0, rect.left) + 'px; background: #ffeb3b; color: #000; border: 2px solid #000; padding: 2px 4px; font-size: 14px; font-family: monospace; font-weight: bold; border-radius: 3px; box-shadow: 0 2px 5px rgba(0,0,0,0.5); pointer-events: none; text-transform: uppercase;';
      container.appendChild(label);
      hints.push({ el, hint, label });
    });

    document.documentElement.appendChild(container);
    
    // Force focus to enter Insert Mode
    input.focus();
    setTimeout(() => input.focus(), 50);

    const cleanup = () => {
      if (container.parentNode) container.parentNode.removeChild(container);
    };

    input.addEventListener('input', (e) => {
      const currentKeys = input.value.toLowerCase();
      let matched = false;
      let exactMatch = null;
      hints.forEach(h => {
        if (h.hint.startsWith(currentKeys)) {
          h.label.style.display = 'block';
          matched = true;
          if (h.hint === currentKeys) exactMatch = h;
        } else {
          h.label.style.display = 'none';
        }
      });

      if (exactMatch) {
        cleanup();
        exactMatch.el.focus(); // Focus first to emulate tab navigation
        exactMatch.el.click();
        if (exactMatch.el.tagName === 'A' && exactMatch.el.href) {
          // Fallback just in case click() is intercepted and prevents default navigation
          setTimeout(() => { window.location.href = exactMatch.el.href; }, 50);
        }
      } else if (!matched && currentKeys.length > 0) {
        cleanup();
      }
    });

    input.addEventListener('blur', () => {
      // If it blurs because we clicked a link or something, clean up.
      setTimeout(() => {
        if (document.activeElement !== input) cleanup();
      }, 100);
    });
  })();
} catch (e) {
  console.error("wlbrowser hintScript error:", e);
}
`
