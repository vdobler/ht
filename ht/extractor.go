// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"errors"
	"fmt"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

// Extractor allows to extract data from an executed Test.
// It supports extracting HTML attribute values and HTML text node values.
// Support for different stuff like HTTP Header, JSON values, etc. are
// a major TODO.
// Examples for CSRF token in the HTML:
//    <meta name="_csrf" content="18f0ca3f-a50a-437f-9bd1-15c0caa28413" />
//    <input type="hidden" name="_csrf" value="18f0ca3f-a50a-437f-9bd1-15c0caa28413"/>
type Extractor struct {
	// HTMLElementSelector is the CSS selector of an element, e.g.
	//     head meta[name="_csrf"]   or
	//     form#login input[name="tok"]
	//     div.token > span
	HTMLElementSelector string

	// HTMLElementAttribute is the name of the attribute from which the
	// value should be extracted.  The magic value "~text~" refers to the
	// text content of the element. E.g. in the examples above the following
	// should be sensible:
	//     content
	//     value
	//     ~text~
	HTMLElementAttribute string
}

func (e Extractor) Extract(t *Test) (string, error) {
	if e.HTMLElementSelector != "" {
		sel, err := cascadia.Compile(e.HTMLElementSelector)
		if err != nil {
			return "", err
		}
		doc, err := html.Parse(t.Response.Body())
		if err != nil {
			return "", err
		}

		node := sel.MatchFirst(doc)
		if node == nil {
			return "", fmt.Errorf("could not find node '%s'", e.HTMLElementSelector)
		}
		if e.HTMLElementAttribute == "~text~" {
			return textContent(node, true), nil
		}
		for _, a := range node.Attr {
			if a.Key == e.HTMLElementAttribute {
				return a.Val, nil
			}
		}
	}

	return "", errors.New("not found")
}

// Extract all values defined by VarEx from the sucessfully executed
// Test t.
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
