// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

var jr = Response{BodyStr: `{"foo": 5, "bar": [1,2,3]}`}
var ar = Response{BodyStr: `["jo nesbo",["jo nesbo","jo nesbo harry hole","jo nesbo sohn","jo nesbo koma","jo nesbo hörbuch","jo nesbo headhunter","jo nesbo pupspulver","jo nesbo leopard","jo nesbo schneemann","jo nesbo the son"],[{"nodes":[{"name":"Bücher","alias":"stripbooks"},{"name":"Trade-In","alias":"tradein-aps"},{"name":"Kindle-Shop","alias":"digital-text"}]},{}],[]]`}
var jre = Response{BodyStr: `{"foo": 5, "bar": [1,"qux",3], "waz": true, "nil": null, "uuid": "ad09b43c-6538-11e6-8b77-86f30ca893d3", "pi": 3.141}`}
var jrx = Response{BodyStr: `{"foo": 5, "blub...`}
var jrs = Response{BodyStr: `"foo"`}
var jri = Response{BodyStr: `123`}
var jrf = Response{BodyStr: `45.67`}
var jrm = Response{BodyStr: `"{\"foo\":5,\"bar\":[1,2,3]}"`} // "modern JSON"

var jsonExpressionTests = []TC{
	{jr, &JSONExpr{Expression: "(.foo == 5) && ($len(.bar)==3) && (.bar[1]==2)"}, nil},
	{jr, &JSONExpr{Expression: "$max(.bar) == 3"}, nil},
	{jr, &JSONExpr{Expression: "$has(.bar, 2)"}, nil},
	{jr, &JSONExpr{Expression: "$has(.bar, 7)"}, errCheck},
	{jr, &JSONExpr{Expression: ".foo == 3"}, errCheck},
	{ar, &JSONExpr{Expression: "$len(.) > 3"}, nil},
	{ar, &JSONExpr{Expression: "$len(.) == 4"}, nil},
	{ar, &JSONExpr{Expression: ".[0] == \"jo nesbo\""}, nil},
	{ar, &JSONExpr{Expression: "$len(.[1]) == 10"}, nil},
	{ar, &JSONExpr{Expression: ".[1][6] == \"jo nesbo pupspulver\""}, nil},
}

func TestJSONExpression(t *testing.T) {
	for i, tc := range jsonExpressionTests {
		runTest(t, i, tc)
	}
}

var jsonConditionTests = []TC{
	{jr, &JSON{Element: "foo", Condition: Condition{Equals: "5"}}, nil},
	{jr, &JSON{Element: "bar.1", Condition: Condition{Equals: "2"}}, nil},
	{jr, &JSON{Element: "bar.2"}, nil},
	{jr, &JSON{Element: "bar#1", Sep: "#", Condition: Condition{Equals: "2"}}, nil},
	{jr, &JSON{Element: "foo", Condition: Condition{Equals: "bar"}}, errCheck},
	{jr, &JSON{Element: "bar.5"}, fmt.Errorf("No index 5 in array bar of len 3")},
	{jr, &JSON{Element: "bar.3", Condition: Condition{Equals: "2"}}, errCheck},
	{jr, &JSON{Element: "foo.wuz", Condition: Condition{Equals: "bar"}}, errCheck},
	{jr, &JSON{Element: "qux", Condition: Condition{Equals: "bar"}},
		fmt.Errorf("Element qux not found")},

	{ar, &JSON{Element: "0", Condition: Condition{Equals: `"jo nesbo"`}}, nil},
	{ar, &JSON{Element: "1.4", Condition: Condition{Contains: `jo nesbo`}}, nil},
	{ar, &JSON{Element: "2.0.nodes.2.name", Condition: Condition{Equals: `"Kindle-Shop"`}}, nil},

	{jre, &JSON{Element: "bar.1", Condition: Condition{Equals: `"qux"`}}, nil},
	{jre, &JSON{Element: "waz", Condition: Condition{Equals: `true`}}, nil},
	{jre, &JSON{Element: "nil", Condition: Condition{Equals: `null`}}, nil},
	{jre, &JSON{Element: "nil", Condition: Condition{Prefix: `"`}}, errCheck},
	{jre, &JSON{Element: "uuid", Condition: Condition{
		Regexp: `^"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"$`}}, nil},

	{jre, &JSON{}, nil},
	{jrx, &JSON{}, errCheck},

	{jri, &JSON{Element: ".", Condition: Condition{Equals: "123"}}, nil},
	{jrf, &JSON{Element: ".", Condition: Condition{Equals: "45.67"}}, nil},
	{jrs, &JSON{Element: ".", Condition: Condition{Contains: `foo`}}, nil},

	{jrm, &JSON{Element: ".",
		Embedded: &JSON{Element: "foo", Condition: Condition{Equals: "5"}}},
		nil},
	{jrm, &JSON{Element: ".",
		Embedded: &JSON{Element: "bar.1", Condition: Condition{Equals: "2"}}},
		nil},
	{jrm, &JSON{Element: ".",
		Embedded: &JSON{Element: "bar.1", Condition: Condition{Equals: "XX"}}},
		errCheck},

	// different types of "not found"
	{jr, &JSON{Element: "waz", Condition: Condition{Contains: "needle"}},
		errors.New("Element waz not found")},
	{jr, &JSON{Element: "bar.4", Condition: Condition{Contains: "needle"}},
		errors.New("No index 4 in array bar of len 3")},
	{Response{BodyStr: `{"foo": "Some ugly long string value."}`},
		&JSON{Element: "foo", Condition: Condition{Contains: "needle"}},
		errors.New(`Cannot find "needle" in "Some ugly long string va…`)},
}

