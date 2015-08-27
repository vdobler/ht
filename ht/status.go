// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// status.go provides checks on the status code of a HTTP response.

package ht

import (
	"fmt"
)

func init() {
	RegisterCheck(StatusCode{})
}

// ----------------------------------------------------------------------------
// StatusCode

// StatusCode checks the HTTP statuscode.
type StatusCode struct {
	Expect int `xml:",attr"`
}

// Execute implements Check's Execute method.
func (c StatusCode) Execute(t *Test) error {
	if t.Response.Response.StatusCode != c.Expect {
		return fmt.Errorf("got %d, want %d", t.Response.Response.StatusCode, c.Expect)
	}
	return nil
}

// Prepare implements Check's Prepare method.
func (StatusCode) Prepare() error { return nil }
