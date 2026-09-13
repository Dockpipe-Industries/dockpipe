package pipelang

import "encoding/binary"

// Exact length-prefixed strings keep fixture-only JSON support out of subset
// executables. Values and trace order still come from the independent oracle.
func encodeFiniteBinaryOracle(rows []finiteConditionalOracleCase) []byte {
	data := binary.LittleEndian.AppendUint32(nil, uint32(len(rows)))
	text := func(s string) {
		data = binary.LittleEndian.AppendUint32(data, uint32(len(s)))
		data = append(data, s...)
	}
	for _, row := range rows {
		text(row.Value)
		if row.Trace == nil {
			data = binary.LittleEndian.AppendUint32(data, ^uint32(0))
			continue
		}
		data = binary.LittleEndian.AppendUint32(data, uint32(len(row.Trace)))
		for _, entry := range row.Trace {
			text(entry)
		}
	}
	return data
}

const finiteBinaryOracle = `package oracle
import ("testing"; "encoding/binary"; "os"; "reflect")
type Case struct { Value string; Trace []string }
func Load(t *testing.T, name string, count int) []Case {
 t.Helper()
 data, err := os.ReadFile(name); if err != nil { t.Fatal(err) }
 number := func() uint32 {
  if len(data)<4 {t.Fatal("oracle truncated integer")}
  n:=binary.LittleEndian.Uint32(data); data=data[4:]; return n
 }
 text := func() string {
  n:=uint64(number()); if n>uint64(len(data)) {t.Fatal("oracle truncated string")}
  s:=string(data[:int(n)]); data=data[int(n):]; return s
 }
 if uint64(number())!=uint64(count) {t.Fatal("oracle vector count")}
 if count<0 || count>len(data)/8 {t.Fatal("oracle truncated vectors")}
 wants:=make([]Case,count)
 for i:=range wants {
  wants[i].Value=text()
  n:=uint64(number()); if n==uint64(^uint32(0)) {continue}
  if n>uint64(len(data)/4) {t.Fatal("oracle truncated trace")}
  wants[i].Trace=make([]string,int(n))
  for j:=range wants[i].Trace {wants[i].Trace[j]=text()}
 }
 if len(data)!=0 {t.Fatal("oracle trailing bytes")}
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