func TestJSONCondition(t *testing.T) {
	for i, tc := range jsonConditionTests {
		runTest(t, i, tc)
	}
}

var jsonSchemaTests = []TC{
	{jre, &JSON{Schema: `{"foo": 0, "bar": [0,"",0], "waz": false, pi: 0.0}`}, nil},
	{jre, &JSON{Schema: `{"bar": [0,"",0], "foo": 0, pi: 0.0, "waz": false}`}, nil},
	{jre, &JSON{Schema: `{"foo": 1.1}`}, nil}, //
	{jre, &JSON{Schema: `{"foo": false}`}, errors.New(`element foo: got int, want bool`)},
	{jre, &JSON{Schema: `{"foo": ""}`}, errors.New(`element foo: got int, want string`)},
	{jre, &JSON{Schema: `{"bar": []}`}, nil},
	{jre, &JSON{Schema: `{"bar": [0]}`}, nil},
	{jre, &JSON{Schema: `{"bar": [0, ""]}`}, nil},
	{jre, &JSON{Schema: `{"bar": [0, ""]}`}, nil},
	{jre, &JSON{Schema: `{"bar": [0, 0]}`}, errors.New(`element bar.1: got string, want int`)},
	{jre, &JSON{Schema: `{"bar": true}`}, errors.New(`element bar: got slice, want bool`)},
	{jre, &JSON{Schema: `{"bar": {}}`}, errors.New(`element bar: got slice, want object`)},
	{jre, &JSON{Schema: `{"ooops": 1.1}`}, errors.New(`element : missing child ooops`)},
	{jre, &JSON{Schema: `{"bar": [0, "", 0.0, false]}`},
		errors.New(`element bar: got only 3 array elements, need 4`)},

	{jre, &JSON{Schema: `{"bar": [null, null, 0]}`}, nil},
	{jre, &JSON{Schema: `{"nil": null}`}, nil},
	{jre, &JSON{Schema: `{"nil": 0}`}, errors.New(`element nil: got null, want int`)},

	{jrm, &JSON{Element: ".",
		Embedded: &JSON{Schema: `{"foo": 0, "bar": [0,0,0]}`}}, nil},
	{jrm, &JSON{Element: ".",
		Embedded: &JSON{Schema: `{"foo": 0, "bar": true}`}}, errCheck},
}

func TestJSONSchema(t *testing.T) {
	for i, tc := range jsonSchemaTests {
		runTest(t, i, tc)
	}
}

