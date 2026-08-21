package graphql_test

// These compare this implementation with graphql-js by running both over one
// corpus and diffing what they answered.
//
// The answers graphql-js gave are recorded under testdata/differential, so
// these tests need nothing but Go. Regenerating the recording needs Node; see
// testdata/differential/README.md.
//
// A difference this implementation makes on purpose belongs in known below,
// with its reason, and is asserted to *still* differ — closing one cannot go
// unnoticed, which is how the ported test suites are arranged too.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
	"github.com/ikawaha/graphql/value"
)

// pieceOrder is why one payload carries its pieces in a different order. The
// same fragments, ids, subPaths and data are delivered in the same payload;
// what differs is the order two of them sit in within it, which graphql-js
// arrives at through the order its own queue released them.
const pieceOrder = "the pieces of one payload are in a different order"

// fixtures is where the corpus and the recording of graphql-js's answers live.
const fixtures = "testdata/differential"

// known are the cases this implementation answers differently on purpose. The
// key is "<corpus>/<case>".
var known = map[string]string{
	"incremental/defer: Initiates deferred grouped field sets only if they have been released as pending": pieceOrder,
}

// read loads a corpus file and the answers graphql-js gave for it.
func read[C any, E any](t *testing.T, name string) ([]C, []E) {
	t.Helper()
	var cases []C
	var expected []E
	for _, part := range []struct {
		dir  string
		into any
	}{{"corpus", &cases}, {"expected", &expected}} {
		path := filepath.Join(fixtures, part.dir, name+".json")
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		if err := json.Unmarshal(text, part.into); err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
	}
	if len(cases) != len(expected) {
		t.Fatalf("%s: %d cases but %d recorded answers; regenerate the recording",
			name, len(cases), len(expected))
	}
	return cases, expected
}

// recordingFor loads a recording that answers a corpus kept under another
// name, which is how the schema corpus is asked two different questions.
func recordingFor[E any](t *testing.T, name string, cases int) []E {
	t.Helper()
	path := filepath.Join(fixtures, "expected", name+".json")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var expected []E
	if err := json.Unmarshal(text, &expected); err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(expected) != cases {
		t.Fatalf("%s: %d cases but %d recorded answers; regenerate the recording",
			name, cases, len(expected))
	}
	return expected
}

// diff reports one field of one case, honouring the known differences.
type diff struct {
	t     *testing.T
	key   string
	found bool
}

func newDiff(t *testing.T, corpus, name string) *diff {
	return &diff{t: t, key: corpus + "/" + name}
}

// says compares one thing the two implementations answered.
func (d *diff) says(what, got, want string) {
	if got == want {
		return
	}
	d.found = true
	if why, listed := known[d.key]; listed {
		d.t.Logf("known difference (%s)%s", why, sideBySide(got, want))
		return
	}
	d.t.Errorf("%s%s", what, sideBySide(got, want))
}

// sideBySide shows where two answers part company. A whole introspection
// result is thousands of lines, and dumping both leaves the reader to find the
// one that differs.
func sideBySide(got, want string) string {
	ours, theirs := strings.Split(got, "\n"), strings.Split(want, "\n")
	if len(ours) < 4 && len(theirs) < 4 {
		return fmt.Sprintf("\n   go: %q\n   js: %q", got, want)
	}
	var b strings.Builder
	shown := 0
	for i := 0; i < len(ours) || i < len(theirs); i++ {
		mine, yours := line(ours, i), line(theirs, i)
		if mine == yours {
			continue
		}
		if shown++; shown > 8 {
			fmt.Fprintf(&b, "\n   … and more, %d lines against %d", len(ours), len(theirs))
			break
		}
		fmt.Fprintf(&b, "\n   line %d\n     go: %s\n     js: %s", i+1, mine, yours)
	}
	if shown == 0 {
		return fmt.Sprintf("\n   the same lines, %d against %d", len(ours), len(theirs))
	}
	return b.String()
}

func line(lines []string, i int) string {
	if i >= len(lines) {
		return "(nothing)"
	}
	return strings.TrimRight(lines[i], " ")
}

// done fails a case listed as differing that no longer does.
func (d *diff) done() {
	if why, listed := known[d.key]; listed && !d.found {
		d.t.Errorf("this case now matches graphql-js; remove it from known (%s)", why)
	}
}

