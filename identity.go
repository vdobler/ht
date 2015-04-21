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

// Identity checks the value of the response body by its sha1 hash
type Identity struct {
	SHA1 string // The expected SHA1 has as shown by sha1sum of the whole body."
}

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

func (_ Identity) Prepare() error { return nil }
