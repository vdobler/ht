// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// header.go provides generic checks of HTTP headers.
// Checks for cookie headers reside in cookie.go

package ht

import (
	"fmt"
	"net/http"
)

func init() {
	RegisterCheck(&Header{})
}

// Header provides a textual test of single-valued HTTP headers.
type Header struct {
	// Header is the HTTP header to check.
	Header string

	// Condition is applied to the first header value. A zero value checks
	// for the existence of the given Header only.
	Condition `json:",omitempty"`

	// Absent indicates that no header Header shall be part of the response.
	Absent bool `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (h Header) Execute(t *Test) error {
	key := http.CanonicalHeaderKey(h.Header)
	values := t.Response.Response.Header[key]
	if len(values) == 0 && h.Absent {
		return nil
	} else if len(values) == 0 && !h.Absent {
		return fmt.Errorf("header %s not received", h.Header)
	} else if len(values) > 0 && h.Absent {
		return fmt.Errorf("forbidden header %s received", h.Header)
	}
	return h.Fullfilled(values[0])
}

// Prepare implements Check's Prepare method.
func (h *Header) Prepare() error {
	return h.Condition.Compile()
}
