// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
	"github.com/robertkrimen/otto"
	"github.com/vdobler/ht/populate"
	"golang.org/x/net/html"
)

// Extractor allows to extract information from an executed Test.
type Extractor interface {
	Extract(t *Test) (string, error)
}

// Extract all values defined by VarEx from the successfully executed Test t.
func (t *Test) Extract() map[string]string {
	data := make(map[string]string)
	t.ExValues = make(map[string]Extraction)
	for varname, ex := range t.VarEx {
		value, err := ex.Extract(t)
		if err != nil {
			t.ExValues[varname] = Extraction{Error: err}
			t.errorf("Problems extracting %q in %q: %s",
				varname, t.Name, err)
			continue
		}
		data[varname] = value
		t.ExValues[varname] = Extraction{Value: value}
	}
	return data
}

// ----------------------------------------------------------------------------
// Extractor Registry

// ExtractorRegistry keeps track of all known Extractors.
var ExtractorRegistry = make(map[string]reflect.Type)

// RegisterExtractor registers the extratcor type. Once an extractor is
// registered it may be unmarshaled from its name and marshaled data.
func RegisterExtractor(ex Extractor) {
	name := NameOf(ex)
	typ := reflect.TypeOf(ex)
	if _, ok := ExtractorRegistry[name]; ok {
		panic(fmt.Sprintf("Extractor with name %q already registered.", name))
	}
	ExtractorRegistry[name] = typ
}

func init() {
	RegisterExtractor(HTMLExtractor{})
	RegisterExtractor(BodyExtractor{})
	RegisterExtractor(JSONExtractor{})
	RegisterExtractor(CookieExtractor{})
	RegisterExtractor(JSExtractor{})
	RegisterExtractor(SetVariable{})
}

// ----------------------------------------------------------------------------
// ExtractorMap

// ExtractorMap is a map of Extractors with the sole purpose of
// attaching JSON (un)marshaling methods.
type ExtractorMap map[string]Extractor

// MarshalJSON produces a JSON array of the Extractors in em.
// Each Extractor is serialized in the form
//     { Extractor: "NameOfExtractorAsRegistered", Field1OfExtratcor: Value1, Field2: Value2, ... }
func (em ExtractorMap) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteRune('{')
	i := 0
	for name, ex := range em {
		raw, err := json.Marshal(ex)
		if err != nil {
			return nil, err
		}
		buf.WriteRune('"')
		buf.WriteString(name)
		buf.WriteRune('"')
		buf.WriteRune(':')
		buf.WriteString(`{"Extractor": "`)
		buf.WriteString(NameOf(ex))
		buf.WriteRune('"')
		if string(raw) != "{}" {
			buf.WriteString(", ")
			buf.Write(raw[1 : len(raw)-1])
		}
		buf.WriteRune('}')
		if i < len(em)-1 {
			buf.WriteString(", ")
		}
		i++
	}
	buf.WriteRune('}')

	return buf.Bytes(), nil
}

// Populate implements populate.Populator.Populate.
func (em *ExtractorMap) Populate(src interface{}) error {
	types := make(map[string]struct {
		Extractor string
	})

	err := populate.Lax(&types, src)
	if err != nil {
		return err
	}

	raw := make(map[string]interface{})
	srcMap, ok := src.(map[string]interface{})
	if !ok {
		return fmt.Errorf("ht: cannot populate %T to variable extractor object", src)
	}

	for name := range types {
		r, ok := srcMap[name].(map[string]interface{})
		if !ok {
			return fmt.Errorf("ht: cannot populate extractor for %q from %T", name, srcMap[name])
		}
		delete(r, "Extractor")
		raw[name] = r
	}

	exes := make(map[string]Extractor)
	for name, ex := range types {
		exName := ex.Extractor
		typ, ok := ExtractorRegistry[exName]
		if !ok {
			return fmt.Errorf("ht: no such extractor %s", exName)
		}
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		extractor := reflect.New(typ)
		err = populate.Strict(extractor.Interface(), raw[name])
		if err != nil {
			return fmt.Errorf("ht: cannot build extractor for %q: %s", name, err)
		}
		exes[name] = extractor.Interface().(Extractor)

	}
	*em = exes
	return nil
}

