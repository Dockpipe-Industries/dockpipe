package pipelang

// This package is compiled once per bundle. It owns only fixture loading and
// comparisons; expected values/traces still come from the independent tree model,
// and actual results still come from the generated native Go functions.
const finiteSharedOracle = `package oracle
import ("testing"; "encoding/json"; "os"; "reflect")
type Case struct { Value string; Trace []string }
func Load(t *testing.T, name string, count int) []Case {
 t.Helper()
 data, err := os.ReadFile(name); if err != nil { t.Fatal(err) }
 var wants []Case
 if err := json.Unmarshal(data,&wants); err != nil { t.Fatal(err) }
 if len(wants) != count { t.Fatalf("oracle %s: %d vectors want %d",name,len(wants),count) }
 return wants
}
func Values(t *testing.T, name string, count int, call func(int) string) {
 t.Helper()
 for mask,want := range Load(t,name,count) {
  if got := call(mask); got != want.Value { t.Fatalf("mask %d: %q want %q",mask,got,want.Value) }
 }
}
func Traces(t *testing.T, name string, count int, call func(int) []string) {
 t.Helper()
 for mask,want := range Load(t,name,count) {
  if got := call(mask); !reflect.DeepEqual(got,want.Trace) { t.Fatalf("mask %d: %v want %v",mask,got,want.Trace) }
 }
}
`
