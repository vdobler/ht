// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// status.go provides checks on the status code of a HTTP response.

package check

import (
	"fmt"

	"github.com/vdobler/ht/response"
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

func (c StatusCode) Execute(response *response.Response) error {
	if response.Response.StatusCode != c.Expect {
		return fmt.Errorf("got %d, want %d", response.Response.StatusCode, c.Expect)
	}
	return nil
}

func (_ StatusCode) Prepare() error { return nil }
