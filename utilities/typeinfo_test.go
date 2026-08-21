package utilities_test

import (
	"slices"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
)

// TypeInfo and the calls around it are re-exported here under the names
// graphql-js gives them: the implementation had to move below the schema
// builder, because the builder checks the documents it is given and the rules
// that check them need to know where in the schema they are. A caller reaches
// for these names, so these are what is checked.
func TestTypeInfo_TheNamesThisPackageGives(t *testing.T) {
	s := mustBuild(t, `
		type User { name: String }
		type Query { me: User }
	`)
	doc, err := language.ParseString(`{ me { name } }`)
	if err != nil {
		t.Fatal(err)
	}

	var seen []string
	info := utilities.NewTypeInfoForDocument(s, doc)
	language.Visit(doc, utilities.VisitWithTypeInfo(info, language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if field, ok := node.(*language.Field); ok {
				seen = append(seen, field.Name.Value+":"+info.Type().String())
			}
			return language.VisitContinue
		},
	}))
	if want := []string{"me:User", "name:String"}; !slices.Equal(seen, want) {
		t.Errorf("the walk read %v, want %v", seen, want)
	}

	// The other constructor is for a walk with no document's fragments to
	// follow, and knows nothing until the walk starts.
	if fresh := utilities.NewTypeInfo(s); fresh.Type() != nil {
		t.Errorf("a walk that has not begun is at %v, want nowhere", fresh.Type())
	}

	// And a reference written in a document resolves against the schema.
	found, ok := utilities.TypeFromAST(s, &language.NamedType{Name: &language.Name{Value: "User"}})
	if !ok || found != s.Type("User") {
		t.Errorf("TypeFromAST = %v, %v; want the schema's User", found, ok)
	}
}
