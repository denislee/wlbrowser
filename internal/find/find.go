// Package find wraps WebKit's FindController so the rest of the app can
// drive in-page search without naming WebKit constants.
package find

import (
	webkit "github.com/diamondburned/gotk4-webkitgtk/pkg/webkit/v6"
)

const defaultOpts = uint32(webkit.FindOptionsCaseInsensitive | webkit.FindOptionsWrapAround)
const maxMatches = 1000

// Controller is a thin facade over webkit.FindController.
type Controller struct {
	fc *webkit.FindController
}

func New(view *webkit.WebView) *Controller {
	return &Controller{fc: view.FindController()}
}

// Start (re)starts a search for text. Pass an empty string to clear.
func (c *Controller) Start(text string) {
	if text == "" {
		c.fc.SearchFinish()
		return
	}
	c.fc.Search(text, defaultOpts, maxMatches)
}

func (c *Controller) Next()   { c.fc.SearchNext() }
func (c *Controller) Prev()   { c.fc.SearchPrevious() }
func (c *Controller) Cancel() { c.fc.SearchFinish() }
