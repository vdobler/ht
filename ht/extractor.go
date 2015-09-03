// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/andybalholm/cascadia"
	"github.com/vdobler/ht/internal/json5"
	"golang.org/x/net/html"
)

// Extractor allows to extract information from an executed Test.
type Extractor interface {
	Extract(t *Test) (string, error)
}

// Extract all values defined by VarEx from the sucessfully executed Test t.
func (t *Test) Extract() map[string]string {
	data := make(map[string]string)
	for varname, ex := range t.VarEx {
		value, err := ex.Extract(t)
		if err != nil {
			t.errorf("Problems extracting %q in %q: %s",
				varname, t.Name, err)
			continue
		}
		data[varname] = value
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
		raw, err := json5.Marshal(ex)
		if err != nil {
			return nil, err
		}
		buf.WriteString(name)
		buf.WriteRune(':')
		buf.WriteString(`{Extractor: "`)
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

// UnmarshalJSON unmarshals data to a map of Extractors.
func (em *ExtractorMap) UnmarshalJSON(data []byte) error {
	if *em == nil {
		*em = make(ExtractorMap)
	}
	raw := map[string]json5.RawMessage{}
	err := json5.Unmarshal(data, &raw)
	if err != nil {
		return err
	}
	for name, ex := range raw {
		u := struct{ Extractor string }{}
		err = json5.Unmarshal(ex, &u)
		if err != nil {
			return err
		}
		typ, ok := ExtractorRegistry[u.Extractor]
		if !ok {
			return fmt.Errorf("ht: no such extractor %s", u.Extractor)
		}
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		extractor := reflect.New(typ)
		err = json5.Unmarshal(ex, extractor.Interface())
		if err != nil {
			return err
		}
		(*em)[name] = extractor.Interface().(Extractor)
	}
	return nil
}

// ----------------------------------------------------------------------------
// HTMLExtractor

// HTMLExtractor allows to extract data from an executed Test.
// It supports extracting HTML attribute values and HTML text node values.
// Support for different stuff like HTTP Header, JSON values, etc. are
// a major TODO.
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
	// text content of the element. E.g. in the examples above the following
	// should be sensible:
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
		if e.Attribute == "~text~" {
			return textContent(node, true), nil
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

// BodyExtractor extracts a value from the uninterpreted response body.
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

	submatches := re.FindSubmatch(t.Response.BodyBytes)
	if len(submatches) > e.Submatch {
		return string(submatches[e.Submatch]), nil
	}

	return "", fmt.Errorf("got only %d submatches in %q", len(submatches)-1, submatches[0])
}
