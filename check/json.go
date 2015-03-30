// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// json.go contains checks for a JSON body.

package check

import (
	"encoding/json"
	"fmt"

	"github.com/nytlabs/gojee"
	"github.com/vdobler/ht/response"
)

func init() {
	RegisterCheck(&JSON{})
}

// ----------------------------------------------------------------------------
// JSON

// JSON checking via github.com/nytlabs/gojee
type JSON struct {
	// Expression is a boolean gojee expression which must evaluate
	// to true for the check to pass.
	Expression string `xml:",attr"`

	tt *jee.TokenTree
}

func (c *JSON) Compile() (err error) {
	tokens, err := jee.Lexer(c.Expression)
	if err != nil {
		return err
	}
	c.tt, err = jee.Parser(tokens)
	if err != nil {
		return err
	}
	return nil
}

func (c *JSON) Okay(response *response.Response) error {
	if c.tt == nil {
		if err := c.Compile(); err != nil {
			return MalformedCheck{Err: err}
		}
	}

	var bmsg jee.BMsg
	err := json.Unmarshal(response.Body, &bmsg)
	if err != nil {
		return err
	}

	result, err := jee.Eval(c.tt, bmsg)
	if err != nil {
		return err
	}

	if b, ok := result.(bool); !ok {
		return MalformedCheck{Err: fmt.Errorf("Expected bool, got %T (%#v)", result, result)}
	} else if !b {
		return Failed
	}
	return nil
}
