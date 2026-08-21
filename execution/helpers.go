package execution

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// quote wraps a name for a message.
func quote(s string) string { return strconv.Quote(s) }

// nameOf reads a name, coping with there being none.
func nameOf(n *language.Name) string {
	if n == nil {
		return ""
	}
	return n.Value
}

// formatValuePath renders a path inside a supplied value, for a complaint
// about part of one.
func formatValuePath(path []any) string {
	var b strings.Builder
	for _, step := range path {
		switch key := step.(type) {
		case string:
			b.WriteString("." + key)
		case int:
			b.WriteString("[" + strconv.Itoa(key) + "]")
		}
	}
	return b.String()
}

// isNothing reports whether a resolver returned nothing at all.
//
// A resolver that returns a typed nil pointer means null just as much as one
// that returns an untyped nil, but the two are not the same value once inside
// an interface, and comparing to nil only catches the second.
func isNothing(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}

// declaredTypeName reads the type name a value states for itself.
//
// A value may say what it is in two ways: by answering GraphQLTypeName, or by
// carrying a __typename key, which is what a server whose values are maps
// decoded from somewhere else naturally has. graphql-js reads the second, and
// a schema written as SDL has no other way to say which type a value is.
func declaredTypeName(v any) string {
	if v == nil {
		return ""
	}
	if named, says := v.(interface{ GraphQLTypeName() string }); says {
		return named.GraphQLTypeName()
	}
	if held, says := v.(*value.OrderedMap); says && held != nil {
		if name, isName := held.Get("__typename"); isName {
			if text, isText := name.(string); isText {
				return text
			}
		}
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return ""
	}
	held := rv.MapIndex(reflect.ValueOf("__typename").Convert(rv.Type().Key()))
	if !held.IsValid() {
		return ""
	}
	name, isName := held.Interface().(string)
	if !isName {
		return ""
	}
	return name
}

// goTypeName reads the name of a value's Go type, with pointers followed.
//
// It is the last resort for deciding which object type a value is: a Go type
// named after the GraphQL one is the common case, and asking costs nothing
// when nothing else has said.
func goTypeName(v any) string {
	if v == nil {
		return ""
	}
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

// PanicError is what a resolver that panicked produces.
//
// A panic in a resolver is a fault in code the server author wrote, and it
// would otherwise take down every request the process is serving rather than
// the one field it belongs to. It is caught and reported as that field
// failing, with the stack kept so the cause can still be found.
type PanicError struct {
	// Value is what was passed to panic.
	Value any
	// Stack is where it happened.
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic in resolver: %v", e.Value)
}

// Unwrap exposes the panic value when it was itself an error, so that
// errors.Is and errors.As reach through.
func (e *PanicError) Unwrap() error {
	err, isError := e.Value.(error)
	if !isError {
		return nil
	}
	return err
}

// captureStack records where a panic happened.
func captureStack() []byte {
	buf := make([]byte, 8192)
	return buf[:runtime.Stack(buf, false)]
}
