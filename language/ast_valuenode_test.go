package language

import "testing"

// A node type of a caller's own, held by value rather than by pointer. The
// Node interface does not stop one being written.
type valueNode struct{ n string }

func (valueNode) Kind() Kind          { return KindName }
func (valueNode) Location() *Location { return nil }

// Node is an ordinary interface: nothing stops a caller satisfying it with a
// value rather than a pointer. Everything that takes a Node has to cope, since
// the check for an absent node is what would otherwise panic on one.
func TestNodeHeldByValue(t *testing.T) {
	for _, name := range []string{"Print", "Visit", "Transform"} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC: %v", r)
				}
			}()
			var node Node = valueNode{n: "x"}
			switch name {
			case "Print":
				_ = Print(node)
			case "Visit":
				Visit(node, Visitor{})
			case "Transform":
				_, _ = Transform(node, Transformer{})
			}
		})
	}
}
