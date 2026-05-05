package keys

// Trie is a generic prefix tree over Sequence -> T. It powers two things:
//   - resolving a partially typed sequence to its bound action (mode.go)
//   - detecting that a longer extension still exists, which is what tells the
//     mode machine to wait for another keystroke before firing a shorter match.
type Trie[T any] struct {
	root *node[T]
}

type node[T any] struct {
	children map[Token]*node[T]
	value    T
	hasValue bool
}

func NewTrie[T any]() *Trie[T] {
	return &Trie[T]{root: newNode[T]()}
}

func newNode[T any]() *node[T] {
	return &node[T]{children: make(map[Token]*node[T])}
}

// Add inserts seq -> val. Returns false if a value was already bound at this
// exact sequence (caller decides whether to overwrite or error out).
func (t *Trie[T]) Add(seq Sequence, val T) bool {
	n := t.root
	for _, tok := range seq {
		c, ok := n.children[tok]
		if !ok {
			c = newNode[T]()
			n.children[tok] = c
		}
		n = c
	}
	first := !n.hasValue
	n.value = val
	n.hasValue = true
	return first
}

// Walker tracks an in-progress match across successive key events.
type Walker[T any] struct {
	tree *Trie[T]
	cur  *node[T]
}

func (t *Trie[T]) Walk() *Walker[T] {
	return &Walker[T]{tree: t, cur: t.root}
}

// Reset puts the walker back at the root.
func (w *Walker[T]) Reset() { w.cur = w.tree.root }

// Status describes what happened after Feed.
type Status int

const (
	// StatusNoMatch: this token has no continuation from the current state;
	// the walker is left at the root so the caller can retry the same token
	// as a fresh start.
	StatusNoMatch Status = iota
	// StatusPartial: prefix matches but no value at this node yet, or there's
	// a value but extensions also exist (caller should wait for timeout).
	StatusPartial
	// StatusComplete: exact match at a terminal node (no extensions). Fire now.
	StatusComplete
	// StatusAmbiguous: exact match exists at this node, but extensions are
	// also possible. Caller should wait for the next key or for the sequence
	// timeout, then fire the value.
	StatusAmbiguous
)

// Feed advances the walker by one token. The returned value is meaningful
// only for StatusComplete and StatusAmbiguous.
func (w *Walker[T]) Feed(tok Token) (Status, T) {
	var zero T
	c, ok := w.cur.children[tok]
	if !ok {
		w.cur = w.tree.root
		return StatusNoMatch, zero
	}
	w.cur = c
	hasKids := len(c.children) > 0
	switch {
	case c.hasValue && !hasKids:
		w.cur = w.tree.root
		return StatusComplete, c.value
	case c.hasValue && hasKids:
		return StatusAmbiguous, c.value
	default:
		return StatusPartial, zero
	}
}

// PendingValue returns the value at the walker's current node, if any. Used
// when the sequence timeout fires on an ambiguous state.
func (w *Walker[T]) PendingValue() (T, bool) {
	var zero T
	if w.cur.hasValue {
		v := w.cur.value
		w.cur = w.tree.root
		return v, true
	}
	w.cur = w.tree.root
	return zero, false
}

// AtRoot reports whether the walker is fresh (no partial sequence in flight).
func (w *Walker[T]) AtRoot() bool { return w.cur == w.tree.root }
