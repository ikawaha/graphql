package value

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// How much of a value a message shows. Past these it would say more about the
// value than about what is wrong with it.
const (
	describeMaxItems = 10
	describeMaxDepth = 2
)

// Describe renders a value the way an error message wants to show it: a string
// in quotes, a list in brackets, an object in braces.
//
// It is graphql-js's inspect, and the messages it appears in are worded around
// what it produces. Two things it cannot do the same way: a Go map has no
// order, so its keys are sorted, and a Go value carries no constructor name,
// so anything too deep to show is plainly "[Object]".
func Describe(v any) string {
	return renderValue(v, nil)
}

// renderValue renders one value. seen holds the values enclosing this one,
// which is both how depth is counted and how a value containing itself is
// noticed.
func renderValue(v any, seen []uintptr) string {
	if v == nil {
		return "null"
	}
	switch typed := v.(type) {
	case string:
		return strconv.Quote(typed)
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	case *OrderedMap:
		if typed == nil {
			return "null"
		}
		return renderOrdered(typed, seen)
	}

	rv := reflect.ValueOf(v)
	// Nothing is a clearer answer than an empty one for a value that is not
	// there: a nil map is not an object without keys, and JavaScript, which
	// has one way of saying it, says null.
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		if rv.IsNil() {
			return "null"
		}
	}
	// A function is named rather than shown, as graphql-js names one. Asked
	// before whether the value writes itself as JSON, because a function is
	// not an object and graphql-js only asks an object.
	if rv.Kind() == reflect.Func {
		return renderFunc(rv)
	}
	if written, ok := renderMarshalled(v, rv, seen); ok {
		return written
	}
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		// Following a pointer is where a Go value can lead back to itself: a
		// struct holding one of its own kind is the usual way, and the struct
		// has no address of its own to remember it by. This is the same
		// bookkeeping a map or a slice gets, one step earlier.
		inner, circular := enter(rv, seen)
		if circular {
			return "[Circular]"
		}
		return renderValue(rv.Elem().Interface(), inner)
	case reflect.Slice, reflect.Array:
		return renderList(rv, seen)
	case reflect.Chan, reflect.UnsafePointer:
		// The address these hold would differ from one run to the next, and
		// an error message that changed run to run would be worse than one
		// that only says what kind of thing it found.
		return "[" + rv.Kind().String() + "]"
	case reflect.Map:
		return renderMap(rv, seen)
	case reflect.Struct:
		return renderStruct(rv, seen)
	case reflect.Float32, reflect.Float64:
		return renderFloat(rv.Float())
	default:
		return fmt.Sprintf("%v", v)
	}
}

// renderFloat writes a number as JavaScript does: without a trailing ".0" on a
// whole one, and with the names JavaScript gives the two ends and the
// not-a-number.
func renderFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// renderFunc names a function, as graphql-js names one rather than showing
// what it holds. Go's name for a function declared inside another is the
// enclosing name followed by func1, func2 and so on, where JavaScript has no
// name to give and writes plain "[function]".
func renderFunc(rv reflect.Value) string {
	name := ""
	if fn := runtime.FuncForPC(rv.Pointer()); fn != nil {
		name = fn.Name()
		if slash := strings.LastIndex(name, "/"); slash >= 0 {
			name = name[slash+1:]
		}
		if dot := strings.Index(name, "."); dot >= 0 {
			name = name[dot+1:]
		}
	}
	if name == "" {
		return "[function]"
	}
	return "[function " + name + "]"
}

// renderMarshalled asks a value that knows how to write itself as JSON to do
// so, which is what graphql-js does with a value carrying a toJSON: what the
// value says about itself is what an error message shows. A value that says
// it is a string is shown as that string rather than as a quoted one, again
// following graphql-js.
//
// It answers false when the value has nothing to say or fails to say it, so
// that the value is shown the ordinary way instead.
func renderMarshalled(v any, rv reflect.Value, seen []uintptr) (string, bool) {
	writer, ok := v.(json.Marshaler)
	if !ok {
		return "", false
	}
	raw, err := writer.MarshalJSON()
	if err != nil {
		return "", false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return "", false
	}
	inner, circular := enter(rv, seen)
	if circular {
		return "[Circular]", true
	}
	if written, isString := decoded.(string); isString {
		return written, true
	}
	return renderValue(decoded, inner), true
}