var findJSONelementTests = []struct {
	doc  string
	elem string
	want string
	err  string
}{
	// Primitive types
	{`123`, "", `123`, ""},
	{`123`, ".", `123`, ""},
	{`-123.456`, "", `-123.456`, ""},
	{`-123.456`, ".", `-123.456`, ""},
	{`"abc"`, "", `"abc"`, ""},
	{`"abc"`, ".", `"abc"`, ""},
	{`null`, "", `null`, ""},
	{`null`, ".", `null`, ""},
	{`true`, "", `true`, ""},
	{`true`, ".", `true`, ""},
	{`false`, "", `false`, ""},
	{`false`, ".", `false`, ""},
	{`123`, "X", `123`, "Element X not found"},

	// Whole (non-primitive) documents
	{`[3, 1 , 4, 1  ]`, "", `[3, 1 , 4, 1  ]`, ""},
	{`[3, 1 , 4, 1  ]`, ".", `[3, 1 , 4, 1  ]`, ""},
	{`{"A" : 123 , "B": "foo"} `, "", `{"A" : 123 , "B": "foo"} `, ""},
	{`{"A" : 123 , "B": "foo"} `, ".", `{"A" : 123 , "B": "foo"} `, ""},

	// Arrays
	{`[3, 1, 4, 1]`, "2", `4`, ""},
	{`[3, 1, "foo", 1]`, "2", `"foo"`, ""},
	{`[3, 1, 4, 1]`, "7", ``, "No index 7 in array  of len 4"},
	{`[3, 1, 4, 1]`, "-7", ``, "No index -7 in array  of len 4"},
	{`[3, 1, 4, 1]`, "foo", ``, "foo is not a valid index"},
	{`[3, 1, 4, 1]`, "2e0", ``, "2e0 is not a valid index"},
	{`{"A":{"B":[1,2,3]}}`, "A.B.5", ``, "No index 5 in array A.B of len 3"},

	// Objects
	{`{"A": 123, "B": "foo", "C": true, "D": null}`, "A", `123`, ""},
	{`{"A": 123, "B": "foo", "C": true, "D": null}`, "B", `"foo"`, ""},
	{`{"A": 123, "B": "foo", "C": true, "D": null}`, "C", `true`, ""},
	{`{"A": 123, "B": "foo", "C": true, "D": null}`, "D", `null`, ""},
	{`{"A": 123, "B": "foo", "C": true, "D": null}`, "E", ``, "Element E not found"},

	// Nested stuff
	{`{"A": [0, 1, {"B": true, "C": 2.72}, 3]}`, "A.2.C", `2.72`, ""},
	{`{"A": [0, 1, {"B": true, "C": 2.72}, 3]}`, ".A...2.C..", `2.72`, ""},
	{`{"a":{"b":{"c":{"d":77}}}}`, "a.b.c.d", `77`, ""},
	{`{"a":{"b":{"c":{"d":77}}}}`, "a.b.c", `{"d":77}`, ""},
	{`{"a":{"b":{"c":{"d":77}}}}`, "a.b", `{"c":{"d":77}}`, ""},
	{`{"a":{"b":{"c":{"d":77}}}}`, "a.b.c.d.X", ``, "Element a.b.c.d.X not found"},
	{`{"a":{"b":{"c":{"d":77}}}}`, "a.b.c.X", ``, "Element a.b.c.X not found"},
	{`{"a":{"b":{"c":{"d":77}}}}`, "a.b.X", ``, "Element a.b.X not found"},

	// Ill-formed JSON
	{`{"A":[{"B":flop}]}`, "", `{"A":[{"B":flop}]}`, ""},
	{`{"A":[{"B":flop}]}`, ".", `{"A":[{"B":flop}]}`, ""},
	{`{"A":[{"B":flop}]}`, "A.0.B", ``, "invalid character 'l' in literal false (expecting 'a')"},
	{`{"A":[{"B":3..1..4}]}`, "A.0.B", ``, "invalid character '.' after decimal point in numeric literal"},
}

func TestFindJSONElement(t *testing.T) {
	for i, tc := range findJSONelementTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			raw, err := findJSONelement([]byte(tc.doc), tc.elem, ".")
			if err != nil {
				if eg := err.Error(); eg != tc.err {
					t.Errorf("%d: %s from %s: got error %q, want %q",
						i, tc.elem, tc.doc, eg, tc.err)
				}
			} else {
				if tc.err != "" {
					t.Errorf("%d: %s from %s: got nil error, want %q",
						i, tc.elem, tc.doc, tc.err)
				} else {
					if got := string(raw); got != tc.want {
						t.Errorf("%d: %s from %s: got %q, want %q",
							i, tc.elem, tc.doc, got, tc.want)
					}
				}
			}
		})
	}

}

var ajeTests = []struct {
	J    string
	want string
}{
	{`"hello world`,
		`json syntax error in line 1, byte 12: ` +
			`unexpected end of JSON input`,
	},
	{`{
  "foo": 3.14,
  "bar:  123,
  "waz": -5
}`,
		`json syntax error in line 3, byte 14: ` +
			`invalid character '\n' in string literal`,
	},
	{`[
12,
3.4,

5.6.7
]`,
		`json syntax error in line 5, byte 4: ` +
			`invalid character '.' after array element`,
	},
}

func TestAugmentJSONError(t *testing.T) {
	for i, tc := range ajeTests {
		var v interface{}
		in := []byte(tc.J)
		err := json.Unmarshal(in, &v)
		if err == nil {
			t.Errorf("%d: no error?", i)
		} else {
			got := augmentJSONError(err, in).Error()
			if got != tc.want {
				t.Errorf("%d: wrong error\n got: %s\nwant: %s",
					i, got, tc.want)

			}
		}
	}
}
