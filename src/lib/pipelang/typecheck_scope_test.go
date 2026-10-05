package pipelang

import (
	"reflect"
	"sync"
	"testing"
)

func TestImmutableLocalInferenceScopeIsolation(t *testing.T) {
	analysis := analyzeHIRGraph(t, `public Class Root { public string Select(string value,bool pick){string first=value;string second=first;if(pick){string same=second;return same;}else{string same=second;return same;}}}`)
	body := methodByIdentity(analysis, semanticMethodNamed(t, analysis, "Select").Identity).Body
	for _, tc := range []struct {
		name        string
		environment map[string]ResolvedTypeRef
		wantError   bool
	}{
		{"sibling_scopes", map[string]ResolvedTypeRef{"value": resolvedPrimitive(TypeString), "pick": resolvedPrimitive(TypeBool)}, false},
		{"error_after_locals", map[string]ResolvedTypeRef{"value": resolvedPrimitive(TypeString), "pick": resolvedPrimitive(TypeString)}, true},
		{"shadow", map[string]ResolvedTypeRef{"value": resolvedPrimitive(TypeString), "pick": resolvedPrimitive(TypeBool), "second": resolvedPrimitive(TypeString)}, true},
		{"nil_scope", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before map[string]ResolvedTypeRef
			if tc.environment != nil {
				before = make(map[string]ResolvedTypeRef)
				for k, v := range tc.environment {
					before[k] = v
				}
			}
			// Independent inference calls may share the read-only caller environment.
			var group sync.WaitGroup
			for i := 0; i < 8; i++ {
				group.Add(1)
				go func() {
					defer group.Done()
					got, err := analysis.checked.inferExprType(body, tc.environment)
					if (err != nil) != tc.wantError || (err == nil && !got.Equal(resolvedPrimitive(TypeString))) {
						t.Errorf("inference: %v, %v", got, err)
					}
				}()
			}
			group.Wait()
			if !reflect.DeepEqual(tc.environment, before) {
				t.Fatal("inference mutated caller bindings")
			}
		})
	}
}
