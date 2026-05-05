package cmdbar

import (
	"unicode"

	gdk "github.com/diamondburned/gotk4/pkg/gdk/v4"
	gtk "github.com/diamondburned/gotk4/pkg/gtk/v4"
	pango "github.com/diamondburned/gotk4/pkg/pango"
)

// Suggestion represents a single item in the autocomplete list.
type Suggestion struct {
	Text string
	Desc string
}

// Bar is a single-line entry that acts as an address bar.
type Bar struct {
	Entry    *gtk.Entry
	List     *gtk.ListBox
	Scrolled *gtk.ScrolledWindow

	OnSubmit func(text string)
	OnCancel func()
	OnChange func(text string)
}

func New() *Bar {
	e := gtk.NewEntry()
	e.SetVisible(false)
	e.SetPlaceholderText("Address or Search...")

	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionSingle)
	list.AddCSSClass("rich-list")

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetChild(list)
	scrolled.SetVisible(false)
	scrolled.SetPropagateNaturalHeight(true)
	scrolled.SetMaxContentHeight(400)
	scrolled.SetHasFrame(true)

	b := &Bar{Entry: e, List: list, Scrolled: scrolled}

	list.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		text := row.ObjectProperty("name").(string)
		b.Hide()
		if b.OnSubmit != nil {
			b.OnSubmit(text)
		}
	})

	e.ConnectActivate(func() {
		// if something is selected in the list, use it
		row := list.SelectedRow()
		var text string
		if row != nil {
			// Find the text in the row
			// we'll store the URL as the widget name or an object data
			text = row.ObjectProperty("name").(string)
		} else {
			text = e.Text()
		}

		b.Hide()
		if b.OnSubmit != nil {
			b.OnSubmit(text)
		}
	})

	e.ConnectChanged(func() {
		if b.OnChange != nil {
			b.OnChange(e.Text())
		}
	})

	focusCtrl := gtk.NewEventControllerFocus()
	focusCtrl.ConnectLeave(func() {
		// Only hide if focus is not moving to the list
		if !list.HasFocus() && !scrolled.HasFocus() {
			b.Cancel()
		}
	})
	e.AddController(focusCtrl)

	// Add key controller to Entry to handle up/down arrows for the ListBox
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		if keyval == gdk.KEY_Down || ((keyval == gdk.KEY_n || keyval == gdk.KEY_j) && (state&gdk.ControlMask) != 0) {
			// Select next
			row := list.SelectedRow()
			var idx int
			if row == nil {
				idx = 0
			} else {
				idx = row.Index() + 1
			}
			next := list.RowAtIndex(idx)
			if next != nil {
				list.SelectRow(next)
			}
			return true
		}
		if keyval == gdk.KEY_Up || (keyval == gdk.KEY_p && (state&gdk.ControlMask) != 0) {
			// Select prev
			row := list.SelectedRow()
			if row != nil {
				idx := row.Index() - 1
				if idx >= 0 {
					list.SelectRow(list.RowAtIndex(idx))
				} else {
					list.UnselectAll()
				}
			}
			return true
		}
		return false
	})
	e.AddController(keyCtrl)

	return b
}

func (b *Bar) UpdateSuggestions(suggestions []Suggestion) {
	// Remove old items
	for listChild := b.List.FirstChild(); listChild != nil; listChild = b.List.FirstChild() {
		b.List.Remove(listChild)
	}

	if len(suggestions) == 0 || !b.Entry.Visible() {
		b.Scrolled.SetVisible(false)
		return
	}

	for _, s := range suggestions {
		box := gtk.NewBox(gtk.OrientationVertical, 2)
		box.SetMarginTop(4)
		box.SetMarginBottom(4)
		box.SetMarginStart(8)
		box.SetMarginEnd(8)

		lblText := gtk.NewLabel(s.Text)
		lblText.SetHAlign(gtk.AlignStart)
		lblText.SetEllipsize(pango.EllipsizeEnd)
		
		box.Append(lblText)

		if s.Desc != "" {
			lblDesc := gtk.NewLabel(s.Desc)
			lblDesc.SetHAlign(gtk.AlignStart)
			lblDesc.SetEllipsize(pango.EllipsizeEnd)
			lblDesc.AddCSSClass("dim-label")
			box.Append(lblDesc)
		}

		row := gtk.NewListBoxRow()
		row.SetChild(box)
		row.SetObjectProperty("name", s.Text) // Store the URL here
		b.List.Append(row)
	}

	b.Scrolled.SetVisible(true)
}

func (b *Bar) SetText(text string) {
	b.Entry.SetText(text)
}

func (b *Bar) GrabFocus() {
	b.Entry.SetVisible(true)
	b.Entry.GrabFocus()
	b.Entry.SetPosition(-1)
}

func (b *Bar) Hide() {
	b.Entry.SetVisible(false)
	b.Scrolled.SetVisible(false)
	b.List.UnselectAll()
}

// Cancel hides the entry without firing OnSubmit; OnCancel is called instead.
func (b *Bar) Cancel() {
	b.Hide()
	if b.OnCancel != nil {
		b.OnCancel()
	}
}

func deleteLastWordBeforeCursor(text string, pos int) (string, int) {
	r := []rune(text)
	if pos < 0 || pos > len(r) {
		pos = len(r)
	}

	end := pos - 1
	for end >= 0 && unicode.IsSpace(r[end]) {
		end--
	}

	start := end
	for start >= 0 && !unicode.IsSpace(r[start]) && !unicode.IsPunct(r[start]) && r[start] != '/' {
		start--
	}

	if start == end && start >= 0 {
		start-- // always delete at least one char if we hit punctuation
	}

	res := append(r[:start+1], r[pos:]...)
	return string(res), start + 1
}

// AttachEscape wires Escape and Ctrl+K on the Entry to Cancel().
func (b *Bar) AttachEscape() {
	ctrl := gtk.NewEventControllerKey()
	ctrl.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		if keyval == gdk.KEY_Escape || (keyval == gdk.KEY_k && (state&gdk.ControlMask) != 0) {
			b.Cancel()
			return true
		}
		if keyval == gdk.KEY_w && (state&gdk.ControlMask) != 0 {
			text := b.Entry.Text()
			pos := b.Entry.Position()
			newText, newPos := deleteLastWordBeforeCursor(text, pos)
			b.Entry.SetText(newText)
			b.Entry.SetPosition(newPos)
			return true
		}
		return false
	})
	b.Entry.AddController(ctrl)
}
