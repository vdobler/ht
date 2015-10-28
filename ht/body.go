// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// body.go contains basic checks on the un-interpreted body of a HTTP response.

package ht

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"
)

func init() {
	RegisterCheck(UTF8Encoded{})
	RegisterCheck(&Body{})
	RegisterCheck(&Sorted{})
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

// ----------------------------------------------------------------------------
// Sorted

// Sorted checks for an ordered occurence of items.
// The check Sorted could be replaced by a Regexp based Body test
// without loss of functionality; Sorted just makes the idea of
// "looking for a sorted occurence" clearer.
type Sorted struct {
	// Text is the list of text fragments to look for in the
	// response body.
	Text []string

	// AllowMissing makes the check not fail if values of Text
	// cannot be found in the response body.
	AllowMissing bool `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (s *Sorted) Execute(t *Test) error {
	bb := t.Response.BodyBytes
	found := 0
	for i, text := range s.Text {
		idx := bytes.Index(bb, []byte(text))
		if idx == -1 {
			if s.AllowMissing {
				continue
			}
			return fmt.Errorf("missing %d. value %q", i, text)
		}

		bb = bb[idx+len(text):]
		found++
	}
	if found < 2 {
		return fmt.Errorf("found only %d values", found)
	}
	return nil
}

// Prepare implements Check's Prepare method.
func (s *Sorted) Prepare() error {
	if len(s.Text) < 2 {
		return MalformedCheck{
			Err: errors.New("not enough values to check sorted"),
		}
	}
	return nil
}
