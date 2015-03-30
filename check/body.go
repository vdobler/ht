// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// body.go contains basic checks on the un-interpreted body of a HTTP response.

package check

import (
	"fmt"
	"unicode/utf8"

	"github.com/vdobler/ht/response"
)

func init() {
	RegisterCheck(UTF8Encoded{})
	RegisterCheck(&Body{})
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
// Body

type Body Condition

func (c Body) Okay(response *response.Response) error {
	body, err := response.Body, response.BodyErr
	if err != nil {
		return BadBody
	}
	return Condition(c).FullfilledBytes(body)
}

func (c *Body) Compile() (err error) {
	return ((*Condition)(c)).Compile()
}
