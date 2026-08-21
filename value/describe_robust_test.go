package value_test

// Describe is called to build an error message out of whatever a resolver
// returned, so it has to cope with any Go value at all — including the shapes
// that would send a naive renderer round for ever.

import (
	"testing"

	"github.com/ikawaha/graphql/value"
)

func expectNoPanic(t *testing.T, name string, f func()) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC: %v", r)
			}
		}()
		f()
	})
}

type ring struct {
	Name string
	Next *ring
}

type holder struct {
	Items  []*holder
	Map    map[string]*holder
	hidden int
}

func TestDescribe_SurvivesAnyValue(t *testing.T) {
	// Cycles of every shape a Go value can make.
	self := &ring{Name: "a"}
	self.Next = self
	expectNoPanic(t, "a struct pointing at itself", func() { value.Describe(self) })

	a, b := &ring{Name: "a"}, &ring{Name: "b"}
	a.Next, b.Next = b, a
	expectNoPanic(t, "two structs pointing at each other", func() { value.Describe(a) })

	h := &holder{hidden: 1}
	h.Items = []*holder{h}
	expectNoPanic(t, "a struct holding a list of itself", func() { value.Describe(h) })

	m := &holder{Map: map[string]*holder{}}
	m.Map["self"] = m
	expectNoPanic(t, "a struct holding a map of itself", func() { value.Describe(m) })

	slice := make([]any, 1)
	slice[0] = slice
	expectNoPanic(t, "a slice holding itself", func() { value.Describe(slice) })

	deep := any(nil)
	for range 200 {
		deep = []any{deep}
	}
	expectNoPanic(t, "a very deep value", func() { value.Describe(deep) })

	expectNoPanic(t, "a struct with only unexported fields", func() {
		value.Describe(struct{ hidden int }{1})
	})
	expectNoPanic(t, "a pointer to a pointer", func() {
		p := &self
		value.Describe(&p)
	})
	expectNoPanic(t, "an interface holding a typed nil", func() {
		var nilRing *ring
		value.Describe(any(nilRing))
	})
	expectNoPanic(t, "a map with a struct key", func() {
		value.Describe(map[ring]int{{Name: "k"}: 1})
	})
	expectNoPanic(t, "a channel and a function", func() {
		value.Describe([]any{make(chan int), func() {}})
	})
}
