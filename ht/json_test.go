// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import "testing"

var jr = Response{BodyStr: `{"foo": 5, "bar": [1,2,3]}`}
var ar = Response{BodyStr: `["jo nesbo",["jo nesbo","jo nesbo harry hole","jo nesbo sohn","jo nesbo koma","jo nesbo hörbuch","jo nesbo headhunter","jo nesbo pupspulver","jo nesbo leopard","jo nesbo schneemann","jo nesbo the son"],[{"nodes":[{"name":"Bücher","alias":"stripbooks"},{"name":"Trade-In","alias":"tradein-aps"},{"name":"Kindle-Shop","alias":"digital-text"}]},{}],[]]`}
var jre = Response{BodyStr: `{"foo": 5, "bar": [1,"qux",3], "waz": true, "nil": null, "uuid": "ad09b43c-6538-11e6-8b77-86f30ca893d3"}`}
var jrx = Response{BodyStr: `{"foo": 5, "blub...`}

var jsonExpressionTests = []TC{
	{jr, &JSONExpr{Expression: "(.foo == 5) && ($len(.bar)==3) && (.bar[1]==2)"}, nil},
	{jr, &JSONExpr{Expression: "$max(.bar) == 3"}, nil},
	{jr, &JSONExpr{Expression: "$has(.bar, 2)"}, nil},
	{jr, &JSONExpr{Expression: "$has(.bar, 7)"}, someError},
	{jr, &JSONExpr{Expression: ".foo == 3"}, someError},
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
	{jr, &JSON{Element: "foo", Condition: Condition{Equals: "bar"}}, someError},
	{jr, &JSON{Element: "bar.3"}, ErrNotFound},
	{jr, &JSON{Element: "bar.3", Condition: Condition{Equals: "2"}}, ErrNotFound},
	{jr, &JSON{Element: "foo.wuz", Condition: Condition{Equals: "bar"}}, ErrNotFound},
	{jr, &JSON{Element: "qux", Condition: Condition{Equals: "bar"}}, ErrNotFound},

	{ar, &JSON{Element: "0", Condition: Condition{Equals: `"jo nesbo"`}}, nil},
	{ar, &JSON{Element: "1.4", Condition: Condition{Contains: `jo nesbo`}}, nil},
	{ar, &JSON{Element: "2.0.nodes.2.name", Condition: Condition{Equals: `"Kindle-Shop"`}}, nil},

	{jre, &JSON{Element: "bar.1", Condition: Condition{Equals: `"qux"`}}, nil},
	{jre, &JSON{Element: "waz", Condition: Condition{Equals: `true`}}, nil},
	{jre, &JSON{Element: "nil", Condition: Condition{Equals: `null`}}, nil},
	{jre, &JSON{Element: "nil", Condition: Condition{Prefix: `"`}}, someError},
	{jre, &JSON{Element: "uuid", Condition: Condition{
		Regexp: `^"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"$`}}, nil},

	{jre, &JSON{}, nil},
	{jrx, &JSON{}, someError},
}

func TestJSONCondition(t *testing.T) {
	for i, tc := range jsonConditionTests {
		runTest(t, i, tc)
	}
}
