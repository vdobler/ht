// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// html.go contains checks on a HTML body.

package check

import (
	"fmt"

	"github.com/andybalholm/cascadia"
	"github.com/vdobler/ht/response"
	"golang.org/x/net/html"
)

func init() {
	RegisterCheck(&HTMLContains{})
	RegisterCheck(&HTMLContainsText{})
	RegisterCheck(ValidHTML{})
}

// ----------------------------------------------------------------------------
// ValidHTML

// ValidHTML checks for valid HTML 5. Kinda: It never fails. TODO: make it useful.
type ValidHTML struct{}

func (c ValidHTML) Okay(response *response.Response) error {
	if response.BodyErr != nil {
		return BadBody
	}
	_, err := html.Parse(response.BodyReader())
	if err != nil {
		return fmt.Errorf("Invalid HTML: %s", err.Error())
	}

	return nil
}

// ----------------------------------------------------------------------------
// HTMLContains

// HTMLContains checks for the existens of HTML elements selected by CSS selectors.
type HTMLContains struct {
	// Selector is the CSS selector of the HTML elements.
	Selector string `xml:",attr"`

	// Count determines the number of occurences to check for:
	//     < 0: no occurence
	//    == 0: one ore more occurences
	//     > 0: exactly that many occurences
	Count int `xml:",attr,omitempty"`

	sel cascadia.Selector
}

func (c *HTMLContains) Okay(response *response.Response) error {
	if c.sel == nil {
		if err := c.Compile(); err != nil {
			return err
		}
	}
	if response.BodyErr != nil {
		return BadBody
	}

	doc, err := html.Parse(response.BodyReader())
	if err != nil {
		return CantCheck{err}
	}

	matches := c.sel.MatchAll(doc)

	switch {
	case c.Count < 0 && len(matches) > 0:
		return FoundForbidden
	case c.Count == 0 && len(matches) == 0:
		return NotFound
	case c.Count > 0:
		if len(matches) != c.Count {
			return WrongCount{Got: len(matches), Want: c.Count}
		}
	}

	return nil
}

func (c *HTMLContains) Compile() (err error) {
	c.sel, err = cascadia.Compile(c.Selector)
	if err != nil {
		c.sel = nil
		return MalformedCheck{Err: err}
	}
	return nil
}

// ----------------------------------------------------------------------------
// HTMLContainsText

// HTMLContainsText check the text content off HTML elements selected by a CSS rule.
type HTMLContainsText struct {
	// Selector is the CSS selector of the HTML elements.
	Selector string `xml:",attr"`

	// The plain text content of each selected element.
	Text []string

	// If true: Text contains the all matches of Selector.
	Complete bool `xml:",attr,omitempty"`

	sel cascadia.Selector
}

func (c *HTMLContainsText) Okay(response *response.Response) error {
	if c.sel == nil {
		if err := c.Compile(); err != nil {
			return err
		}
	}
	if response.BodyErr != nil {
		return BadBody
	}
	doc, err := html.Parse(response.BodyReader())
	if err != nil {
		return CantCheck{err}
	}

	matches := c.sel.MatchAll(doc)

	for i, want := range c.Text {
		if i == len(matches) {
			return WrongCount{Got: len(matches), Want: len(c.Text)}
		}

		got := textContent(matches[i])
		if want != got {
			return fmt.Errorf("found %q, want %q", got, want)
		}
	}

	if c.Complete && len(c.Text) != len(matches) {
		return WrongCount{Got: len(matches), Want: len(c.Text)}

	}
	return nil
}

func (c *HTMLContainsText) Compile() (err error) {
	c.sel, err = cascadia.Compile(c.Selector)
	if err != nil {
		c.sel = nil
		return MalformedCheck{Err: err}
	}
	return nil
}

// textContent returns the full text content of n.
func textContent(n *html.Node) string {
	switch n.Type {
	case html.TextNode:
		return n.Data
	case html.ElementNode:
		s := ""
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			cs := textContent(child)
			if cs != "" {
				if s != "" && s[len(s)-1] != ' ' {
					s += " "
				}
				s += cs
			}
		}
		return s
	}
	return ""
}
