// Package streamir defines the experimental synchronous native-stream profile.
// It is separate from pure Core: calls are explicit host effects with borrowed
// capabilities and a typed partial-progress outcome. No file/process access occurs here.
package streamir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
)

const Profile = "pipelang.native-stream.v1"
const CompositionProfile = "pipelang.native-stream.v2"
const ABI = 1

type Type string

const (
	ReadStream  Type = "ReadStream"
	WriteStream Type = "WriteStream"
	Int         Type = "int"
	Bool        Type = "bool"
	Uint64      Type = "uint64"
	Result      Type = "StreamResult"
	StatusType  Type = "StreamStatus"
)

type Operation struct {
	Name string `json:"name"` // Source-qualified alias, e.g. Codec.encode.
	ID   string `json:"id"`   // Package-owned stable semantic identity.
}
type Manifest struct {
	Profile    string      `json:"profile"`
	Package    string      `json:"package"`
	ABI        int         `json:"abi"`
	Operations []Operation `json:"operations"`
}
type Binding struct {
	Package        string
	ManifestSHA256 string
	Operation      Operation
}
type Function struct {
	Class      string
	Name       string
	Parameters []Type
	Binding    int
	Arguments  [3]int // ReadStream, WriteStream, int parameter positions.
	Public     bool
	ReturnType Type
	Body       []Statement
}
type Program struct {
	Profile      string
	SourceSHA256 string
	Bindings     []Binding
	Functions    []Function
}

var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
var qualified = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}\.[A-Za-z][A-Za-z0-9_]{0,63}$`)
var semantic = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)

func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func ValidDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && hex.EncodeToString(b) == s
}
func ValidateManifest(m Manifest) error {
	if m.Profile != Profile || m.ABI != ABI || !semantic.MatchString(m.Package) || len(m.Operations) == 0 || len(m.Operations) > 64 {
		return fmt.Errorf("invalid native-stream profile, package, ABI or operation count")
	}
	names, ids := map[string]bool{}, map[string]bool{}
	for _, op := range m.Operations {
		if !qualified.MatchString(op.Name) || !semantic.MatchString(op.ID) || names[op.Name] || ids[op.ID] {
			return fmt.Errorf("invalid or duplicate native-stream operation")
		}
		names[op.Name] = true
		ids[op.ID] = true
	}
	return nil
}

// Validate is independent of the source checker; backends must call it even for
// caller-constructed IR. Native symbols or paths never come from source text.
func Validate(p Program) error {
	if (p.Profile != Profile && p.Profile != CompositionProfile) || !ValidDigest(p.SourceSHA256) || len(p.Bindings) == 0 || len(p.Bindings) > 64 || len(p.Functions) == 0 || len(p.Functions) > 64 {
		return fmt.Errorf("invalid native-stream program header or extent")
	}
	aliases, ids := map[string]bool{}, map[string]bool{}
	for _, b := range p.Bindings {
		if !ValidDigest(b.ManifestSHA256) {
			return fmt.Errorf("invalid manifest digest")
		}
		if err := ValidateManifest(Manifest{Profile: Profile, Package: b.Package, ABI: ABI, Operations: []Operation{b.Operation}}); err != nil {
			return err
		}
		key := b.Package + "/" + b.Operation.ID
		if aliases[b.Operation.Name] || ids[key] {
			return fmt.Errorf("ambiguous native-stream binding")
		}
		aliases[b.Operation.Name] = true
		ids[key] = true
	}
	if p.Profile == CompositionProfile {
		return validateComposition(p)
	}
	methods := map[string]bool{}
	for _, f := range p.Functions {
		if f.Body != nil || f.ReturnType != "" || f.Public {
			return fmt.Errorf("composition fields in v1 function")
		}
		key := f.Class + "." + f.Name
		if !identifier.MatchString(f.Class) || !identifier.MatchString(f.Name) || methods[key] || len(f.Parameters) != 3 || f.Binding < 0 || f.Binding >= len(p.Bindings) {
			return fmt.Errorf("invalid native-stream function")
		}
		methods[key] = true
		seen := map[int]bool{}
		for i, typ := range []Type{ReadStream, WriteStream, Int} {
			arg := f.Arguments[i]
			if arg < 0 || arg >= 3 || seen[arg] || f.Parameters[arg] != typ {
				return fmt.Errorf("native-stream call argument type or ownership mismatch")
			}
			seen[arg] = true
		}
	}
	return nil
}