// ----------------------------------------------------------------------------
// HTMLExtractor

// HTMLExtractor allows to extract data from an executed Test.
// It supports extracting HTML attribute values and HTML text node values.
// Examples for CSRF token in the HTML:
//    <meta name="_csrf" content="18f0ca3f-a50a-437f-9bd1-15c0caa28413" />
//    <input type="hidden" name="_csrf" value="18f0ca3f-a50a-437f-9bd1-15c0caa28413"/>
type HTMLExtractor struct {
	// Selector is the CSS selector of an element, e.g.
	//     head meta[name="_csrf"]   or
	//     form#login input[name="tok"]
	//     div.token span
	Selector string

	// Attribute is the name of the attribute from which the
	// value should be extracted.  The magic value "~text~" refers to the
	// normalized text content of the element and ~rawtext~ to the raw
	// text content.
	// E.g. in the examples above the following should be sensible:
	//     content
	//     value
	//     ~text~
	Attribute string
}

// Extract implements Extractor's Extract method.
func (e HTMLExtractor) Extract(t *Test) (string, error) {
	if e.Selector != "" {
		sel, err := cascadia.Compile(e.Selector)
		if err != nil {
			return "", err
		}
		doc, err := html.Parse(t.Response.Body())
		if err != nil {
			return "", err
		}

		node := sel.MatchFirst(doc)
		if node == nil {
			return "", fmt.Errorf("could not find node '%s'", e.Selector)
		}
		if e.Attribute == "~rawtext~" {
			return TextContent(node, true), nil
		} else if e.Attribute == "~text~" {
			return TextContent(node, false), nil
		}

		for _, a := range node.Attr {
			if a.Key == e.Attribute {
				return a.Val, nil
			}
		}
	}

	return "", errors.New("not found")
}

// ----------------------------------------------------------------------------
// BodyExtractor

// BodyExtractor extracts a value from the uninterpreted response body
// via a regular expression.
type BodyExtractor struct {
	// Regexp is the regular expression to look for in the body.
	Regexp string

	// SubMatch selects which submatch (capturing group) of Regexp shall
	// be returned. A 0 value indicates the whole match.
	Submatch int `json:",omitempty"`
}

// Extract implements Extractor's Extract method.
func (e BodyExtractor) Extract(t *Test) (string, error) {
	if t.Response.BodyErr != nil {
		return "", ErrBadBody
	}

	re, err := regexp.Compile(e.Regexp)
	if err != nil {
		return "", err
	}

	if e.Submatch < 0 {
		return "", errors.New("BodyExtractor.Submatch < 0")
	}

	submatches := re.FindStringSubmatch(t.Response.BodyStr)
	if len(submatches) > e.Submatch {
		return string(submatches[e.Submatch]), nil
	}
	if len(submatches) == 0 {
		return "", fmt.Errorf("no match found in %q", t.Response.BodyStr)
	}
	return "", fmt.Errorf("got only %d submatches in %q", len(submatches)-1, submatches[0])
}

// ----------------------------------------------------------------------------
// JSONExtractor

// JSONExtractor extracts a value from a JSON response body.
//
// JSONExtractor works like the JSON check (i.e. elements are selected by their
// path) with two differences:
//   * null values are extracted as the empty string ""
//   * strings are stripped of their quotes
// Non-leaf elements can be extraced and will be returned verbatim. E.g. extarcting
// element Foo from
//     {"Foo": [ 1 , 2,3]  }
// will extract the follwoing string with verbatim spaces in the array:
//     "[ 1 , 2,3]"
//
type JSONExtractor struct {
	// Element path to extract.
	Element string `json:",omitempty"`

	// Sep is the separator in the element path.
	// A zero value is equivalent to "."
	Sep string `json:",omitempty"`
}

