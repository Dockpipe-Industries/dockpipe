package secretenv

import "testing"

func TestResponseRequiresExactBindings(t *testing.T) {
	request := Request{Schema: Schema, Bindings: map[string]string{"TOKEN": "key"}}
	for _, values := range []map[string]string{
		{},
		{"OTHER": "value"},
		{"TOKEN": "value", "EXTRA": "value"},
		{"TOKEN": "invalid\x00value"},
	} {
		if ValidateResponse(request, Response{Schema: Schema, Values: values}) == nil {
			t.Fatal("unsafe response accepted")
		}
	}
	if err := ValidateResponse(request, Response{Schema: Schema, Values: map[string]string{"TOKEN": ""}}); err != nil {
		t.Fatal("empty values must remain distinguishable from missing bindings")
	}
}

func TestInvalidBindingFails(t *testing.T) {
	for _, bindings := range []map[string]string{
		{}, {"BAD-NAME": "key"}, {"TOKEN": ""}, {"TOKEN": "key\x00suffix"},
	} {
		if ValidateBindings(bindings) == nil {
			t.Fatal("invalid binding accepted")
		}
	}
}
