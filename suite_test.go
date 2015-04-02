// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
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

	suite.Log = log.New(os.Stdout, "", log.LstdFlags)
	err = suite.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if testing.Short() {
		t.Skip("Skipping execution without network in short mode.")
	}

	result := suite.ExecuteTests()
	if result.Status != Pass {
		for _, tr := range result.TestResults {
			if tr.Status == Pass {
				continue
			}
			fmt.Println("Test", tr.Name)
			if tr.Error != nil {
				fmt.Println("  Error: ", tr.Error)
			} else {
				for _, cr := range tr.CheckResults {
					if cr.Status == Pass {
						continue
					}
					fmt.Println("  Fail: ", cr.Name, cr.JSON, cr.Status, cr.Error)
				}
			}
			if tr.Response != nil && tr.Response.Response != nil &&
				tr.Response.Response.Request != nil {
				tr.Response.Response.Request.TLS = nil
				req := pretty.Sprintf("% #v", tr.Response.Response.Request)
				fmt.Printf("  Request\n%s\n", req)
				tr.Response.Response.Request = nil
				tr.Response.Response.TLS = nil
				resp := pretty.Sprintf("% #v", tr.Response.Response)
				fmt.Printf("  Response\n%s\n", resp)
			}
		}
	}

	result.PrintReport(os.Stdout)
}
