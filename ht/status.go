// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// status.go provides checks on the status code of a HTTP response.

package ht

import "fmt"

func init() {
	RegisterCheck(StatusCode{})
	RegisterCheck(NoServerError{})
}

// ----------------------------------------------------------------------------
// StatusCode

// StatusCode checks the HTTP statuscode.
type StatusCode struct {
	// Expect is the value to expect, e.g. 302.
	//
	// If Expect <= 9 it matches a whole range of status codes, e.g.
	// with Expect==4 any of the 4xx status codes would fulfill this check.
	Expect int
}

// Execute implements Check's Execute method.
func (c StatusCode) Execute(t *Test) error {
	if c.Expect < 10 {
		if t.Response.Response.StatusCode/100 != c.Expect {
			return fmt.Errorf("got %d, want %dxx",
				t.Response.Response.StatusCode, c.Expect)
		}
		return nil
	}

	if t.Response.Response.StatusCode != c.Expect {
		return fmt.Errorf("got %d, want %d",
			t.Response.Response.StatusCode, c.Expect)
	}
	return nil
}

// Prepare implements Check's Prepare method.
func (StatusCode) Prepare() error { return nil }

// ----------------------------------------------------------------------------
// NoServerError

// NoServerError checks the HTTP status code for not being a 5xx server error
// and that the body could be read without errors or timeouts.
type NoServerError struct{}

// Execute implements Check's Execute method.
func (NoServerError) Execute(t *Test) error {
	if t.Response.Response == nil {
		return fmt.Errorf("No response received")
	}
	if t.Response.Response.StatusCode/100 == 5 {
		return fmt.Errorf("Server Error %q", t.Response.Response.Status)
	}
	if t.Response.BodyErr != nil {
		return t.Response.BodyErr
	}
	return nil
}

// Prepare implements Check's Prepare method.
func (NoServerError) Prepare() error { return nil }
