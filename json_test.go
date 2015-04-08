// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"testing"
)

var jr = Response{BodyBytes: []byte(`{"foo": 5, "bar": [1,2,3]}`)}
var jsonTests = []TC{
	{jr, &JSON{Expression: "(.foo == 5) && ($len(.bar)==3) && (.bar[1]==2)"}, nil},
	{jr, &JSON{Expression: ".foo == 3"}, someError},
}

func TestJSON(t *testing.T) {
	for i, tc := range jsonTests {
		runTest(t, i, tc)
	}
}
