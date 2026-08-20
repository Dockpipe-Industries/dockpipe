// Package applicationir projects canonical PipeLang semantics and Core into a
// target-neutral, read-only application description.
package applicationir

import (
	"encoding/json"
	"fmt"
	"sort"

	"dockpipe/src/lib/pipelang"
	"dockpipe/src/lib/pipelang/coreir"
)

const Schema = "dockpipe.application.v1"

type Identity struct {
	PackageID string `json:"package_id"`
	Path      string `json:"path"`
}
type LocatedIdentity struct {
	Identity
	Source pipelang.SourceRange `json:"source"`
}
type Column struct {
	Field LocatedIdentity `json:"field"`
	Label string          `json:"label"`
}
type OrderKey struct {
	Field     LocatedIdentity `json:"field"`
	Direction string          `json:"direction"`
}
type SectionSpec struct {
	Identity     LocatedIdentity   `json:"identity"`
	ResultType   LocatedIdentity   `json:"result_type"`
	RowType      LocatedIdentity   `json:"row_type"`
	Key          LocatedIdentity   `json:"key"`
	Columns      []Column          `json:"columns"`
	FilterFields []LocatedIdentity `json:"filter_fields"`
	Order        []OrderKey        `json:"order"`
	Filter       *LocatedIdentity  `json:"filter,omitempty"`
	OrderBinding LocatedIdentity   `json:"order_binding"`
}
type Spec struct {
	Identity     LocatedIdentity  `json:"identity"`
	SnapshotType LocatedIdentity  `json:"snapshot_type"`
	Sections     []SectionSpec    `json:"sections"`
	Selection    *LocatedIdentity `json:"selection,omitempty"`
	Details      *LocatedIdentity `json:"details,omitempty"`
	Logs         *LocatedIdentity `json:"logs,omitempty"`
}
type Metadata struct {
	SemanticSchema   string `json:"semantic_schema"`
	LanguageContract string `json:"language_contract"`
	CompilerContract string `json:"compiler_contract"`
	PackageID        string `json:"package_id"`
}
type Application struct {
	Schema       string    `json:"schema"`
	Metadata     Metadata  `json:"metadata"`
	Identity     Identity  `json:"identity"`
	SnapshotType Identity  `json:"snapshot_type"`
	Sections     []Section `json:"sections"`
	Selection    *Identity `json:"selection,omitempty"`
	Details      *Identity `json:"details,omitempty"`
	Logs         *Identity `json:"logs,omitempty"`
}
type Section struct {
	Identity     Identity          `json:"identity"`
	ResultType   Identity          `json:"result_type"`
	RowType      Identity          `json:"row_type"`
	Key          Identity          `json:"key"`
	Columns      []ProjectedColumn `json:"columns"`
	FilterFields []Identity        `json:"filter_fields"`
	Order        []ProjectedOrder  `json:"order"`
	Filter       *Identity         `json:"filter,omitempty"`
	OrderBinding Identity          `json:"order_binding"`
}
type ProjectedColumn struct {
	Field Identity `json:"field"`
	Label string   `json:"label"`
}
type ProjectedOrder struct {
	Field     Identity `json:"field"`
	Direction string   `json:"direction"`
}

type ValidationError struct {
	Source  pipelang.SourceRange
	Message string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Source.File, e.Message) }

