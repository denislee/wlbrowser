package actions

// #cgo pkg-config: webkitgtk-6.0
// #include <webkit/webkit.h>
import "C"

import (
	"unsafe"

	webkit "github.com/diamondburned/gotk4-webkitgtk/pkg/webkit/v6"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// evalJS fires `script` into the page and forgets — no result, no error path.
// The Go binding skips webkit_web_view_evaluate_javascript (it generates only
// the _finish counterpart), so we drop to C directly.
func evalJS(view *webkit.WebView, script string) {
	cs := C.CString(script)
	defer C.free(unsafe.Pointer(cs))
	// Native() returns a uintptr to the C GObject; gotk4's own generated
	// bindings use the same uintptr -> unsafe.Pointer cast — `go vet` flags
	// it but the object is not Go-allocated so this is safe.
	cView := (*C.WebKitWebView)(unsafe.Pointer(coreglib.BaseObject(view).Native())) //nolint:govet
	C.webkit_web_view_evaluate_javascript(cView, cs, -1, nil, nil, nil, nil, nil)
}
