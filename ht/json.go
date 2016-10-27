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
	RegisterCheck(&JSONExpr{})
	RegisterCheck(&JSON{})
}

// ----------------------------------------------------------------------------
// JSONExpr

// JSONExpr allows checking JSON documents via gojee expressions.
// See github.com/nytlabs/gojee (or the vendored version) for details.
//
// Consider this JSON:
//     { "foo": 5, "bar": [ 1, 2, 3 ] }
// The follwing expression have these truth values:
//     .foo == 5                    true
//     $len(.bar) > 2               true as $len(.bar)==3
//     .bar[1] == 2                 true
//     (.foo == 9) || (.bar[0]<7)   true as .bar[0]==1
//     $max(.bar) == 3              true
//     $has(.bar, 7)                false as bar has no 7
type JSONExpr struct {
	// Expression is a boolean gojee expression which must evaluate
	// to true for the check to pass.
	Expression string `json:",omitempty"`

	tt *jee.TokenTree
}

// Prepare implements Check's Prepare method.
func (c *JSONExpr) Prepare() (err error) {
	if c.Expression == "" {
		return fmt.Errorf("Expression must not be empty")
	}

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

// Execute implements Check's Execute method.
func (c *JSONExpr) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	if c.tt == nil {
		if err := c.Prepare(); err != nil {
			return MalformedCheck{Err: err}
		}
	}

	var bmsg jee.BMsg
	err := json.Unmarshal([]byte(t.Response.BodyStr), &bmsg)
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

// ----------------------------------------------------------------------------
// JSON

// JSON allow to check a single string, number, boolean or null element in
// a JSON document against a Condition.
//
// Elements of the JSON document are selected by an element selector.
// In the JSON document
//     { "foo": 5, "bar": [ 1, "qux", 3 ], "waz": true, "nil": null }
// the follwing element selector are present and have the shown values:
//     foo       5
//     bar.0     1
//     bar.1     "qux"
//     bar.2     3
//     waz       true
//     nil       null
type JSON struct {
	// Element in the flattened JSON map to apply the Condition to.
	// E.g.  "foo.2" in "{foo: [4,5,6,7]}" would be 6.
	// An empty value result in just a check for 'wellformedness' of
	// the JSON.
	Element string

	// Condition to apply to the value selected by Element.
	// If Condition is the zero value then only the existence of
	// a JSON element selected by Element is checked.
	// Note that Condition is checked against the actual value in the
	// flattened JSON map which will contain the quotation marks for
	// string values.
	Condition

	// Embeded is a JSON check applied to the value selected by
	// Element. Useful when JSON contains embedded, quoted JSON as
	// a string and checking via Condition is not practical.
	// (It seems this nested JSON is common nowadays. I'm getting old.)
	Embedded *JSON

	// Sep is the separator in Element when checking the Condition.
	// A zero value is equivalent to "."
	Sep string `json:",omitempty"`
}

// Prepare implements Check's Prepare method.
func (c *JSON) Prepare() (err error) { return c.Compile() }

// Execute implements Check's Execute method.
func (c *JSON) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	sep := "."
	if c.Sep != "" {
		sep = c.Sep
	}

	out, err := gojsonexplode.Explodejson([]byte(t.Response.BodyStr), sep)
	if err != nil {
		return fmt.Errorf("unable to explode JSON: %s", err.Error())
	}

	var flat map[string]*json.RawMessage
	err = json.Unmarshal(out, &flat)
	if err != nil {
		return fmt.Errorf("unable to parse exploded JSON: %s", err.Error())
	}

	if c.Element == "" {
		return nil // JSON was welformed, no further checks.
	}

	val, ok := flat[c.Element]
	if !ok {
		return ErrNotFound
	}
	if val == nil {
		return c.Fulfilled("null")
	}

	return c.FulfilledBytes([]byte(*val))
}