// Project validates explicit application choices against both canonical inputs.
// It never parses source or infers a missing application choice.
func Project(semantic *pipelang.SemanticProjection, core *coreir.Program, spec Spec) (*Application, error) {
	if semantic == nil || core == nil {
		return nil, &ValidationError{Message: "semantic projection and Core program are required"}
	}
	if semantic.Schema != pipelang.PipeLangSemanticProjectionVersion || semantic.View != pipelang.SemanticProjectionPublic {
		return nil, &ValidationError{Message: "public pipelang.semantic.v1 projection is required"}
	}
	if core.CompilerContract != semantic.CompilerContract || core.LanguageContract != string(semantic.LanguageContract) {
		return nil, &ValidationError{Message: "semantic projection and Core contracts do not match"}
	}
	known := map[string]bool{}
	members := map[string]pipelang.SemanticMemberProjection{}
	owners := map[string]string{}
	for _, m := range semantic.Modules {
		known[idKey(Identity{string(m.Identity.PackageID), string(m.Identity.Path)})] = true
		for _, t := range m.Types {
			owner := ""
			if t.Identity != nil {
				owner = idKey(Identity{string(t.Identity.PackageID), string(t.Identity.Path)})
				known[owner] = true
			}
			for _, x := range t.Members {
				if x.Identity != nil {
					known[idKey(Identity{string(x.Identity.PackageID), string(x.Identity.Path)})] = true
					members[idKey(Identity{string(x.Identity.PackageID), string(x.Identity.Path)})] = x
					owners[idKey(Identity{string(x.Identity.PackageID), string(x.Identity.Path)})] = owner
				}
			}
		}
	}
	coreKnown := map[string]bool{}
	for _, f := range core.Functions {
		coreKnown[idKey(Identity{f.Identity.PackageID, f.Identity.Path})] = true
	}
	require := func(v LocatedIdentity, what string) error {
		if v.PackageID == "" || v.Path == "" || !known[idKey(v.Identity)] {
			return &ValidationError{v.Source, what + " has no canonical semantic identity"}
		}
		return nil
	}
	if err := require(spec.Identity, "application"); err != nil {
		return nil, err
	}
	if !coreKnown[idKey(spec.Identity.Identity)] {
		return nil, &ValidationError{spec.Identity.Source, "application identity has no matching Core function"}
	}
	if err := require(spec.SnapshotType, "snapshot type"); err != nil {
		return nil, err
	}
	out := &Application{Schema: Schema, Metadata: Metadata{string(semantic.Schema), string(semantic.LanguageContract), semantic.CompilerContract, string(semantic.PackageID)}, Identity: spec.Identity.Identity, SnapshotType: spec.SnapshotType.Identity, Sections: []Section{}}
	seen := map[string]bool{}
	for _, s := range spec.Sections {
		if seen[idKey(s.Identity.Identity)] {
			return nil, &ValidationError{s.Identity.Source, "duplicate section identity"}
		}
		seen[idKey(s.Identity.Identity)] = true
		for what, v := range map[string]LocatedIdentity{"section": s.Identity, "result type": s.ResultType, "row type": s.RowType, "key": s.Key} {
			if err := require(v, what); err != nil {
				return nil, err
			}
		}
		resultMember, ok := members[idKey(s.ResultType.Identity)]
		if !ok || !isResultListOf(resultMember.Type, s.RowType.Identity) || !coreKnown[idKey(s.ResultType.Identity)] {
			return nil, &ValidationError{s.ResultType.Source, "section result must be Result<List<Row>, string>"}
		}
		keyMember, ok := members[idKey(s.Key.Identity)]
		if !ok || keyMember.Type.Primitive != pipelang.TypeString || owners[idKey(s.Key.Identity)] != idKey(s.RowType.Identity) {
			return nil, &ValidationError{s.Key.Source, "stable row key must be a string field"}
		}
		p := Section{Identity: s.Identity.Identity, ResultType: s.ResultType.Identity, RowType: s.RowType.Identity, Key: s.Key.Identity, Columns: []ProjectedColumn{}, FilterFields: []Identity{}, Order: []ProjectedOrder{}}
		if err := require(s.OrderBinding, "order binding"); err != nil {
			return nil, err
		}
		orderMember, ok := members[idKey(s.OrderBinding.Identity)]
		if !ok || !isOrderCallable(orderMember, s.RowType.Identity) || !coreKnown[idKey(s.OrderBinding.Identity)] {
			return nil, &ValidationError{s.OrderBinding.Source, "order binding must be a Core-backed (List<Row>) -> List<Row> method"}
		}
		p.OrderBinding = s.OrderBinding.Identity
		if s.Filter != nil {
			if err := require(*s.Filter, "filter binding"); err != nil {
				return nil, err
			}
			filterMember, ok := members[idKey(s.Filter.Identity)]
			if !ok || !isFilterCallable(filterMember, s.RowType.Identity) || !coreKnown[idKey(s.Filter.Identity)] {
				return nil, &ValidationError{s.Filter.Source, "filter binding must be a Core-backed (List<Row>, string) -> List<Row> method"}
			}
			x := s.Filter.Identity
			p.Filter = &x
		}
		if len(s.Columns) == 0 {
			return nil, &ValidationError{s.Identity.Source, "section requires at least one column"}
		}
		for _, c := range s.Columns {
			if c.Label == "" {
				return nil, &ValidationError{c.Field.Source, "column label is empty"}
			}
			if err := require(c.Field, "column field"); err != nil {
				return nil, err
			}
			if owners[idKey(c.Field.Identity)] != idKey(s.RowType.Identity) {
				return nil, &ValidationError{c.Field.Source, "column is not a field of the section row type"}
			}
			p.Columns = append(p.Columns, ProjectedColumn{c.Field.Identity, c.Label})
		}
		for _, f := range s.FilterFields {
			if err := require(f, "filter field"); err != nil {
				return nil, err
			}
			if owners[idKey(f.Identity)] != idKey(s.RowType.Identity) || members[idKey(f.Identity)].Type.Primitive != pipelang.TypeString {
				return nil, &ValidationError{f.Source, "filter must be a string field of the section row type"}
			}
			p.FilterFields = append(p.FilterFields, f.Identity)
		}
		for _, o := range s.Order {
			if o.Direction != "ascending" && o.Direction != "descending" {
				return nil, &ValidationError{o.Field.Source, "order direction must be ascending or descending"}
			}
			if err := require(o.Field, "order field"); err != nil {
				return nil, err
			}
			if owners[idKey(o.Field.Identity)] != idKey(s.RowType.Identity) || members[idKey(o.Field.Identity)].Type.Primitive != pipelang.TypeString {
				return nil, &ValidationError{o.Field.Source, "order must be a string field of the section row type"}
			}
			p.Order = append(p.Order, ProjectedOrder{o.Field.Identity, o.Direction})
		}
		out.Sections = append(out.Sections, p)
	}
	if len(out.Sections) == 0 {
		return nil, &ValidationError{spec.Identity.Source, "application requires at least one section"}
	}
	sort.SliceStable(out.Sections, func(i, j int) bool { return idKey(out.Sections[i].Identity) < idKey(out.Sections[j].Identity) })
	if spec.Selection != nil {
		if err := require(*spec.Selection, "selection"); err != nil {
			return nil, err
		}
		x := spec.Selection.Identity
		if m, ok := members[idKey(x)]; !ok || !isOptionalRecord(m.Type) || !coreKnown[idKey(x)] {
			return nil, &ValidationError{spec.Selection.Source, "selection must be an Optional<Row> field"}
		}
		out.Selection = &x
	}
	if spec.Details != nil {
		if err := require(*spec.Details, "details"); err != nil {
			return nil, err
		}
		x := spec.Details.Identity
		if m, ok := members[idKey(x)]; !ok || !isStringResult(m.Type) || !coreKnown[idKey(x)] {
			return nil, &ValidationError{spec.Details.Source, "details must be a Result<string, string> field"}
		}
		out.Details = &x
	}
	if spec.Logs != nil {
		if err := require(*spec.Logs, "logs"); err != nil {
			return nil, err
		}
		x := spec.Logs.Identity
		if m, ok := members[idKey(x)]; !ok || !isStringResult(m.Type) || !coreKnown[idKey(x)] {
			return nil, &ValidationError{spec.Logs.Source, "logs must be a Result<string, string> field"}
		}
		out.Logs = &x
	}
	return out, nil
}

