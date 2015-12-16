// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// validhtml.go contains checks to slighty validate a HTML document.

package ht

import (
	"fmt"

	"golang.org/x/net/html"
)

func init() {
	RegisterCheck(ValidHTML{})
}

// ValidHTML checks for valid HTML 5. Kinda: It never fails. TODO: make it useful.
type ValidHTML struct{}

// Execute implements Check's Execute method.
func (c ValidHTML) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}
	_, err := html.Parse(t.Response.Body())
	if err != nil {
		return fmt.Errorf("Invalid HTML: %s", err.Error())
	}

	return nil
}

// Prepare implements Check's Prepare method.
func (ValidHTML) Prepare() error { return nil }
