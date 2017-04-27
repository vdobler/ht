package mock

import (
	"testing"

	"github.com/vdobler/ht/scope"
)

type mapTest struct {
	vars    scope.Variables
	mapping Mapping
	want    string
}

var mapTests = []mapTest{
	{
		scope.Variables{"X": "foo"},
		Mapping{Variable: "A", BasedOn: "X",
			To: map[string]string{"foo": "bar"}},
		"bar",
	},
	{
		scope.Variables{"X": "foo"},
		Mapping{Variable: "A", BasedOn: "X",
			To: map[string]string{"zzz": "bar"}},
		"-undefined-",
	},
	{
		scope.Variables{"X": "foo"},
		Mapping{Variable: "A", BasedOn: "X",
			To: map[string]string{"zzz": "bar", "*": "quz"}},
		"quz",
	},
	{
		scope.Variables{"X": "foo"},
		Mapping{Variable: "A", BasedOn: "K",
			To: map[string]string{"foo": "bar"}},
		"-undefined-",
	},
}

func TestMapping(t *testing.T) {
	for i, tc := range mapTests {
		name, value := tc.mapping.Lookup(tc.vars)
		if name != tc.mapping.Variable {
			t.Errorf("%d. Bad name, got %q, want %q",
				i, name, tc.mapping.Variable)
		}
		if value != tc.want {
			t.Errorf("%d. Bad value, got %q, want %q",
				i, value, tc.want)
		}
	}
}
