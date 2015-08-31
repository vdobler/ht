// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// json.go contains checks for a JSON body.

package ht

import (
	"encoding/json"
	"fmt"

	"github.com/nytlabs/gojee"
	"github.com/nytlabs/gojsonexplode"
)

func init() {
	RegisterCheck(&JSON{})
}

// ----------------------------------------------------------------------------
// JSON

// JSON checking via github.com/nytlabs/gojee (Expression) and
// github.com/nytlabs/gojsonexplode (Path&Condition). Both checks may
// be empty.
type JSON struct {
	// Expression is a boolean gojee expression which must evaluate
	// to true for the check to pass.
	Expression string `json:",omitempty"`

	// Sep is the seperator in Path when checking the Condition.
	// A zero value is equivalanet to "."
	Sep string `json:",omitempty"`

	// Path in the flattened JSON map to apply the Condition to.
	Path string `json:",omitempty"`

	// Condition to apply to the value selected by Path.
	// If Condition is the zero value then only the existens of
	// a JSON element selected by Path is checked.
	// Note that Condition s checked against the actual value in the
	// flattened JSON map which will contain the quotation marks for
	// string values.
	Condition `json:",omitempty"`

	tt *jee.TokenTree
}

// Prepare implements Check's Prepare method.
func (c *JSON) Prepare() (err error) {
	if c.Expression != "" {
		tokens, err := jee.Lexer(c.Expression)
		if err != nil {
			return err
		}
		c.tt, err = jee.Parser(tokens)
		if err != nil {
			return err
		}
	}

	if err := c.Compile(); err != nil {
		return err
	}

	return nil
}

// Execute implements Check's Execute method.
func (c *JSON) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	var exprErr, condErr error
	if c.Expression != "" {
		exprErr = c.executeExpression(t)
	}

	if c.Path != "" {
		condErr = c.executeCondition(t)
	}

	if exprErr != nil && condErr != nil {
		return fmt.Errorf("expression failed & path failed with %s", condErr.Error())
	}
	if exprErr != nil {
		return exprErr
	}
	if condErr != nil {
		return condErr
	}
	return nil
}

func (c *JSON) executeCondition(t *Test) error {
	sep := "."
	if c.Sep != "" {
		sep = c.Sep
	}

	out, err := gojsonexplode.Explodejson(t.Response.BodyBytes, sep)
	if err != nil {
		return fmt.Errorf("unable to explode JSON: %s", err.Error())
	}

	var flat map[string]*json.RawMessage
	err = json.Unmarshal(out, &flat)
	if err != nil {
		return fmt.Errorf("unable to parse exploded JSON: %s", err.Error())
	}

	val, ok := flat[c.Path]
	if !ok {
		return ErrNotFound
	}
	return c.FullfilledBytes([]byte(*val))
}

func (c *JSON) executeExpression(t *Test) error {
	if c.tt == nil {
		if err := c.Prepare(); err != nil {
			return MalformedCheck{Err: err}
		}
	}

	var bmsg jee.BMsg
	err := json.Unmarshal(t.Response.BodyBytes, &bmsg)
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
		return ErrFailed
	}
	return nil
}
