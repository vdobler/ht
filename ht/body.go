// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// body.go contains basic checks on the un-interpreted body of a HTTP response.

package ht

import (
	"fmt"
	"unicode/utf8"
)

func init() {
	RegisterCheck(UTF8Encoded{})
	RegisterCheck(&Body{})
}

// ----------------------------------------------------------------------------
// UTF8Encoded

// UTF8Encoded checks that the response body is valid UTF-8 without BOMs.
type UTF8Encoded struct{}

// Execute implements Check's Execute method.
func (c UTF8Encoded) Execute(t *Test) error {
	p := t.Response.BodyBytes
	char := 0
	for len(p) > 0 {
		r, size := utf8.DecodeRune(p)
		char++
		if r == utf8.RuneError {
			return fmt.Errorf("Invalid UTF-8 after character %d.", char)
		}
		if r == '\ufeff' { // BOMs suck.
			return fmt.Errorf("Unicode BOM at character %d.", char)
		}
		p = p[size:]
	}
	return nil
}

// Prepare implements Check's Prepare method.
func (UTF8Encoded) Prepare() error { return nil }

// ----------------------------------------------------------------------------
// Body

// Body provides simple condition checks on the response body.
type Body Condition

// Execute implements Check's Execute method.
func (b Body) Execute(t *Test) error {
	body, err := t.Response.BodyBytes, t.Response.BodyErr
	if err != nil {
		return ErrBadBody
	}
	return Condition(b).FullfilledBytes(body)
}

// Prepare implements Check's Prepare method.
func (b *Body) Prepare() error {
	return ((*Condition)(b)).Compile()
}