type documentCase struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type documentAnswer struct {
	Name      string `json:"name"`
	Printed   string `json:"printed"`
	Reprinted string `json:"reprinted"`
	Error     string `json:"error"`
}

// TestDocuments is L1: a document parsed and written back out. It needs no
// schema, so it reaches the whole of the lexer, the parser and the printer
// for very little.
func TestDocuments(t *testing.T) {
	cases, expected := read[documentCase, documentAnswer](t, "documents")
	for i, tt := range cases {
		want := expected[i]
		if tt.Name != want.Name {
			t.Fatalf("case %d is %q but the recording is for %q", i, tt.Name, want.Name)
		}
		t.Run(tt.Name, func(t *testing.T) {
			d := newDiff(t, "documents", tt.Name)
			defer d.done()

			doc, err := language.ParseString(tt.Source)
			if err != nil {
				d.says("parsing", err.Error(), want.Error)
				return
			}
			printed := language.Print(doc)
			d.says("printed", printed, want.Printed)

			again, err := language.ParseString(printed)
			if err != nil {
				d.says("reparsing what was printed", "error: "+err.Error(), want.Reprinted)
				return
			}
			d.says("printed again", language.Print(again), want.Reprinted)
		})
	}
}

type schemaCase struct {
	Name string `json:"name"`
	SDL  string `json:"sdl"`
}

type schemaAnswer struct {
	Name     string   `json:"name"`
	Printed  string   `json:"printed"`
	Problems []string `json:"problems"`
	Error    string   `json:"error"`
}

// TestSchemas is L2: a schema built from SDL, written back out, and asked what
// is wrong with it.
func TestSchemas(t *testing.T) {
	cases, expected := read[schemaCase, schemaAnswer](t, "schemas")
	for i, tt := range cases {
		want := expected[i]
		if tt.Name != want.Name {
			t.Fatalf("case %d is %q but the recording is for %q", i, tt.Name, want.Name)
		}
		t.Run(tt.Name, func(t *testing.T) {
			d := newDiff(t, "schemas", tt.Name)
			defer d.done()

			built, err := utilities.BuildSchema(tt.SDL)
			if err != nil {
				d.says("building", err.Error(), want.Error)
				return
			}
			d.says("printed", utilities.PrintSchema(built), want.Printed)

			var problems []string
			for _, why := range schema.ValidateSchema(built) {
				problems = append(problems, why.Message)
			}
			d.says("what is wrong with it", join(problems), join(want.Problems))
		})
	}
}

