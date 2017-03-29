// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import "testing"

var idr = Response{BodyStr: "Hello world"}

var identityTests = []TC{
	{idr, Identity{SHA1: "7b502c3a1f48c8609ae212cdfb639dee39673f5e"}, nil},
	{idr, Identity{SHA1: "99992c3a1f48c8609ae212cdfb639dee39673f5e"}, errCheck},
}

func TestIdentity(t *testing.T) {
	for i, tc := range identityTests {
		runTest(t, i, tc)
	}
}
