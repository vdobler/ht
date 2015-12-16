// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// html.go contains checks on a HTML body.

package ht

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

func init() {
	RegisterCheck(&HTMLTag{})
	RegisterCheck(&HTMLContains{})
	RegisterCheck(W3CValidHTML{})
	RegisterCheck(&Links{})
}

// ----------------------------------------------------------------------------
// W3CValidHTML

// W3CValidHTML checks for valid HTML but checking the response body via
// the online checker from W3C which is very strict.
type W3CValidHTML struct {
	// AllowedErrors is the number of allowed errors (after ignoring errors).
	AllowedErrors int `json:",omitempty"`

	// IgnoredErrros is a list of error messages to be ignored completely.
	IgnoredErrors []Condition `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (w W3CValidHTML) Execute(t *Test) error {
	file := "@file:@sample.html:" + t.Response.BodyStr
	test := &Test{
		Name: "W3CValidHTML",
		Request: Request{
			Method: "POST",
			URL:    "http://validator.w3.org/nu/",
			Params: URLValues{
				"file": {file},
			},
			ParamsAs: "multipart",
			Header: http.Header{
				"Accept":     {"text/html,application/xhtml+xml"},
				"User-Agent": {"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2272.101 Safari/537.36"},
			},
		},
		Checks: CheckList{
			StatusCode{Expect: 200},
		},
		Timeout: Duration(20 * time.Second),
	}

	// TODO: properly limit gloabl rate at which we fire to W3C validator
	time.Sleep(100 * time.Millisecond)

	err := test.Run(nil)
	if err != nil {
		return CantCheck{err}
	}
	if test.Status != Pass {
		return CantCheck{test.Error}
	}

	// Interprete response from validator
	valStat := test.Response.Response.Header.Get("X-W3C-Validator-Status")
	if valStat == "Abort" {
		return CantCheck{fmt.Errorf("validator service sent Abort")}
	} else if valStat == "Valid" {
		return nil
	}

	doc, err := html.Parse(test.Response.Body())
	if err != nil {
		return CantCheck{err}
	}

	// Errors
	validationErrors := []ValidationIssue{}
	noIgnored := 0
outer:
	for _, ve := range w3cValidatorErrSel.MatchAll(doc) {
		vi := extractValidationIssue(ve)
		for _, c := range w.IgnoredErrors {
			if c.Fullfilled(vi.Message) == nil {
				noIgnored++
				continue outer
			}
		}
		validationErrors = append(validationErrors, vi)
	}
	// TODO: maybe warnings too?

	if len(validationErrors) > w.AllowedErrors {
		errmsg := fmt.Sprintf("Ignored %d errors", noIgnored)
		for i, e := range validationErrors {
			errmsg = fmt.Sprintf("%s\nError %d:\n  %s\n  %s\n  %s",
				errmsg, i, e.Position, e.Message, e.Input)
		}
		return fmt.Errorf("%s", errmsg)
	}

	return nil
}

// Prepare implements Check's Prepare method.
func (W3CValidHTML) Prepare() error { return nil }

// ValidationIssue contains extracted information from the output of
// a W3C validator run.
type ValidationIssue struct {
	Position string
	Message  string
	Input    string
}

var (
	w3cValidatorErrSel   = cascadia.MustCompile("li.error")
	w3cValidatorWarnSel  = cascadia.MustCompile("li.msg_warn")
	w3cValidatorLocSel   = cascadia.MustCompile("p.location")
	w3cValidatorMsgSel   = cascadia.MustCompile("p span")
	w3cValidatorInputSel = cascadia.MustCompile("p.extract")
)

func extractValidationIssue(node *html.Node) ValidationIssue {
	p := TextContent(w3cValidatorLocSel.MatchFirst(node), false)
	p = strings.Replace(p, "\n     ", "", -1)
	return ValidationIssue{
		Position: p,
		Message:  TextContent(w3cValidatorMsgSel.MatchFirst(node), false),
		Input:    TextContent(w3cValidatorInputSel.MatchFirst(node), false),
	}
}

// ----------------------------------------------------------------------------
// HTMLTag

// HTMLTag checks for the existens of HTML elements selected by CSS selectors.
type HTMLTag struct {
	// Selector is the CSS selector of the HTML elements.
	Selector string

	// Count determines the number of occurrences to check for:
	//     < 0: no occurrence
	//    == 0: one ore more occurrences
	//     > 0: exactly that many occurrences
	Count int `json:",omitempty"`

	sel cascadia.Selector
}

// Execute implements Check's Execute method.
func (c *HTMLTag) Execute(t *Test) error {
	if c.sel == nil {
		if err := c.Prepare(); err != nil {
			return err
		}
	}
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	doc, err := html.Parse(t.Response.Body())
	if err != nil {
		return CantCheck{err}
	}

	matches := c.sel.MatchAll(doc)

	switch {
	case c.Count < 0 && len(matches) > 0:
		return ErrFoundForbidden
	case c.Count == 0 && len(matches) == 0:
		return ErrNotFound
	case c.Count > 0:
		if len(matches) != c.Count {
			return WrongCount{Got: len(matches), Want: c.Count}
		}
	}

	return nil
}

// Prepare implements Check's Prepare method.
func (c *HTMLTag) Prepare() (err error) {
	c.sel, err = cascadia.Compile(c.Selector)
	if err != nil {
		c.sel = nil
		return MalformedCheck{Err: err}
	}
	return nil
}

// ----------------------------------------------------------------------------
// HTMLContains

// HTMLContains checks the text content (and optionally the order) of HTML
// elements selected by a CSS rule.
//
// The text content found in the HTML document is normalized by roughly the
// following procedure:
//   1.  Newlines are inserted around HTML block elements
//       (i.e. any non-inline element)
//   2.  Newlines and tabs are replaced by spaces.
//   3.  Multiple spaces are replaced by one space.
//   4.  Leading and trailing spaces are trimmed of.
// As an example consider the following HTML:
//   <html><body>
//     <ul class="fancy"><li>One</li><li>S<strong>econ</strong>d</li><li> Three </li></ul>
//   </body></html>
// The normalized text selected by a Selector of "ul.fancy" would be
//    "One Second Three"
type HTMLContains struct {
	// Selector is the CSS selector of the HTML elements.
	Selector string

	// Text contains the expected plain text content of the HTL elements
	// selected through the given selector.
	Text []string `json:",omitempty"`

	// Raw turns of white space normalization and will check the unprocessed
	// text content.
	Raw bool `json:",omitempty"`

	// Complete makes sure that no excess HTML elements are found:
	// If true the len(Text) must be equal to the number of HTML elements
	// selected for the check to succeed.
	Complete bool `json:",omitempty"`

	// InOrder makes the check fail if the selected HTML elements have a
	// different order than given in Text.
	InOrder bool `json:",omitempty"`

	sel cascadia.Selector
}

// Execute implements Check's Execute method.
func (c *HTMLContains) Execute(t *Test) error {
	if c.sel == nil {
		if err := c.Prepare(); err != nil {
			return err
		}
	}
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}
	doc, err := html.Parse(t.Response.Body())
	if err != nil {
		return CantCheck{err}
	}

	matches := c.sel.MatchAll(doc)
	actual := make([]string, len(matches))
	for i, m := range matches {
		actual[i] = TextContent(m, c.Raw)
	}

	last := 0
	for _, want := range c.Text {
		found := -1
		for a := last; a < len(actual); a++ {
			if want == actual[a] {
				found = a
				break
			}
		}
		if found < 0 {
			return fmt.Errorf("missing %q", want)
		}
		if c.InOrder {
			last = found + 1
		} else {
			last = 0
		}
	}

	if c.Complete && len(c.Text) != len(matches) {
		return WrongCount{Got: len(matches), Want: len(c.Text)}

	}
	return nil
}

// Prepare implements Check's Prepare method.
func (c *HTMLContains) Prepare() (err error) {
	c.sel, err = cascadia.Compile(c.Selector)
	if err != nil {
		c.sel = nil
		return MalformedCheck{Err: err}
	}
	return nil
}

// TextContent returns the full text content of n. With raw processing the
// unprocessed content is returned. If raw==false then whitespace is
// normalized.
func TextContent(n *html.Node, raw bool) string {
	tc := textContentRec(n, raw)
	if !raw {
		tc = normalizeWhitespace(tc)
	}
	return tc
}

// normalizeWhitespace replaces newlines and tabs with spaces, collapses
// multiple spaces to one and trims s on both ends from spaces.
func normalizeWhitespace(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Replace(strings.Replace(s, "\n", " ", -1), "\t", " ", -1)
	for strings.Index(s, "  ") != -1 {
		// TODO: speedup
		s = strings.Replace(s, "  ", " ", -1)
	}
	return s
}

// inlineElement contains the inline span HTML tags. Taken from
// https://developer.mozilla.org/de/docs/Web/HTML/Inline_elemente
var inlineElement = map[string]bool{
	"b":        true,
	"big":      true,
	"i":        true,
	"small":    true,
	"tt":       true,
	"abbr":     true,
	"acronym":  true,
	"cite":     true,
	"code":     true,
	"dfn":      true,
	"em":       true,
	"kbd":      true,
	"strong":   true,
	"samp":     true,
	"var":      true,
	"a":        true,
	"bdo":      true,
	"br":       true,
	"img":      true,
	"map":      true,
	"object":   true,
	"q":        true,
	"script":   true,
	"span":     true,
	"sub":      true,
	"sup":      true,
	"button":   true,
	"input":    true,
	"label":    true,
	"select":   true,
	"true":     true,
	"textarea": true,
}

// textContentRec serializes the text content of n. In raw mode the text
// content is completely unprocessed. If raw == false then content of block
// elements is surrounded by additional newlines.
func textContentRec(n *html.Node, raw bool) string {
	if n == nil {
		return ""
	}
	switch n.Type {
	case html.TextNode:
		return n.Data
	case html.ElementNode:
		s := ""
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			cs := textContentRec(child, raw)
			if !raw && child.Type == html.ElementNode &&
				!inlineElement[child.Data] {
				cs = "\n" + cs + "\n"
			}
			s += cs
		}
		return s
	}
	return ""
}

// ----------------------------------------------------------------------------
// Links

// Links checks links and references in HTML pages for availability.
type Links struct {
	// Head triggers HEAD requests instead of GET requests.
	Head bool

	// Which links to test; a combination of "a", "img", "link" and "script".
	// E.g. use "a img" to check the href of all a tags and src of all img tags.
	Which string

	// Concurrency determines how many of the found links are checked
	// concurrently. A zero value indicates sequential checking.
	Concurrency int `json:",omitempty"`

	// Timeout is the client timeout if different from main test.
	Timeout Duration `json:",omitempty"`

	// OnlyLinks and IgnoredLinks can be used to select only a subset of
	// all links.
	OnlyLinks, IgnoredLinks []Condition `json:",omitempty"`

	tags []string
}

// collectURLs returns the URLs selected by Which and OnlyLinks and IgnoredLinks
// as the map keys.
func (c *Links) collectURLs(t *Test) (map[string]struct{}, error) {
	if t.Response.BodyErr != nil {
		return nil, ErrBadBody
	}
	doc, err := html.Parse(t.Response.Body())
	if err != nil {
		return nil, CantCheck{err}
	}

	// Collect all non-ignored URL as map keys (for automatic deduplication).
	refs := make(map[string]struct{})
	for _, tag := range c.tags {
	outer:
		for _, u := range c.linkURL(doc, tag, t.Request.Request.URL) {
			for i, cond := range c.OnlyLinks {
				if cond.Fullfilled(u) == nil {
					break
				}
				if i == len(c.OnlyLinks)-1 {
					continue outer
				}
			}
			for _, cond := range c.IgnoredLinks {
				if cond.Fullfilled(u) == nil {
					continue outer
				}
			}
			refs[u] = struct{}{}
		}
	}

	return refs, nil
}

// Execute implements Check's Execute method.
func (c *Links) Execute(t *Test) error {
	refs, err := c.collectURLs(t)
	if err != nil {
		return err
	}
	suite := &Suite{}
	method := "GET"
	if c.Head {
		method = "HEAD"
	}
	timeout := t.Timeout
	if c.Timeout > 0 {
		timeout = c.Timeout
	}

	for r := range refs {
		test := &Test{
			Name: r,
			Request: Request{
				Method:          method,
				URL:             r,
				FollowRedirects: true,
			},
			Checks: CheckList{
				StatusCode{Expect: 200},
			},
			Verbosity: t.Verbosity - 1,
			Timeout:   timeout,
		}
		test.PopulateCookies(t.Jar, t.Request.Request.URL)
		suite.Tests = append(suite.Tests, test)
	}

	err = suite.Prepare()
	if err != nil {
		return CantCheck{fmt.Errorf("Constructed meta test are bad: %s", err)}
	}
	conc := 1
	if c.Concurrency > 1 {
		conc = c.Concurrency
	}
	suite.ExecuteConcurrent(conc)
	if suite.Status != Pass {
		broken := []string{}
		for _, test := range suite.Tests {
			if test.Status == Error || test.Status == Bogus {
				broken = append(broken, fmt.Sprintf("%s  -->  %s",
					test.Request.URL, test.Error))
			} else if test.Status == Fail {
				broken = append(broken, fmt.Sprintf("%s  -->  %d",
					test.Request.URL,
					test.Response.Response.StatusCode))
			}
		}
		return fmt.Errorf("%s", strings.Join(broken, "\n"))
	}
	return nil
}

// linkURL will extract all links for the given tag type from the parsed
// HTML document. All links are made absolute by parsing in the context of
// requestURL
func (Links) linkURL(document *html.Node, tag string, requestURL *url.URL) []string {
	href := linkURLattr[tag].sel
	matches := href.MatchAll(document)
	ak := linkURLattr[tag].attr
	refs := []string{}
	for _, m := range matches {
		for _, a := range m.Attr {
			if a.Key != ak || a.Val == "#" ||
				strings.HasPrefix(a.Val, "mailto:") {
				continue
			}
			u, err := requestURL.Parse(a.Val)
			if err != nil {
				refs = append(refs, "Error: "+err.Error())
			} else {
				u.Fragment = "" // easier to clear here
				refs = append(refs, u.String())
			}
		}
	}
	return refs
}

// linkURLattr maps tags to the appropriate link attributes and CSS selectors.
var linkURLattr = map[string]struct {
	attr string
	sel  cascadia.Selector
}{
	"a":      {"href", cascadia.MustCompile("a")},
	"img":    {"src", cascadia.MustCompile("img")},
	"script": {"src", cascadia.MustCompile("script")},
	"link":   {"href", cascadia.MustCompile("link")},
}

// Prepare implements Check's Prepare method.
func (c *Links) Prepare() (err error) {
	c.tags = nil
	for _, tag := range strings.Split(c.Which, " ") {
		tag = strings.TrimSpace(tag)
		switch tag {
		case "": // ignored
		case "a", "img", "link", "script":
			c.tags = append(c.tags, tag)
		default:
			fmt.Println("Bad", tag)
			return fmt.Errorf("Unknown link tag %q", tag)
		}
	}

	if len(c.tags) == 0 {
		return fmt.Errorf("Bad or missing value for Which: %q", c.Which)
	}
	return nil
}
