package utilities_test

import (
	"sort"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// Ported from graphql-js src/utilities/__tests__/extendSchema-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
//
// A type carries the node it was defined by and the nodes that extended it.
// Only a type an extension touched has the latter, and every extension in the
// document has to end up on exactly one type.
func TestPortedExtendSchema_ASTNodes(t *testing.T) {
	base, err := utilities.BuildSchema(`
		type Query

		scalar SomeScalar
		enum SomeEnum
		union SomeUnion
		input SomeInput
		type SomeObject
		interface SomeInterface

		directive @foo on SCALAR
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	first := mustParseDocument(t, `
		extend type Query {
			newField(testArg: TestInput): TestEnum
		}

		extend scalar SomeScalar @foo

		extend enum SomeEnum {
			NEW_VALUE
		}

		extend union SomeUnion = SomeObject

		extend input SomeInput {
			newField: String
		}

		extend interface SomeInterface {
			newField: String
		}

		enum TestEnum {
			TEST_VALUE
		}

		input TestInput {
			testInputField: TestEnum
		}
	`)
	second := mustParseDocument(t, `
		extend type Query {
			oneMoreNewField: TestUnion
		}

		extend scalar SomeScalar @test

		extend enum SomeEnum {
			ONE_MORE_NEW_VALUE
		}

		extend union SomeUnion = TestType

		extend input SomeInput {
			oneMoreNewField: String
		}

		extend interface SomeInterface {
			oneMoreNewField: String
		}

		union TestUnion = TestType

		interface TestInterface {
			interfaceField: String
		}

		type TestType implements TestInterface {
			interfaceField: String
		}

		directive @test(arg: Int) repeatable on FIELD | SCALAR
	`)

	once, err := utilities.ExtendSchema(base, first)
	if err != nil {
		t.Fatalf("extending the schema: %v", err)
	}
	twice, err := utilities.ExtendSchema(once, second)
	if err != nil {
		t.Fatalf("extending the schema again: %v", err)
	}
	inOneGo, err := utilities.ExtendSchema(base, utilities.ConcatDocuments(first, second))
	if err != nil {
		t.Fatalf("extending the schema in one go: %v", err)
	}
	if got, want := utilities.PrintSchema(inOneGo), utilities.PrintSchema(twice); got != want {
		t.Errorf("extending twice and extending in one go gave different schemas:\ngot\n%s\nwant\n%s", got, want)
	}

	query := typeOf[*schema.ObjectType](t, twice, "Query")
	someScalar := typeOf[*schema.ScalarType](t, twice, "SomeScalar")
	someEnum := typeOf[*schema.EnumType](t, twice, "SomeEnum")
	someUnion := typeOf[*schema.UnionType](t, twice, "SomeUnion")
	someInput := typeOf[*schema.InputObjectType](t, twice, "SomeInput")
	someInterface := typeOf[*schema.InterfaceType](t, twice, "SomeInterface")

	testEnum := typeOf[*schema.EnumType](t, twice, "TestEnum")
	testInput := typeOf[*schema.InputObjectType](t, twice, "TestInput")
	testUnion := typeOf[*schema.UnionType](t, twice, "TestUnion")
	testType := typeOf[*schema.ObjectType](t, twice, "TestType")
	testInterface := typeOf[*schema.InterfaceType](t, twice, "TestInterface")
	testDirective := twice.Directive("test")
	if testDirective == nil {
		t.Fatal("@test is not in the extended schema")
	}

	// A type the document defines outright was never extended.
	for name, held := range map[string]int{
		"TestType":      len(testType.ExtensionASTNodes),
		"TestEnum":      len(testEnum.ExtensionASTNodes),
		"TestUnion":     len(testUnion.ExtensionASTNodes),
		"TestInput":     len(testInput.ExtensionASTNodes),
		"TestInterface": len(testInterface.ExtensionASTNodes),
	} {
		if held != 0 {
			t.Errorf("%s holds %d extension nodes, want none", name, held)
		}
	}

	// Between the definitions the new types carry and the extensions the old
	// ones do, every definition of both documents is accounted for once.
	accounted := []language.Node{
		testInput.ASTNode, testEnum.ASTNode, testUnion.ASTNode,
		testInterface.ASTNode, testType.ASTNode, testDirective.ASTNode,
	}
	for _, node := range query.ExtensionASTNodes {
		accounted = append(accounted, node)
	}
	for _, node := range someScalar.ExtensionASTNodes {
		accounted = append(accounted, node)
	}
	for _, node := range someEnum.ExtensionASTNodes {
		accounted = append(accounted, node)
	}
	for _, node := range someUnion.ExtensionASTNodes {
		accounted = append(accounted, node)
	}
	for _, node := range someInput.ExtensionASTNodes {
		accounted = append(accounted, node)
	}
	for _, node := range someInterface.ExtensionASTNodes {
		accounted = append(accounted, node)
	}

	var written []language.Node
	for _, doc := range []*language.Document{first, second} {
		for _, def := range doc.Definitions {
			written = append(written, def)
		}
	}
	if got, want := printedAndSorted(accounted), printedAndSorted(written); !equalStrings(got, want) {
		t.Errorf("the nodes the schema kept are not the ones the documents held:\ngot  %v\nwant %v", got, want)
	}

	// Each part of an extension keeps the node it was written as, too.
	for _, tt := range []struct {
		what string
		node language.Node
		want string
	}{
		{"Query.newField", query.Field("newField").ASTNode, "newField(testArg: TestInput): TestEnum"},
		{"Query.newField(testArg:)", query.Field("newField").Args[0].ASTNode, "testArg: TestInput"},
		{"Query.oneMoreNewField", query.Field("oneMoreNewField").ASTNode, "oneMoreNewField: TestUnion"},
		{"SomeEnum.NEW_VALUE", someEnum.Value("NEW_VALUE").ASTNode, "NEW_VALUE"},
		{"SomeEnum.ONE_MORE_NEW_VALUE", someEnum.Value("ONE_MORE_NEW_VALUE").ASTNode, "ONE_MORE_NEW_VALUE"},
		{"SomeInput.newField", someInput.Field("newField").ASTNode, "newField: String"},
		{"SomeInput.oneMoreNewField", someInput.Field("oneMoreNewField").ASTNode, "oneMoreNewField: String"},
		{"SomeInterface.newField", someInterface.Field("newField").ASTNode, "newField: String"},
		{"SomeInterface.oneMoreNewField", someInterface.Field("oneMoreNewField").ASTNode, "oneMoreNewField: String"},
		{"TestInput.testInputField", testInput.Field("testInputField").ASTNode, "testInputField: TestEnum"},
		{"TestEnum.TEST_VALUE", testEnum.Value("TEST_VALUE").ASTNode, "TEST_VALUE"},
		{"TestInterface.interfaceField", testInterface.Field("interfaceField").ASTNode, "interfaceField: String"},
		{"TestType.interfaceField", testType.Field("interfaceField").ASTNode, "interfaceField: String"},
		{"@test(arg:)", testDirective.Args[0].ASTNode, "arg: Int"},
	} {
		if tt.node == nil {
			t.Errorf("%s has no node", tt.what)
			continue
		}
		if got := language.Print(tt.node); got != tt.want {
			t.Errorf("%s was written as %q, want %q", tt.what, got, tt.want)
		}
	}
}

// typeOf looks a type up and says what kind it was wanted as.
func typeOf[T schema.NamedType](t *testing.T, s *schema.Schema, name string) T {
	t.Helper()
	found, isKind := s.Type(name).(T)
	if !isKind {
		t.Fatalf("%s is not in the schema as the kind it was looked for", name)
	}
	return found
}

func printedAndSorted(nodes []language.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			out = append(out, "<nil>")
			continue
		}
		out = append(out, language.Print(node))
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
