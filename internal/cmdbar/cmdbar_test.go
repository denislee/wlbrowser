package cmdbar

import "testing"

func TestDeleteLastWordBeforeCursor(t *testing.T) {
	tests := []struct {
		text    string
		pos     int
		want    string
		wantPos int
	}{
		{"hello world", 11, "hello ", 6},
		{"hello world", 5, " world", 0},
		{"hello world", 6, "world", 0},
		{"hello  world", 7, "world", 0},
		{"abc/def", 7, "abc/", 4},
		{"abc.def", 7, "abc.", 4},
		{"word", 4, "", 0},
		{"  word", 6, "  ", 2},
		{"word  ", 6, "", 0},
		{"a b c", 5, "a b ", 4},
		{"", 0, "", 0},
		{"test", 0, "test", 0},
	}

	for _, tt := range tests {
		got, gotPos := deleteLastWordBeforeCursor(tt.text, tt.pos)
		if got != tt.want || gotPos != tt.wantPos {
			t.Errorf("deleteLastWordBeforeCursor(%q, %d) = (%q, %d), want (%q, %d)",
				tt.text, tt.pos, got, gotPos, tt.want, tt.wantPos)
		}
	}
}