// join renders a list of complaints as one string, so that a difference in how
// many there are reads as plainly as a difference in what they say.
func join(problems []string) string {
	if len(problems) == 0 {
		return "(nothing)"
	}
	text, err := json.MarshalIndent(problems, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(text)
}

type validationCase struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

// at is one place a complaint points at.
type at struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type problem struct {
	Message   string `json:"message"`
	Locations []at   `json:"locations"`
}

type validationAnswer struct {
	Name       string    `json:"name"`
	ParseError string    `json:"parseError"`
	Problems   []problem `json:"problems"`
}

// TestValidations is L3: a document checked against graphql-js's own harness
// schema, compared message for message.
//
// This is the layer the ported suites leave least covered. They compare how
// many errors there are and where each points, which decision 13 settled on;
// what each says is checked here instead, against what graphql-js said.
func TestValidations(t *testing.T) {
	cases, expected := read[validationCase, validationAnswer](t, "validations")
	sdl, err := os.ReadFile(filepath.Join(fixtures, "corpus", "harness.graphql"))
	if err != nil {
		t.Fatalf("reading the harness schema: %v", err)
	}
	harness, err := utilities.BuildSchema(string(sdl))
	if err != nil {
		t.Fatalf("building the harness schema: %v", err)
	}

	for i, tt := range cases {
		want := expected[i]
		if tt.Name != want.Name {
			t.Fatalf("case %d is %q but the recording is for %q", i, tt.Name, want.Name)
		}
		t.Run(tt.Name, func(t *testing.T) {
			d := newDiff(t, "validations", tt.Name)
			defer d.done()

			// Fragment arguments are experimental, and the documents taken
			// from graphql-js's own tests use them; both sides read them.
			doc, err := language.ParseString(tt.Query, language.ExperimentalFragmentArguments())
			if err != nil {
				d.says("parsing", err.Error(), want.ParseError)
				return
			}
			var got []problem
			for _, why := range validation.Validate(harness, doc) {
				found := problem{Message: why.Message}
				for _, where := range why.Locations {
					found.Locations = append(found.Locations, at{where.Line, where.Column})
				}
				got = append(got, found)
			}
			d.says("what is wrong with it", render(got), render(want.Problems))
		})
	}
}

// render lays a list of complaints out so that a difference reads plainly.
func render(problems []problem) string {
	if len(problems) == 0 {
		return "(nothing)"
	}
	text, err := json.MarshalIndent(problems, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(text)
}

type executionCase struct {
	Name          string                      `json:"name"`
	SDL           string                      `json:"sdl"`
	Query         string                      `json:"query"`
	OperationName string                      `json:"operationName"`
	Variables     map[string]value.Maybe[any] `json:"variables"`
	Root          json.RawMessage             `json:"root"`
	EchoArgs      bool                        `json:"echoArgs"`
}

type executionAnswer struct {
	Name   string          `json:"name"`
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message   string `json:"message"`
		Path      []any  `json:"path"`
		Locations []at   `json:"locations"`
	} `json:"errors"`
	Error string `json:"error"`
}

// TestExecutions is L4: a document run against a schema, compared response for
// response.
//
// Every field resolves from plain data in the root value, so each
// implementation uses its own default resolver and neither is handed one the
// other could not have. Nothing here is asynchronous: what is compared is the
// input and the output, not how either got there.
func TestExecutions(t *testing.T) {
	runExecutions(t, "executions")
}

// TestCoercions is L4 over the generated corpus: every scalar, every wrapper
// and every shape of value, reached once through a variable and once through a
// literal. See generate.mjs.
func TestCoercions(t *testing.T) {
	runExecutions(t, "coercions")
}

func runExecutions(t *testing.T, corpus string) {
	t.Helper()
	cases, expected := read[executionCase, executionAnswer](t, corpus)
	for i, tt := range cases {
		want := expected[i]
		if tt.Name != want.Name {
			t.Fatalf("case %d is %q but the recording is for %q", i, tt.Name, want.Name)
		}
		t.Run(tt.Name, func(t *testing.T) {
			d := newDiff(t, corpus, tt.Name)
			defer d.done()

			built, err := utilities.BuildSchema(tt.SDL)
			if err != nil {
				d.says("building the schema", "error: "+err.Error(), want.Error)
				return
			}
			var root any
			if len(tt.Root) > 0 {
				decoder := json.NewDecoder(bytes.NewReader(tt.Root))
				decoder.UseNumber()
				if err := decoder.Decode(&root); err != nil {
					t.Fatalf("reading the root value: %v", err)
				}
			}
			params := graphql.Params{
				Schema:        built,
				Query:         tt.Query,
				OperationName: tt.OperationName,
				Variables:     tt.Variables,
				RootValue:     root,
			}
			if tt.EchoArgs {
				params.FieldResolver = echoArgs
			}
			got := graphql.Do(context.Background(), params)
			d.says("the response", response(t, got), recorded(t, want))
		})
	}
}

// echoArgs answers a field with what its arguments came to, so that the result
// of coercing an input is visible and not merely its failure. The keys are
// sorted, since one implementation's map has an order and the other's does not.
func echoArgs(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
	text, err := json.Marshal(canonical(args.Raw()))
	if err != nil {
		return nil, err
	}
	return string(text), nil
}

// canonical puts an object's keys in name order, which is what the runner does
// on the other side before it stringifies the same arguments.
//
// What this corpus asks is whether the two implementations coerced a value to
// the same thing, and the order the keys come back in is not part of that: a
// value that arrived as JSON keeps the order the request wrote, and one the
// coercion built has whatever order the implementation builds objects in.
// Sorting both sides asks the question this layer means to ask; what each
// writes an object as when it names one in a message is compared everywhere a
// message is compared.
func canonical(v any) any {
	switch held := v.(type) {
	case *value.OrderedMap:
		if held == nil {
			return nil
		}
		out := make(map[string]any, held.Len())
		for k, inner := range held.All() {
			out[k] = canonical(inner)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(held))
		for k, inner := range held {
			out[k] = canonical(inner)
		}
		return out
	case []any:
		out := make([]any, len(held))
		for i, inner := range held {
			out[i] = canonical(inner)
		}
		return out
	default:
		return v
	}
}

// response renders what this implementation answered in the shape the
// recording holds, so that the two can be compared as text.
func response(t *testing.T, got graphql.Result) string {
	t.Helper()
	shape := map[string]any{}
	if data, ran := got.Data.Get(); ran {
		shape["data"] = data
	}
	errs := []any{}
	for _, why := range got.Errors {
		where := []any{}
		for _, loc := range why.Locations {
			where = append(where, map[string]any{"line": loc.Line, "column": loc.Column})
		}
		errs = append(errs, map[string]any{
			"message": why.Message, "path": why.Path, "locations": where,
		})
	}
	shape["errors"] = errs
	return marshal(t, shape)
}

// recorded renders what graphql-js answered in the same shape.
func recorded(t *testing.T, want executionAnswer) string {
	t.Helper()
	shape := map[string]any{}
	if len(want.Data) > 0 && string(want.Data) != "null" {
		// Into an OrderedMap, not a plain map: a response's keys are in the
		// order the document asked for them, and a Go map would sort them.
		data := &value.OrderedMap{}
		if err := json.Unmarshal(want.Data, data); err != nil {
			t.Fatalf("reading the recorded data: %v", err)
		}
		shape["data"] = data
	} else if len(want.Data) > 0 {
		shape["data"] = nil
	}
	errs := []any{}
	for _, why := range want.Errors {
		where := []any{}
		for _, loc := range why.Locations {
			where = append(where, map[string]any{"line": loc.Line, "column": loc.Column})
		}
		errs = append(errs, map[string]any{
			"message": why.Message, "path": why.Path, "locations": where,
		})
	}
	shape["errors"] = errs
	return marshal(t, shape)
}

func marshal(t *testing.T, shape map[string]any) string {
	t.Helper()
	// data first, then errors, and each written by whatever holds it: an
	// OrderedMap keeps the order the document asked for.
	parts := make([]string, 0, 2)
	for _, key := range []string{"data", "errors"} {
		held, present := shape[key]
		if !present {
			continue
		}
		text, err := json.MarshalIndent(held, "  ", "  ")
		if err != nil {
			t.Fatalf("writing the response out: %v", err)
		}
		parts = append(parts, "  "+strconv.Quote(key)+": "+string(text))
	}
	return "{\n" + strings.Join(parts, ",\n") + "\n}"
}

type introspectionAnswer struct {
	Name     string          `json:"name"`
	Answered json.RawMessage `json:"answered"`
	Rebuilt  string          `json:"rebuilt"`
	Error    string          `json:"error"`
}

// TestIntrospections is L5: what a schema says about itself, and what that
// rebuilds into.
//
// One introspection query, frozen in the corpus, is put to both: asking each
// its own question would compare two different answers. The response is
// compared whole, key order included, since that is the order the document
// asked for — an answer that agrees field for field but writes an empty list
// where the other writes nothing is not the same answer, and a client rebuilds
// a different schema from it.
func TestIntrospections(t *testing.T) {
	cases, _ := read[schemaCase, schemaAnswer](t, "schemas")
	expected := recordingFor[introspectionAnswer](t, "introspections", len(cases))
	query, err := os.ReadFile(filepath.Join(fixtures, "corpus", "introspection-query.graphql"))
	if err != nil {
		t.Fatalf("reading the introspection query: %v", err)
	}

	for i, tt := range cases {
		want := expected[i]
		if tt.Name != want.Name {
			t.Fatalf("case %d is %q but the recording is for %q", i, tt.Name, want.Name)
		}
		t.Run(tt.Name, func(t *testing.T) {
			d := newDiff(t, "introspections", tt.Name)
			defer d.done()

			built, err := utilities.BuildSchema(tt.SDL)
			if err != nil {
				d.says("building the schema", "error: "+err.Error(), want.Error)
				return
			}
			asked := graphql.Do(context.Background(), graphql.Params{
				Schema: built, Query: string(query),
			})
			if len(asked.Errors) > 0 {
				d.says("asking the schema about itself", asked.Errors[0].Message, want.Error)
				return
			}
			if want.Error != "" {
				d.says("asking the schema about itself", "(no complaint)", want.Error)
				return
			}
			answered, ran := asked.Data.Get()
			if !ran {
				d.says("asking the schema about itself", "(no data)", "an answer")
				return
			}
			d.says("what the schema said about itself",
				marshal(t, map[string]any{"data": answered}),
				marshal(t, map[string]any{"data": ordered(t, want.Answered)}))

			// And the round trip a client makes: rebuild from the answer and
			// print, which is where anything the answer lost shows up.
			result, err := utilities.IntrospectionResultFrom(answered)
			if err != nil {
				d.says("reading the answer", "error: "+err.Error(), want.Rebuilt)
				return
			}
			rebuilt, err := utilities.BuildClientSchema(result)
			if err != nil {
				d.says("rebuilding from the answer", "error: "+err.Error(), want.Rebuilt)
				return
			}
			d.says("the schema rebuilt from the answer", utilities.PrintSchema(rebuilt), want.Rebuilt)
		})
	}
}

// ordered decodes recorded JSON while keeping the order its keys were written
// in, which a Go map would lose.
func ordered(t *testing.T, raw json.RawMessage) *value.OrderedMap {
	t.Helper()
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	held := &value.OrderedMap{}
	if err := json.Unmarshal(raw, held); err != nil {
		t.Fatalf("reading the recorded answer: %v", err)
	}
	return held
}

type incrementalCase struct {
	Name  string          `json:"name"`
	Query string          `json:"query"`
	Root  json.RawMessage `json:"root"`
}

type incrementalAnswer struct {
	Name     string            `json:"name"`
	Payloads []json.RawMessage `json:"payloads"`
	Error    string            `json:"error"`
}

// TestIncremental is L6: a document that asks for parts of the response to
// arrive later, compared payload for payload.
//
// The whole sequence is compared, not only what each payload says: how many
// there are and in what order is the shape a client is waiting for. Only
// documents whose data is already there are in the corpus, so neither
// implementation is waiting on anything and the sequence is settled.
func TestIncremental(t *testing.T) {
	cases, expected := read[incrementalCase, incrementalAnswer](t, "incremental")
	sdl, err := os.ReadFile(filepath.Join(fixtures, "corpus", "incremental.graphql"))
	if err != nil {
		t.Fatalf("reading the schema: %v", err)
	}
	built, err := utilities.BuildSchema(string(sdl))
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	for i, tt := range cases {
		want := expected[i]
		if tt.Name != want.Name {
			t.Fatalf("case %d is %q but the recording is for %q", i, tt.Name, want.Name)
		}
		t.Run(tt.Name, func(t *testing.T) {
			d := newDiff(t, "incremental", tt.Name)
			defer d.done()

			var root any
			if len(tt.Root) > 0 {
				decoder := json.NewDecoder(bytes.NewReader(tt.Root))
				decoder.UseNumber()
				if err := decoder.Decode(&root); err != nil {
					t.Fatalf("reading the root value: %v", err)
				}
			}
			got := graphql.DoIncrementally(context.Background(), graphql.Params{
				Schema: built, Query: tt.Query, RootValue: root,
				FieldResolver: failing,
				// What is compared here is execution. graphql-js's own runner
				// calls the executor straight, without checking the document,
				// and a document checked on one side only is not the same
				// input; validation is compared by TestValidations.
				SkipValidation: true,
			})

			payloads := []any{got.Initial}
			// Nothing deferred leaves the channel nil, and ranging over a nil
			// channel waits for ever.
			if got.Subsequent != nil {
				for part := range got.Subsequent {
					payloads = append(payloads, part)
				}
			}
			d.says("the payloads", sequence(t, payloads), recordedSequence(t, want.Payloads))
		})
	}
}

// failing is the Go side of the corpus sentinel: a field whose value says it
// fails does, with the message the corpus gave. graphql-js's tests write a
// resolver that throws; a corpus holds data, so the corpus says which field
// fails and each implementation raises it its own way.
func failing(ctx context.Context, source any, args schema.Arguments, info *schema.ResolveInfo) (any, error) {
	held, err := execution.DefaultResolver(ctx, source, args, info)
	if err != nil {
		return nil, err
	}
	if fields, isObject := held.(map[string]any); isObject {
		if why, says := fields["__throw"].(string); says {
			return nil, errors.New(why)
		}
	}
	return held, nil
}

// sequence renders a run of payloads as one text, so that a difference in how
// many there are reads as plainly as a difference in what one says.
func sequence(t *testing.T, payloads []any) string {
	t.Helper()
	parts := make([]string, 0, len(payloads))
	for _, payload := range payloads {
		text, err := json.MarshalIndent(payload, "  ", "  ")
		if err != nil {
			t.Fatalf("writing a payload out: %v", err)
		}
		parts = append(parts, "  "+string(text))
	}
	return "[\n" + strings.Join(parts, ",\n") + "\n]"
}

// recordedSequence renders what graphql-js answered in the same shape, keeping
// the order each payload's keys were written in.
func recordedSequence(t *testing.T, payloads []json.RawMessage) string {
	t.Helper()
	held := make([]any, 0, len(payloads))
	for _, raw := range payloads {
		held = append(held, ordered(t, raw))
	}
	return sequence(t, held)
}

// TestLegacyIncremental puts the same documents through the payload format
// that came before the current one, which graphql-js still answers with and so
// does this. A client written against the earlier draft sees these.
func TestLegacyIncremental(t *testing.T) {
	cases, _ := read[incrementalCase, incrementalAnswer](t, "incremental")
	expected := recordingFor[incrementalAnswer](t, "legacy-incremental", len(cases))
	sdl, err := os.ReadFile(filepath.Join(fixtures, "corpus", "incremental.graphql"))
	if err != nil {
		t.Fatalf("reading the schema: %v", err)
	}
	built, err := utilities.BuildSchema(string(sdl))
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	for i, tt := range cases {
		want := expected[i]
		if tt.Name != want.Name {
			t.Fatalf("case %d is %q but the recording is for %q", i, tt.Name, want.Name)
		}
		t.Run(tt.Name, func(t *testing.T) {
			d := newDiff(t, "legacy-incremental", tt.Name)
			defer d.done()

			var root any
			if len(tt.Root) > 0 {
				decoder := json.NewDecoder(bytes.NewReader(tt.Root))
				decoder.UseNumber()
				if err := decoder.Decode(&root); err != nil {
					t.Fatalf("reading the root value: %v", err)
				}
			}
			got := graphql.DoLegacyIncrementally(context.Background(), graphql.Params{
				Schema: built, Query: tt.Query, RootValue: root,
				FieldResolver:  failing,
				SkipValidation: true,
			})

			payloads := []any{got.Initial}
			if got.Subsequent != nil {
				for part := range got.Subsequent {
					payloads = append(payloads, part)
				}
			}
			d.says("the payloads", sequence(t, payloads), recordedSequence(t, want.Payloads))
		})
	}
}

type subscriptionCase struct {
	Name   string            `json:"name"`
	Field  string            `json:"field"`
	Query  string            `json:"query"`
	Events []json.RawMessage `json:"events"`
}

// TestSubscriptions compares one response per event.
//
// The corpus carries each event as the root field's value, which is what this
// executor hands a subscriber; graphql-js makes the event the root value and
// resolves the root field from it, so the recording was made with each event
// wrapped. That difference is deliberate and recorded in COMPATIBILITY.md;
// what is compared here is everything after it.
func TestSubscriptions(t *testing.T) {
	cases, expected := read[subscriptionCase, incrementalAnswer](t, "subscriptions")
	sdl, err := os.ReadFile(filepath.Join(fixtures, "corpus", "subscription.graphql"))
	if err != nil {
		t.Fatalf("reading the schema: %v", err)
	}
	built, err := utilities.BuildSchema(string(sdl))
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	for i, tt := range cases {
		want := expected[i]
		if tt.Name != want.Name {
			t.Fatalf("case %d is %q but the recording is for %q", i, tt.Name, want.Name)
		}
		t.Run(tt.Name, func(t *testing.T) {
			d := newDiff(t, "subscriptions", tt.Name)
			defer d.done()

			events := make(chan any, len(tt.Events))
			for _, raw := range tt.Events {
				var event any
				decoder := json.NewDecoder(bytes.NewReader(raw))
				decoder.UseNumber()
				if err := decoder.Decode(&event); err != nil {
					t.Fatalf("reading an event: %v", err)
				}
				events <- event
			}
			close(events)

			doc, err := language.ParseString(tt.Query)
			if err != nil {
				d.says("parsing", err.Error(), want.Error)
				return
			}
			got := execution.Subscribe(context.Background(), execution.Request{
				Schema:    built,
				Document:  doc,
				RootValue: map[string]any{tt.Field: (<-chan any)(events)},
			})
			if len(got.Errors) > 0 {
				d.says("starting the subscription", got.Errors[0].Message, want.Error)
				return
			}
			payloads := []any{}
			for payload := range got.Events {
				payloads = append(payloads, payload)
			}
			d.says("the payloads", sequence(t, payloads), recordedSequence(t, want.Payloads))
		})
	}
}
