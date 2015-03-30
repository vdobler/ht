// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// body.go contains basic checks on the un-interpreted body of a HTTP response.

package check

import (
	"bytes"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/vdobler/ht/response"
)

func init() {
	RegisterCheck(UTF8Encoded{})
	RegisterCheck(BodyContains{})
	RegisterCheck(&BodyMatch{})
}

// ----------------------------------------------------------------------------
// UTF8Encoded

// UTF8Encoded checks that the response body is valid UTF-8 without BOMs.
type UTF8Encoded struct{}

func (c UTF8Encoded) Okay(response *response.Response) error {
	p := response.Body
	char := 0
	for len(p) > 0 {
		r, size := utf8.DecodeRune(p)
		if r == utf8.RuneError {
			return fmt.Errorf("Invalid UTF-8 at character %d in body.", char)
		}
		if r == '\ufeff' { // BOMs suck.
			return fmt.Errorf("Unicode BOM at character %d.", char)
		}
		p = p[size:]
		char++
	}
	return nil
}

// ----------------------------------------------------------------------------
// BodyContains

// BodyContains checks textual occurences in the response body.
type BodyContains struct {
	// Text is the literal text (no wildcards, no regexp) to look for in the body.
	Text string `xml:",attr"`

	// Count determines the number of occurences to check for:
	//     < 0: no occurence
	//    == 0: one ore more occurences
	//     > 0: exactly that many occurences
	Count int `json:",omitempty" xml:",attr,omitempty"`
}

func (c BodyContains) Okay(response *response.Response) error {
	body, err := response.Body, response.BodyErr
	text := []byte(c.Text)
	if err != nil {
		return BadBody
	}
	switch {
	case c.Count < 0:
		if pos := bytes.Index(body, text); pos != -1 {
			return FoundForbidden
		}
	case c.Count == 0:
		if pos := bytes.Index(body, text); pos == -1 {
			return NotFound
		}
	case c.Count > 0:
		if count := bytes.Count(body, text); count != c.Count {
			return WrongCount{Got: count, Want: c.Count}
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// BodyMatch

// BodyMatch checks the response body by matching a regular expression.
type BodyMatch struct {
	// Regexp is the regular expression to look for in the request body.
	Regexp string `xml:",attr"`

	// Count determines the number of occurences to check for:
	//     < 0: no occurence
	//    == 0: one ore more occurences
	//     > 0: exactly that many occurences
	Count int `xml:",attr,omitempty"`

	re *regexp.Regexp
}

func (c *BodyMatch) Okay(response *response.Response) error {
	if c.re == nil {
		err := c.Compile()
		if err != nil {
			return err
		}
	}
	if response.BodyErr != nil {
		return BadBody
	}

	m := c.re.FindAll(response.Body, -1)
	switch {
	case c.Count < 0 && m != nil:
		return FoundForbidden
	case c.Count == 0 && m == nil:
		return NotFound
	case c.Count > 0 && len(m) != c.Count:
		return WrongCount{Got: len(m), Want: c.Count}
	}
	return nil
}

func (c *BodyMatch) Compile() (err error) {
	c.re, err = regexp.Compile(c.Regexp)
	if err != nil {
		c.re = nil
		return MalformedCheck{Err: err}
	}
	return nil
}
