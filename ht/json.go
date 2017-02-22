// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// json.go contains checks for a JSON body.

package ht

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/nytlabs/gojee"
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

// JSON allow to check an element in a JSON document against a Condition.
//
// The element of the JSON document is selected by its "path". Example:
// In the JSON document
//     {
//       "foo": 5,
//       "bar": [ 1, "qux" ,3 ],
//       "waz": true,
//       "maa": { "muh": 3.141, "mee": 0 },
//       "nil": null
//     }
// the following table shows several element paths and their value:
//     foo       5
//     bar       [ 1, "qux" ,3 ]
//     bar.0     1
//     bar.1     "qux"
//     bar.2     3
//     waz       true
//     maa       { "muh": 3.141, "mee": 0 }
//     maa.muh   3.141
//     maa.mee   0
//     nil       null
// Note that the value for "bar" is the raw string and contains the original
// white space characters as present in the original JSON document.
type JSON struct {
	// Element in the flattened JSON map to apply the Condition to.
	// E.g.  "foo.2" in "{foo: [4,5,6,7]}" would be 6.
	// The whole JSON can be selected by Sep, typically ".".
	// An empty value result in just a check for 'wellformedness' of
	// the JSON.
	Element string

	// Condition to apply to the value selected by Element.
	// If Condition is the zero value then only the existence of
	// a JSON element selected by Element is checked.
	// Note that Condition is checked against the actual raw value of
	// the JSON document and will contain quotation marks for strings.
	Condition

	// Embedded is a JSON check applied to the value selected by
	// Element. Useful when JSON contains embedded, quoted JSON as
	// a string and checking via Condition is not practical.
	// (It seems this nested JSON is common nowadays. I'm getting old.)
	Embedded *JSON `json:",omitempty"`

	// Sep is the separator in Element when checking the Condition.
	// A zero value is equivalent to "."
	Sep string `json:",omitempty"`
}

// Prepare implements Check's Prepare method.
func (c *JSON) Prepare() error {
	err := c.Compile()
	if err != nil {
		return err
	}
	if c.Embedded != nil {
		return c.Embedded.Prepare()
	}
	return nil
}

func findJSONelement(data []byte, element, sep string) ([]byte, error) {
	path := strings.Split(element, sep)
	for e, elem := range path {
		if elem == "" {
			continue
		}
		data = bytes.TrimSpace(data)
		if len(data) == 0 {
			return nil, nil
		}
		switch data[0] {
		case '[':
			v := []json.RawMessage{}
			err := json.Unmarshal(data, &v)
			if err != nil {
				return nil, err
			}
			i, err := strconv.Atoi(elem)
			if err != nil {
				return nil, fmt.Errorf("%s is not a valid index", elem)
			}
			if i < 0 || i >= len(v) {
				return nil, fmt.Errorf("no index %d in array %s of len %d",
					i, strings.Join(path[:e], sep), len(v))
			}
			data = []byte(v[i])
		case '{':
			v := map[string]json.RawMessage{}
			err := json.Unmarshal(data, &v)
			if err != nil {
				return nil, err
			}
			raw, ok := v[elem]
			if !ok {
				return nil, fmt.Errorf("element %s not found",
					strings.Join(path[:e+1], sep))
			}
			data = []byte(raw)
		default:
			return nil, fmt.Errorf("element %s not found",
				strings.Join(path[:e+1], sep))
		}
	}
	return data, nil
}

// Execute implements Check's Execute method.
func (c *JSON) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	sep := "." // The default value for Sep.
	if c.Sep != "" {
		sep = c.Sep
	}

	raw, err := findJSONelement([]byte(t.Response.BodyStr), c.Element, sep)
	if err != nil {
		return err
	}

	// Check for wellformed JSON.
	var v interface{}
	err = json.Unmarshal(raw, &v)
	if err != nil {
		return err
	}

	if c.Embedded != nil {
		unquoted, err := strconv.Unquote(string(raw))
		if err != nil {
			return fmt.Errorf("element %s: %s", c.Element, err)
		}
		etest := &Test{Response: Response{BodyStr: unquoted}}
		eerr := c.Embedded.Execute(etest)
		if eerr != nil {
			return fmt.Errorf("embedded: %s", eerr)
		}
	}

	return c.Fulfilled(string(raw))
}
