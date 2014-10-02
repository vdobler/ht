// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package check

import (
	_ "image/png"
	"strings"
	"testing"
)

func TestSubstituteVariables(t *testing.T) {
	r := strings.NewReplacer("a", "X", "e", "Y", "o", "Z")
	var ck Check
	ck = BodyContains{Text: "Hallo"}
	f := SubstituteVariables(ck, r)
	if bc, ok := f.(BodyContains); !ok {
		t.Errorf("Bad type %T", f)
	} else if bc.Text != "HXllZ" {
		t.Errorf("Got %s", bc.Text)
	}

	// TODO: a bit more testing here
}