// Extract implements Extractor's Extract method.
func (e JSONExtractor) Extract(t *Test) (string, error) {
	if t.Response.BodyErr != nil {
		return "", ErrBadBody
	}

	sep := "."
	if e.Sep != "" {
		sep = e.Sep
	}

	raw, err := findJSONelement([]byte(t.Response.BodyStr), e.Element, sep)
	if err != nil {
		return "", err
	}
	s := string(raw)

	// Report null as empty string.
	if s == "null" {
		return "", nil
	}

	// Strip quotes from strings.
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) && len(s) >= 2 {
		s = s[1 : len(s)-1]
	}
	return s, nil
}

// ----------------------------------------------------------------------------
// CookieExtractor

// CookieExtractor extracts the value of a cookie received in a Set-Cookie
// header.  The value of the first cookie with the given name is extracted.
type CookieExtractor struct {
	Name string // Name is the name of the cookie.
}

// Extract implements Extractor's Extract method.
func (e CookieExtractor) Extract(t *Test) (string, error) {
	cookies := findCookiesByName(t, e.Name)
	if len(cookies) == 0 {
		return "", fmt.Errorf("cookie %s not received", e.Name)
	}

	return cookies[0].Value, nil
}

// ----------------------------------------------------------------------------
// JSExtractor

// JSExtractor extracts arbitrary stuff via custom JavaScript code.
//
// The current Test is present in the JavaScript VM via binding the name
// "Test" at top-level to the current Test being checked.
//
// The Script is evaluated and the final expression is the value
// extracted with the following exceptions:
//  - undefined or null is treated as an error
//  - Objects and Arrays are treated as errors. The error message is reported
//    in the field 'errmsg' of the object or the index 0 of the array.
//  - Strings, Numbers and Bools are treated as properly extracted values
//    which are returned.
//  - Other types result in undefined behaviour.
//
// The JavaScript code is interpreted by otto. See the documentation at
// https://godoc.org/github.com/robertkrimen/otto for details.
type JSExtractor struct {
	// Script is JavaScript code to be evaluated.
	//
	// The script may be read from disk with the following syntax:
	//     @file:/path/to/script
	Script string `json:",omitempty"`
}

// Extract implements Extractor's Extract method.
func (e JSExtractor) Extract(t *Test) (string, error) {
	// Reading the script with fileData is a hack as it accepts
	// the @vfile syntax but cannot do the variable replacements
	// as Prepare is called on the already 'replaced' checked.

	source, basename, err := fileData(e.Script, nil) // TODO: can use t.Variables here!
	if err != nil {
		return "", err
	}
	if basename == "" {
		basename = "<inline>"
	}
	vm := otto.New()
	script, err := vm.Compile(basename, source)
	if err != nil {
		return "", err
	}

	// Set Test object and execute script.
	currentTest, err := vm.ToValue(*t)
	if err != nil {
		return "", err
	}
	vm.Set("Test", currentTest)
	val, err := vm.Run(script)
	if err != nil {
		return "", err
	}

	// If the value resulting from Run is an Object it is considered
	// and error.
	if val.IsObject() {
		switch class := val.Class(); class {
		case "Array":
			first, err := val.Object().Get("0")
			if err != nil {
				return "", fmt.Errorf("extracted array without index 0")
			}
			return "", errors.New(first.String())
		case "Object":
			errmsg, err := val.Object().Get("errmsg")
			if err != nil {
				return "", fmt.Errorf("Ooops")
			}
			return "", errors.New(errmsg.String())
		default:
			return "", errors.New("extracted " + class)
		}
	}
	// Undefined, null (and NaN but this seems buggy) are errors too.
	if !val.IsDefined() || val.IsNull() {
		s, _ := val.ToString() // TODO: handle error?
		return "", fmt.Errorf("%s", s)
	}

	// Convert to string and return
	str, err := val.ToString()
	if err != nil {
		return "", err
	}
	return str, nil
}

// ----------------------------------------------------------------------------
// SetVariable

// SetVariable allows to pragmatically "extract" a fixed value.
type SetVariable struct {
	// To is the value to extract.
	To string
}

// Extract implements Extractor's Extract method.
func (e SetVariable) Extract(t *Test) (string, error) {
	return e.To, nil
}
