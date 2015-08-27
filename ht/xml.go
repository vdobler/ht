// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// xml.go contains XPath checks against XML documents.

package ht

import "launchpad.net/xmlpath"

func init() {
	RegisterCheck(&XML{})
}

// ----------------------------------------------------------------------------
// XML

// XML allows to check XML request bodies.
type XML struct {
	// Path is a XPath expression understood by launchpad.net/xmlpath.
	Path string

	// Condition the first element addressed by Path must fullfill.
	Condition

	path *xmlpath.Path
}

// Execute implements Check's Execute method.
func (x *XML) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return CantCheck{t.Response.BodyErr}
	}

	root, err := xmlpath.Parse(t.Response.Body())
	if err != nil {
		return err
	}

	if s, ok := x.path.String(root); !ok {
		return NotFound
	} else if e := x.Fullfilled(s); err != nil {
		return e
	}

	return nil
}

// Prepare implements Check's Prepare method.
func (x *XML) Prepare() error {
	p, err := xmlpath.Compile(x.Path)
	if err != nil {
		return err
	}

	x.path = p
	return nil
}
