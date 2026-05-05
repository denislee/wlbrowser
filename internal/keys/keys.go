// Package keys parses vim-style key strings into normalized token sequences
// and provides a prefix trie for matching against incoming GDK key events.
package keys

import (
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
)

// Token is a single keystroke: a GDK keyval plus normalized modifier bits.
// Shift is folded into the keyval for printable lowercase ASCII letters
// (`<S-j>` and `J` both become `{Mods: 0, Key: 'J'}`).
type Token struct {
	Mods gdk.ModifierType
	Key  uint
}

// Sequence is an ordered list of tokens forming a binding (e.g. `gg`).
type Sequence []Token

var relevantMods = gdk.ControlMask | gdk.AltMask | gdk.SuperMask | gdk.ShiftMask

// Normalize coerces a raw GDK (mods, keyval) into our canonical form so that
// parsed bindings and live key events can compare equal.
func Normalize(mods gdk.ModifierType, key uint) Token {
	mods &= relevantMods
	if mods&gdk.ShiftMask != 0 {
		switch {
		case key >= 'a' && key <= 'z':
			key = key - 'a' + 'A'
			mods &^= gdk.ShiftMask
		case key >= 'A' && key <= 'Z':
			// Live event: GTK already delivered the uppercase keyval.
			mods &^= gdk.ShiftMask
		}
	}
	return Token{Mods: mods, Key: key}
}

// Parse turns a vim-style binding string into a Sequence.
// Examples: `j`, `gg`, `<C-r>`, `<C-S-l>`, `<Space>m`, `5G`.
func Parse(s string) (Sequence, error) {
	if s == "" {
		return nil, fmt.Errorf("empty key string")
	}
	var seq Sequence
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if rs[i] == '<' {
			j := i + 1
			for j < len(rs) && rs[j] != '>' {
				j++
			}
			if j >= len(rs) {
				return nil, fmt.Errorf("unterminated `<` in %q", s)
			}
			tok, err := parseAngle(string(rs[i+1 : j]))
			if err != nil {
				return nil, err
			}
			seq = append(seq, tok)
			i = j + 1
			continue
		}
		seq = append(seq, Normalize(0, uint(rs[i])))
		i++
	}
	return seq, nil
}

func parseAngle(body string) (Token, error) {
	if body == "" {
		return Token{}, fmt.Errorf("empty `<>` token")
	}
	parts := strings.Split(body, "-")
	keyName := parts[len(parts)-1]
	var mods gdk.ModifierType
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "c", "ctrl", "control":
			mods |= gdk.ControlMask
		case "s", "shift":
			mods |= gdk.ShiftMask
		case "a", "alt":
			mods |= gdk.AltMask
		case "m", "meta", "super", "win":
			mods |= gdk.SuperMask
		default:
			return Token{}, fmt.Errorf("unknown modifier %q in <%s>", m, body)
		}
	}
	key, ok := lookupKey(keyName)
	if !ok {
		return Token{}, fmt.Errorf("unknown key %q in <%s>", keyName, body)
	}
	return Normalize(mods, key), nil
}

// lookupKey resolves a key name (the part inside `<...>` after modifiers) to
// a GDK keyval. Single-rune names map to their codepoint.
func lookupKey(name string) (uint, bool) {
	if k, ok := namedKeys[name]; ok {
		return k, true
	}
	if k, ok := namedKeys[strings.ToLower(name)]; ok {
		return k, true
	}
	rs := []rune(name)
	if len(rs) == 1 {
		return uint(rs[0]), true
	}
	return 0, false
}

var namedKeys = map[string]uint{
	"esc":       gdk.KEY_Escape,
	"escape":    gdk.KEY_Escape,
	"cr":        gdk.KEY_Return,
	"return":    gdk.KEY_Return,
	"enter":     gdk.KEY_Return,
	"tab":       gdk.KEY_Tab,
	"bs":        gdk.KEY_BackSpace,
	"backspace": gdk.KEY_BackSpace,
	"space":     gdk.KEY_space,
	"up":        gdk.KEY_Up,
	"down":      gdk.KEY_Down,
	"left":      gdk.KEY_Left,
	"right":     gdk.KEY_Right,
	"home":      gdk.KEY_Home,
	"end":       gdk.KEY_End,
	"pageup":    gdk.KEY_Page_Up,
	"pagedown":  gdk.KEY_Page_Down,
	"del":       gdk.KEY_Delete,
	"delete":    gdk.KEY_Delete,
	"insert":    gdk.KEY_Insert,
	"f1":        gdk.KEY_F1,
	"f2":        gdk.KEY_F2,
	"f3":        gdk.KEY_F3,
	"f4":        gdk.KEY_F4,
	"f5":        gdk.KEY_F5,
	"f6":        gdk.KEY_F6,
	"f7":        gdk.KEY_F7,
	"f8":        gdk.KEY_F8,
	"f9":        gdk.KEY_F9,
	"f10":       gdk.KEY_F10,
	"f11":       gdk.KEY_F11,
	"f12":       gdk.KEY_F12,
	"plus":      gdk.KEY_plus,
	"minus":     gdk.KEY_minus,
	"equal":     gdk.KEY_equal,
	"slash":     gdk.KEY_slash,
	"colon":     gdk.KEY_colon,
	"semicolon": gdk.KEY_semicolon,
}
