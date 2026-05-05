package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const desktopFileTemplate = `[Desktop Entry]
Version=1.0
Type=Application
Name=wlbrowser
Comment=Wayland-friendly browser
Exec=%s %%u
Icon=web-browser
Terminal=false
MimeType=text/html;text/xml;application/xhtml+xml;x-scheme-handler/http;x-scheme-handler/https;
Categories=Network;WebBrowser;
`

// EnsureDefaultBrowser ensures that wlbrowser is registered as a desktop
// application and set as the default web browser.
func EnsureDefaultBrowser() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	appsDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return fmt.Errorf("failed to create applications directory: %w", err)
	}

	desktopPath := filepath.Join(appsDir, "wlbrowser.desktop")
	desktopContent := fmt.Sprintf(desktopFileTemplate, exe)

	if err := os.WriteFile(desktopPath, []byte(desktopContent), 0644); err != nil {
		return fmt.Errorf("failed to write desktop file: %w", err)
	}

	// Use xdg-settings to set as default browser
	cmd := exec.Command("xdg-settings", "set", "default-web-browser", "wlbrowser.desktop")
	if err := cmd.Run(); err != nil {
		// xdg-settings might not be available or might fail, try xdg-mime as fallback
		mimeTypes := []string{"text/html", "x-scheme-handler/http", "x-scheme-handler/https", "application/xhtml+xml"}
		for _, mt := range mimeTypes {
			exec.Command("xdg-mime", "default", "wlbrowser.desktop", mt).Run()
		}
	}

	return nil
}
