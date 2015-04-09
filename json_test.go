// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"testing"
)

var jr = Response{BodyBytes: []byte(`{"foo": 5, "bar": [1,2,3]}`)}
var ar = Response{BodyBytes: []byte(`["jo nesbo",["jo nesbo","jo nesbo harry hole","jo nesbo sohn","jo nesbo koma","jo nesbo hörbuch","jo nesbo headhunter","jo nesbo pupspulver","jo nesbo leopard","jo nesbo schneemann","jo nesbo the son"],[{"nodes":[{"name":"Bücher","alias":"stripbooks"},{"name":"Trade-In","alias":"tradein-aps"},{"name":"Kindle-Shop","alias":"digital-text"}]},{}],[]]`)}

var jsonTests = []TC{
	{jr, &JSON{Expression: "(.foo == 5) && ($len(.bar)==3) && (.bar[1]==2)"}, nil},
	{jr, &JSON{Expression: ".foo == 3"}, someError},
	{ar, &JSON{Expression: "$len(.) > 3"}, nil},
	{ar, &JSON{Expression: "$len(.) == 4"}, nil},
	{ar, &JSON{Expression: ".[0] == \"jo nesbo\""}, nil},
	{ar, &JSON{Expression: "$len(.[1]) == 10"}, nil},
	{ar, &JSON{Expression: ".[1][6] == \"jo nesbo pupspulver\""}, nil},
}

func TestJSON(t *testing.T) {
	for i, tc := range jsonTests {
		runTest(t, i, tc)
	}
}
