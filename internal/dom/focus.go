// Package dom holds JS shims injected into every page WebKit loads. The
// focus tracker tells Go-side code when an editable element gains/loses focus
// so the mode machine can switch between Normal and Insert without the user
// having to think about it.
package dom

import (
	"fmt"

	javascriptcore "github.com/diamondburned/gotk4-webkitgtk/pkg/javascriptcore/v6"
	webkit "github.com/diamondburned/gotk4-webkitgtk/pkg/webkit/v6"
)

const focusScript = `
(() => {
  const post = (v) => {
    try { window.webkit.messageHandlers.wlbFocus.postMessage(v); } catch (_) {}
  };
  const isEditable = (el) => {
    if (!el || !el.tagName) return false;
    if (el.isContentEditable) return true;
    const tag = el.tagName;
    if (tag === 'TEXTAREA') return true;
    if (tag === 'INPUT') {
      const t = (el.type || 'text').toLowerCase();
      return !['button','checkbox','radio','submit','reset','image','file','hidden'].includes(t);
    }
    const role = el.getAttribute && el.getAttribute('role');
    return role === 'textbox' || role === 'searchbox' || role === 'combobox';
  };
  document.addEventListener('focusin',  (e) => { if (isEditable(e.target)) post('1'); }, true);
  document.addEventListener('focusout', (e) => { if (isEditable(e.target)) post('0'); }, true);
})();
`

// RegisterFocusTracker installs the focus-tracking UserScript on the given
// content manager and routes script messages from `wlbFocus` to onChange.
func RegisterFocusTracker(manager *webkit.UserContentManager, onChange func(insert bool)) error {
	if !manager.RegisterScriptMessageHandler("wlbFocus", "") {
		return fmt.Errorf("failed to register wlbFocus message handler")
	}
	script := webkit.NewUserScript(
		focusScript,
		webkit.UserContentInjectAllFrames,
		webkit.UserScriptInjectAtDocumentStart,
		nil, nil,
	)
	manager.AddScript(script)
	manager.ConnectScriptMessageReceived(func(value *javascriptcore.Value) {
		// Only one handler is registered, so every message is wlbFocus.
		onChange(value.String() == "1")
	})
	return nil
}
