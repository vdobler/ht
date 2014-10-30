// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"log"
	"os"
	"testing"

	"github.com/kr/pretty"
)

func TestLoadSuite(t *testing.T) {
	suite, err := LoadSuite("suite.ht", []string{"testdata", "."})
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	pretty.Printf("%# v\n", suite)

	suite.Log = log.New(os.Stdout, "", log.LstdFlags)
	err = suite.Compile()
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if testing.Short() {
		t.Skip("Skipping execution without network in short mode.")
	}

	result := suite.ExecuteTests()
	if result.Status != Pass {
		s := pretty.Sprintf("% #v", result)
		t.Fatalf("Unexpected problems result=\n%s\n", s)
	}

}
