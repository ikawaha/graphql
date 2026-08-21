package execution

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// DefaultResolver produces a field's value from the source it was reached
// through, for a field the schema gave no resolver of its own.
//
// graphql-js reads a property of the source object, which JavaScript makes
// trivial. Go has no such thing, so what stands in for it is spelt out:
//
//   - a map keyed by string is read by the field's name;
//   - a [value.OrderedMap] likewise;
//   - a struct is read by a field tagged `graphql:"name"`, or failing that a
//     field whose name matches the GraphQL one ignoring case;
//   - a method of the matching name is called, with the context and the
//     field's arguments if it takes them, and may return an error alongside
//     its value.
//
// A pointer is followed, and a nil one resolves to null rather than failing:
// an absent object has absent fields. A field the source does not have
// resolves to null too, which is what a missing property gives in JavaScript.
func DefaultResolver(ctx context.Context, source any, args schema.Arguments, info *schema.ResolveInfo) (any, error) {
	if source == nil {
		return nil, nil
	}
	name := info.FieldName

	switch typed := source.(type) {
	case map[string]any:
		return typed[name], nil
	case *value.OrderedMap:
		v, _ := typed.Get(name)
		return v, nil
	}

	rv := reflect.ValueOf(source)
	// A method may be declared on the pointer type, so it is looked for before
	// the pointer is followed.
	if result, found, err := callMethod(ctx, rv, name, args); found {
		return result, err
	}
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
		if result, found, err := callMethod(ctx, rv, name, args); found {
			return result, err
		}
	}

	if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
		entry := rv.MapIndex(reflect.ValueOf(name).Convert(rv.Type().Key()))
		if !entry.IsValid() {
			return nil, nil
		}
		return entry.Interface(), nil
	}
	if rv.Kind() != reflect.Struct {
		return nil, nil
	}
	if index, found := fieldIndexOf(rv.Type(), name); found {
		return rv.FieldByIndex(index).Interface(), nil
	}
	return nil, nil
}

// callMethod calls a method standing for a field, if the value has one.
//
// The method may take a context and may return an error, so the four
// combinations are all accepted; anything else is not treated as a resolver.
func callMethod(
	ctx context.Context,
	rv reflect.Value,
	name string,
	args schema.Arguments,
) (any, bool, error) {
	if !rv.IsValid() {
		return nil, false, nil
	}
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil, false, nil
	}
	method := rv.MethodByName(exported(name))
	if !method.IsValid() {
		return nil, false, nil
	}
	t := method.Type()

	// A method may ask for the context, the field's arguments, or both, in
	// that order. Anything else is not a stand-in for a field.
	var in []reflect.Value
	switch {
	case t.NumIn() == 0:
	case t.NumIn() == 1 && t.In(0) == contextType:
		in = []reflect.Value{reflect.ValueOf(ctx)}
	case t.NumIn() == 1 && t.In(0) == argumentsType:
		in = []reflect.Value{reflect.ValueOf(args)}
	case t.NumIn() == 2 && t.In(0) == contextType && t.In(1) == argumentsType:
		in = []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(args)}
	default:
		return nil, false, nil
	}

	switch {
	case t.NumOut() == 1:
	case t.NumOut() == 2 && t.Out(1) == errorType:
	default:
		return nil, false, nil
	}

	out := method.Call(in)
	if len(out) == 2 && !out[1].IsNil() {
		// The second result was checked to be an error above, so this cannot
		// be anything else.
		return nil, true, out[1].Interface().(error)
	}
	return out[0].Interface(), true, nil
}

var (
	contextType   = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType     = reflect.TypeOf((*error)(nil)).Elem()
	argumentsType = reflect.TypeOf(schema.Arguments{})
)

// fieldIndexCache remembers where each GraphQL field lives in a Go struct.
//
// Working it out means walking the struct's fields and reading their tags,
// which would otherwise happen once per field per object in every response.
var fieldIndexCache sync.Map // reflect.Type -> map[string][]int

// fieldIndexOf finds the struct field standing for a GraphQL field.
func fieldIndexOf(t reflect.Type, name string) ([]int, bool) {
	byName, cached := fieldIndexCache.Load(t)
	if !cached {
		byName, _ = fieldIndexCache.LoadOrStore(t, indexStruct(t))
	}
	index, found := byName.(map[string][]int)[strings.ToLower(name)]
	return index, found
}

// indexStruct works out which GraphQL field each struct field answers to.
//
// An embedded struct is walked as though its fields were written out, which is
// how a Go type composed of parts stands for a GraphQL type that implements an
// interface.
func indexStruct(t reflect.Type) map[string][]int {
	byName := map[string][]int{}
	var walk func(t reflect.Type, prefix []int)
	walk = func(t reflect.Type, prefix []int) {
		for i := range t.NumField() {
			field := t.Field(i)
			at := append(append([]int{}, prefix...), i)

			if field.Anonymous && !field.IsExported() {
				// An embedded unexported struct still contributes its exported
				// fields, which is how composition by embedding works.
				embedded := field.Type
				for embedded.Kind() == reflect.Pointer {
					embedded = embedded.Elem()
				}
				if embedded.Kind() == reflect.Struct {
					walk(embedded, at)
				}
				continue
			}
			if !field.IsExported() {
				continue
			}

			tag, tagged := field.Tag.Lookup("graphql")
			if tagged {
				tag, _, _ = strings.Cut(tag, ",")
				// A field tagged "-" is deliberately not exposed.
				if tag == "-" {
					continue
				}
				if tag != "" {
					byName[strings.ToLower(tag)] = at
					continue
				}
			}
			if field.Anonymous {
				embedded := field.Type
				for embedded.Kind() == reflect.Pointer {
					embedded = embedded.Elem()
				}
				if embedded.Kind() == reflect.Struct {
					walk(embedded, at)
					continue
				}
			}
			// An outer field wins over one of the same name reached through
			// embedding, which is how Go resolves the same ambiguity.
			key := strings.ToLower(field.Name)
			if existing, taken := byName[key]; !taken || len(at) < len(existing) {
				byName[key] = at
			}
		}
	}
	walk(t, nil)
	return byName
}

// exported returns the name a Go method would have for a GraphQL field.
func exported(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// ResolverFor is a convenience for writing a resolver that only needs the
// source, which most do.
func ResolverFor[Source, Result any](f func(context.Context, Source) (Result, error)) schema.FieldResolver {
	return func(ctx context.Context, source any, _ schema.Arguments, info *schema.ResolveInfo) (any, error) {
		typed, ok := source.(Source)
		if !ok {
			var zero Source
			return nil, fmt.Errorf("%s.%s: source is %T, want %T",
				info.ParentType.Name(), info.FieldName, source, zero)
		}
		return f(ctx, typed)
	}
}
