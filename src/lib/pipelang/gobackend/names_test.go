package gobackend

import (
	"strings"
	"testing"
)

func TestPackageNamespaceIncludesEveryDeclarationKind(t *testing.T) {
	const support = `package generated
import "unicode/utf8"
import (s "strings"; _ "sort")
type Carrier[T any] struct { Field T }
type (First struct{}; Second interface { Method() })
const (Failure = "x"; Other, Last = "y", "z")
var (Table = [...]string{"type Fake struct{}", "func Hidden() {}"}; Left, Right int)
func Helper[T any](value Carrier[T]) Carrier[T] { local := value; return local }
func (Carrier[T]) Method() {}
func (First) Method() {}
`
	names, err := packageNames([]byte(support))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"utf8", "s", "Carrier", "First", "Second", "Failure", "Other", "Last", "Table", "Left", "Right", "Helper"}
	if len(names) != len(want) {
		t.Fatalf("package names: %v", names)
	}
	for _, name := range want {
		if !names[name] {
			t.Errorf("missing %s", name)
		}
		if _, err := packageNames([]byte(support + "\nfunc " + name + "() {}\n")); err == nil || !strings.Contains(err.Error(), "duplicate generated package name") {
			t.Errorf("duplicate %s was not rejected: %v", name, err)
		}
	}
}
