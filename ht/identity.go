// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// identity.go provides has based identiy tecking of response bodies

package ht

import (
	"crypto/sha1"
	"fmt"
)

func init() {
	RegisterCheck(Identity{})
}

// ----------------------------------------------------------------------------
// Identity

// Identity checks the value of the response body by comparing its SHA1 hash
// to the expacted has value.
type Identity struct {
	// SHA1 is the expected hash as shown by sha1sum of the whole body.
	// E.g. 2ef7bde608ce5404e97d5f042f95f89f1c232871 for a "Hello World!"
	// body (no newline).
	SHA1 string
}

// Execute implements Check's Execute method.
func (i Identity) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return CantCheck{t.Response.BodyErr}
	}
	hash := sha1.Sum(t.Response.BodyBytes)
	s := fmt.Sprintf("%02x", hash)
	if s == i.SHA1 {
		return nil
	}
	return fmt.Errorf("Got %s", s)
}

// Prepare implements Check's Prepare method.
func (_ Identity) Prepare() error { return nil }