func CanonicalJSON(app *Application) ([]byte, error) {
	if app == nil {
		return nil, fmt.Errorf("application IR is nil")
	}
	return json.MarshalIndent(app, "", "  ")
}
func idKey(i Identity) string { return i.PackageID + "\x00" + i.Path }

func isResultListOf(t pipelang.SemanticTypeRefProjection, row Identity) bool {
	return t.Identity != nil && t.Identity.PackageID == pipelang.PipeLangBuiltinPackageID && t.Identity.Path == pipelang.PipeLangResultSemanticPath && len(t.Arguments) == 2 &&
		t.Arguments[0].Identity != nil && t.Arguments[0].Identity.Path == pipelang.PipeLangListSemanticPath && len(t.Arguments[0].Arguments) == 1 &&
		t.Arguments[0].Arguments[0].Identity != nil && string(t.Arguments[0].Arguments[0].Identity.PackageID) == row.PackageID && string(t.Arguments[0].Arguments[0].Identity.Path) == row.Path && t.Arguments[1].Primitive == pipelang.TypeString
}
func isOptionalRecord(t pipelang.SemanticTypeRefProjection) bool {
	return t.Identity != nil && t.Identity.PackageID == pipelang.PipeLangBuiltinPackageID && t.Identity.Path == pipelang.PipeLangOptionalSemanticPath && len(t.Arguments) == 1 && t.Arguments[0].Identity != nil
}
func isStringResult(t pipelang.SemanticTypeRefProjection) bool {
	return t.Identity != nil && t.Identity.PackageID == pipelang.PipeLangBuiltinPackageID && t.Identity.Path == pipelang.PipeLangResultSemanticPath && len(t.Arguments) == 2 && t.Arguments[0].Primitive == pipelang.TypeString && t.Arguments[1].Primitive == pipelang.TypeString
}
func isListOf(t pipelang.SemanticTypeRefProjection, row Identity) bool {
	return t.Identity != nil && t.Identity.PackageID == pipelang.PipeLangBuiltinPackageID && t.Identity.Path == pipelang.PipeLangListSemanticPath && len(t.Arguments) == 1 && t.Arguments[0].Identity != nil && string(t.Arguments[0].Identity.PackageID) == row.PackageID && string(t.Arguments[0].Identity.Path) == row.Path
}
func isFilterCallable(m pipelang.SemanticMemberProjection, row Identity) bool {
	return len(m.Parameters) == 2 && isListOf(m.Parameters[0].Type, row) && m.Parameters[1].Type.Primitive == pipelang.TypeString && isListOf(m.Type, row)
}
func isOrderCallable(m pipelang.SemanticMemberProjection, row Identity) bool {
	return len(m.Parameters) == 1 && isListOf(m.Parameters[0].Type, row) && isListOf(m.Type, row)
}
