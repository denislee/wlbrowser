package keys

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
)

func tok(mods gdk.ModifierType, key uint) Token { return Normalize(mods, key) }

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Sequence
	}{
		{"j", Sequence{tok(0, 'j')}},
		{"gg", Sequence{tok(0, 'g'), tok(0, 'g')}},
		{"G", Sequence{tok(0, 'G')}},
		{"<S-j>", Sequence{tok(0, 'J')}}, // shift+j folds to 'J'
		{"<C-r>", Sequence{tok(gdk.ControlMask, 'r')}},
		{"<C-S-r>", Sequence{tok(gdk.ControlMask, 'R')}},
		{"<Esc>", Sequence{tok(0, gdk.KEY_Escape)}},
		{"<CR>", Sequence{tok(0, gdk.KEY_Return)}},
		{"<Space>m", Sequence{tok(0, gdk.KEY_space), tok(0, 'm')}},
		{"<F2>", Sequence{tok(0, gdk.KEY_F2)}},
		{"5G", Sequence{tok(0, '5'), tok(0, 'G')}},
		{":", Sequence{tok(0, ':')}},
		{"/", Sequence{tok(0, '/')}},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("Parse(%q): len %d != %d (%v vs %v)", c.in, len(got), len(c.want), got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("Parse(%q)[%d] = %+v, want %+v", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "<", "<>", "<C->", "<Foo>", "<C-Foo>"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) expected error", in)
		}
	}
}

func TestNormalizeShiftFolding(t *testing.T) {
	// Live event: user presses Shift+j; GTK gives keyval='J' with ShiftMask.
	// Normalize must yield the same token as parsing "<S-j>" or "J".
	live := Normalize(gdk.ShiftMask, 'J')
	parsed, _ := Parse("<S-j>")
	if live != parsed[0] {
		t.Errorf("live %+v != parsed %+v", live, parsed[0])
	}
	parsedBare, _ := Parse("J")
	if live != parsedBare[0] {
		t.Errorf("live %+v != parsed bare %+v", live, parsedBare[0])
	}
}

func TestTrieMatchExact(t *testing.T) {
	tr := NewTrie[string]()
	seq, _ := Parse("j")
	tr.Add(seq, "scroll-down")

	w := tr.Walk()
	st, v := w.Feed(tok(0, 'j'))
	if st != StatusComplete || v != "scroll-down" {
		t.Errorf("got (%v, %q), want (Complete, scroll-down)", st, v)
	}
	if !w.AtRoot() {
		t.Error("walker not reset after complete match")
	}
}

func TestTrieMatchSequence(t *testing.T) {
	tr := NewTrie[string]()
	seqGG, _ := Parse("gg")
	tr.Add(seqGG, "scroll-top")

	w := tr.Walk()
	st, _ := w.Feed(tok(0, 'g'))
	if st != StatusPartial {
		t.Errorf("after first g: status %v, want Partial", st)
	}
	st, v := w.Feed(tok(0, 'g'))
	if st != StatusComplete || v != "scroll-top" {
		t.Errorf("after gg: (%v, %q), want (Complete, scroll-top)", st, v)
	}
}

func TestTrieAmbiguous(t *testing.T) {
	tr := NewTrie[string]()
	g, _ := Parse("g")
	gg, _ := Parse("gg")
	tr.Add(g, "g-alone")
	tr.Add(gg, "gg-pair")

	w := tr.Walk()
	st, v := w.Feed(tok(0, 'g'))
	if st != StatusAmbiguous || v != "g-alone" {
		t.Errorf("first g: (%v, %q), want (Ambiguous, g-alone)", st, v)
	}
	// Second g resolves to gg-pair.
	st, v = w.Feed(tok(0, 'g'))
	if st != StatusComplete || v != "gg-pair" {
		t.Errorf("second g: (%v, %q), want (Complete, gg-pair)", st, v)
	}
}

func TestTrieNoMatch(t *testing.T) {
	tr := NewTrie[string]()
	gg, _ := Parse("gg")
	tr.Add(gg, "scroll-top")

	w := tr.Walk()
	w.Feed(tok(0, 'g'))           // partial
	st, _ := w.Feed(tok(0, 'x')) // no continuation
	if st != StatusNoMatch {
		t.Errorf("got %v, want NoMatch", st)
	}
	if !w.AtRoot() {
		t.Error("walker should reset on no-match")
	}
}