// enter records that a value is being descended into. It answers with the
// enclosing values including this one, unless this one is already among them.
//
// Identity is the address the value holds, which is what a map, a slice and a
// pointer each have and what makes two of them the same value.
func enter(rv reflect.Value, seen []uintptr) (inner []uintptr, circular bool) {
	var id uintptr
	switch rv.Kind() {
	case reflect.Map, reflect.Slice, reflect.Pointer:
		id = rv.Pointer()
	}
	if id != 0 {
		for _, outer := range seen {
			if outer == id {
				return nil, true
			}
		}
	}
	return append(append([]uintptr(nil), seen...), id), false
}

func renderList(rv reflect.Value, seen []uintptr) string {
	if rv.Len() == 0 {
		return "[]"
	}
	inner, circular := enter(rv, seen)
	switch {
	case circular:
		return "[Circular]"
	case len(inner) > describeMaxDepth:
		return "[Array]"
	}

	shown := min(rv.Len(), describeMaxItems)
	items := make([]string, 0, shown+1)
	for i := range shown {
		items = append(items, renderValue(rv.Index(i).Interface(), inner))
	}
	switch remaining := rv.Len() - shown; {
	case remaining == 1:
		items = append(items, "... 1 more item")
	case remaining > 1:
		items = append(items, fmt.Sprintf("... %d more items", remaining))
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func renderMap(rv reflect.Value, seen []uintptr) string {
	if rv.Len() == 0 {
		return "{}"
	}
	inner, circular := enter(rv, seen)
	switch {
	case circular:
		return "[Circular]"
	case len(inner) > describeMaxDepth:
		return "[Object]"
	}

	keys := rv.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
	})
	entries := make([]string, 0, len(keys))
	for _, k := range keys {
		entries = append(entries,
			fmt.Sprint(k.Interface())+": "+renderValue(rv.MapIndex(k).Interface(), inner))
	}
	return "{ " + strings.Join(entries, ", ") + " }"
}

// renderStruct renders a struct the way an object is rendered, since a struct
// is what a Go server usually has where JavaScript would have an object. Only
// the exported fields are shown: the rest are not part of what the value is
// to anyone outside.
func renderStruct(rv reflect.Value, seen []uintptr) string {
	fields := rv.Type()
	if fields.NumField() == 0 {
		return "{}"
	}
	// A struct stands in for an object, so it is held to the same depth: a
	// value nested past it is named rather than spelled out. Without this a
	// chain of structs — a linked list, a tree of nodes — would be rendered
	// whole, and what is being built here is one line of an error message.
	inner, circular := enter(rv, seen)
	switch {
	case circular:
		return "[Circular]"
	case len(inner) > describeMaxDepth:
		return "[Object]"
	}

	entries := make([]string, 0, fields.NumField())
	for i := range fields.NumField() {
		if declared := fields.Field(i); declared.IsExported() {
			entries = append(entries, declared.Name+": "+renderValue(rv.Field(i).Interface(), inner))
		}
	}
	if len(entries) == 0 {
		return "{}"
	}
	return "{ " + strings.Join(entries, ", ") + " }"
}

// renderOrdered renders a response object, which unlike a map knows the order
// its keys were written in.
func renderOrdered(m *OrderedMap, seen []uintptr) string {
	if m.Len() == 0 {
		return "{}"
	}
	inner, circular := enter(reflect.ValueOf(m), seen)
	switch {
	case circular:
		return "[Circular]"
	case len(inner) > describeMaxDepth:
		return "[Object]"
	}
	entries := make([]string, 0, m.Len())
	for _, k := range m.Keys() {
		held, _ := m.Get(k)
		entries = append(entries, k+": "+renderValue(held, inner))
	}
	return "{ " + strings.Join(entries, ", ") + " }"
}
