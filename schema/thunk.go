package schema

import "sync"

// lazy defers building a value until it is first needed.
//
// Types refer to one another, and often to themselves, so the fields of an
// object cannot always be built at the moment the object is created: the type
// a field points at may not exist yet. A thunk closes that loop by deferring
// the work until every type has been created.
//
// The reference implementation resolves such a thunk in place, replacing the
// function with its result the first time it is asked. That is safe in a
// single-threaded runtime but not here, where one schema serves many requests
// at once, so resolution goes through sync.Once. Building a schema resolves
// every thunk up front, which leaves nothing but an already-completed Once on
// the path a request takes.
//
// A lazy must not be copied once it has been used.
type lazy[T any] struct {
	once  sync.Once
	build func() T
	value T
}

// newLazy returns a lazy that calls build the first time it is read.
func newLazy[T any](build func() T) *lazy[T] {
	return &lazy[T]{build: build}
}

// get returns the value, building it on the first call.
func (l *lazy[T]) get() T {
	l.once.Do(func() {
		if l.build != nil {
			l.value = l.build()
			// Release the closure so that whatever it captured can be
			// collected once the value exists.
			l.build = nil
		}
	})
	return l.value
}

// resolve builds the value if it has not been built, and reports nothing. It
// is what schema construction calls to get every thunk out of the way before
// any request is served.
func (l *lazy[T]) resolve() { l.get() }
