// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import "testing"

var jr = Response{BodyBytes: []byte(`{"foo": 5, "bar": [1,2,3]}`)}
var ar = Response{BodyBytes: []byte(`["jo nesbo",["jo nesbo","jo nesbo harry hole","jo nesbo sohn","jo nesbo koma","jo nesbo hörbuch","jo nesbo headhunter","jo nesbo pupspulver","jo nesbo leopard","jo nesbo schneemann","jo nesbo the son"],[{"nodes":[{"name":"Bücher","alias":"stripbooks"},{"name":"Trade-In","alias":"tradein-aps"},{"name":"Kindle-Shop","alias":"digital-text"}]},{}],[]]`)}

var jsonExpressionTests = []TC{
	{jr, &JSON{Expression: "(.foo == 5) && ($len(.bar)==3) && (.bar[1]==2)"}, nil},
	{jr, &JSON{Expression: ".foo == 3"}, someError},
	{ar, &JSON{Expression: "$len(.) > 3"}, nil},
	{ar, &JSON{Expression: "$len(.) == 4"}, nil},
	{ar, &JSON{Expression: ".[0] == \"jo nesbo\""}, nil},
	{ar, &JSON{Expression: "$len(.[1]) == 10"}, nil},
	{ar, &JSON{Expression: ".[1][6] == \"jo nesbo pupspulver\""}, nil},
}

func TestJSONExpression(t *testing.T) {
	for i, tc := range jsonExpressionTests {
		runTest(t, i, tc)
	}
}

var jsonConditionTests = []TC{
	{jr, &JSON{Path: "foo", Condition: Condition{Equals: "5"}}, nil},
	{jr, &JSON{Path: "bar.1", Condition: Condition{Equals: "2"}}, nil},
	{jr, &JSON{Path: "bar.2"}, nil},
	{jr, &JSON{Path: "bar#1", Sep: "#", Condition: Condition{Equals: "2"}}, nil},
	{jr, &JSON{Path: "foo", Condition: Condition{Equals: "bar"}}, someError},
	{jr, &JSON{Path: "bar.3"}, ErrNotFound},
	{jr, &JSON{Path: "bar.3", Condition: Condition{Equals: "2"}}, ErrNotFound},
	{jr, &JSON{Path: "foo.wuz", Condition: Condition{Equals: "bar"}}, ErrNotFound},
	{jr, &JSON{Path: "qux", Condition: Condition{Equals: "bar"}}, ErrNotFound},
	{ar, &JSON{Path: "0", Condition: Condition{Equals: `"jo nesbo"`}}, nil},
	{ar, &JSON{Path: "1.4", Condition: Condition{Contains: `jo nesbo`}}, nil},
	{ar, &JSON{Path: "2.0.nodes.2.name", Condition: Condition{Equals: `"Kindle-Shop"`}}, nil},
}

func TestJSONCondition(t *testing.T) {
	for i, tc := range jsonConditionTests {
		runTest(t, i, tc)
	}
}
