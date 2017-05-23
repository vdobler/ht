// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// json.go contains checks for a JSON body.

package ht

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/nytlabs/gojee"
	hjson "github.com/vdobler/ht/internal/hjson"
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
	return err
}

// Execute implements Check's Execute method.
func (c *JSONExpr) Execute(t *Test) error {
	if c.tt == nil {
		if err := c.Prepare(); err != nil {
			return MalformedCheck{err}
		}
	}
	if t.Response.BodyErr != nil {
		return ErrBadBody
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
		return MalformedCheck{fmt.Errorf("Expected bool, got %T (%#v)", result, result)}
	} else if !b {
		return ErrFailed
	}
	return nil
}

// ----------------------------------------------------------------------------
// JSON

// JSON allow to check an element in a JSON document against a Condition
// and to validate the structur of the document against a schema.
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
//
// A schema is an example JSON document with the same structure where each
// leave element just determines the expected type. The JSON document from
// above would conform to the schema:
//     {
//       "foo": 0, "bar": [0,"",1], "waz": false,
//       "maa": { "muh": 0.0, "mee": 0 },
//     }
// Contrary to standard JSON this check allows to distinguish floats from
// ints with the rule that an integer is a valid value for a float in a schema.
// So any string in a schema forces a string value, any int in a schema forces
// an integer value, any float in a schema forces either an int or a float.
// Null values in schemas act as wildcards: any value (int, bool, float, string
// or null) is valid. This is useful if you want to skip validation of e.g.
// the first two array elements.
//
// It is typically not useful to combine schema validation with checking
// a condition.
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

	// Schema is the expected structure of the selected element.
	Schema string

	// Embedded is a JSON check applied to the value selected by
	// Element. Useful when JSON contains embedded, quoted JSON as
	// a string and checking via Condition is not practical.
	// (It seems this nested JSON is common nowadays. I'm getting old.)
	Embedded *JSON `json:",omitempty"`

	// Sep is the separator in Element when checking the Condition.
	// A zero value is equivalent to "."
	Sep string `json:",omitempty"`

	schema interface{}
}

// Prepare implements Check's Prepare method.
func (c *JSON) Prepare() error {
	err := c.Compile()
	if err != nil {
		return err
	}
	if c.Schema != "" {
		err = hjson.Unmarshal([]byte(c.Schema), &c.schema)
		if err != nil {
			return err
		}
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

	if c.schema != nil {
		var actual interface{}
		err = hjson.Unmarshal(raw, &actual)
		if err != nil {
			return err // That should not happen here as JSON-validity
			// has been checked above.

		}
		av, sv := reflect.ValueOf(actual), reflect.ValueOf(c.schema)
		err = compareStructure(c.Element, sep, av, sv)
		if err != nil {
			return err
		}
	}

	return c.Fulfilled(string(raw))
}

func kind(v reflect.Value) string {
	s := v.Kind().String()
	if strings.HasSuffix(s, "64") {
		s = s[:len(s)-2]
	}
	return s
}

// compareStructure returns the/one/all (?) deviations in actual from
// the desired structure.
func compareStructure(element, sep string, actual, schema reflect.Value) error {
	for actual.Kind() == reflect.Interface {
		actual = actual.Elem()
	}
	for schema.Kind() == reflect.Interface {
		schema = schema.Elem()
	}
	if !schema.IsValid() {
		// null value in schema accepts everything.
		return nil
	}
	if !actual.IsValid() {
		// null value in actual does not match anything.
		return fmt.Errorf("element %s: got null, want %s",
			element, kind(schema))
	}

	switch schema.Kind() {
	case reflect.Float64:
		if actual.Kind() != reflect.Float64 && actual.Kind() != reflect.Int64 {
			return fmt.Errorf("element %s: got %s, want float",
				element, kind(actual))
		}
	case reflect.Bool, reflect.String, reflect.Int64:
		if actual.Kind() != schema.Kind() {
			return fmt.Errorf("element %s: got %s, want %s",
				element, kind(actual), kind(schema))
		}
	case reflect.Slice:
		if err := compareSlice(element, sep, actual, schema); err != nil {
			return err
		}
	case reflect.Map:
		if err := compareMap(element, sep, actual, schema); err != nil {
			return err
		}
	default:
		return fmt.Errorf("ht: ooops: type of %s (%s) should not happen in unmarshaled JSON",
			schema.Kind(), schema)
	}
	return nil
}

func compareSlice(element, sep string, actual, schema reflect.Value) error {
	if actual.Kind() != reflect.Slice {
		return fmt.Errorf("element %s: got %s, want array",
			element, kind(actual))
	}
	an, sn := actual.Len(), schema.Len()
	if an < sn {
		return fmt.Errorf("element %s: got only %d array elements, need %d",
			element, an, sn)
	}
	for i := 0; i < sn; i++ {
		elmt := fmt.Sprintf("%d", i)
		if element != "" {
			elmt = element + sep + elmt
		}
		err := compareStructure(elmt, sep, actual.Index(i), schema.Index(i))
		if err != nil {
			return err
		}

	}
	return nil
}

func compareMap(element, sep string, actual, schema reflect.Value) error {
	if actual.Kind() != reflect.Map {
		return fmt.Errorf("element %s: got %s, want object",
			element, kind(actual))
	}
	// BUG: handle only string keys
	for _, key := range schema.MapKeys() {
		acv := actual.MapIndex(key)
		if !acv.IsValid() {
			return fmt.Errorf("element %s: missing child %s",
				element, key.String())
		}
		elmt := key.String()
		if element != "" {
			elmt = element + sep + elmt
		}
		err := compareStructure(elmt, sep, acv, schema.MapIndex(key))
		if err != nil {
			return err
		}
	}
	return nil
}
