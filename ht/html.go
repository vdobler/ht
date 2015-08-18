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
	RegisterCheck(ValidHTML{})
	RegisterCheck(W3CValidHTML{})
	RegisterCheck(&Links{})
}

// ----------------------------------------------------------------------------
// ValidHTML and W3CValidHTML

// ValidHTML checks for valid HTML 5. Kinda: It never fails. TODO: make it useful.
type ValidHTML struct{}

func (c ValidHTML) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return BadBody
	}
	_, err := html.Parse(t.Response.Body())
	if err != nil {
		return fmt.Errorf("Invalid HTML: %s", err.Error())
	}

	return nil
}

func (_ ValidHTML) Prepare() error { return nil }

// W3CValidHTML checks for valid HTML but checking the response body via
// the online checker from W3C which is very strict.
type W3CValidHTML struct {
	// AllowedErrors is the number of allowed errors (after ignoring errors).
	AllowedErrors int `json:",omitempty"`

	// IgnoredErrros is a list of error messages to be ignored completely.
	IgnoredErrors []Condition `json:",omitempty"`
}

func (w W3CValidHTML) Execute(t *Test) error {
	file := "@file:@sample.html:" + string(t.Response.BodyBytes)
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

func (_ W3CValidHTML) Prepare() error { return nil }

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
	p := textContent(w3cValidatorLocSel.MatchFirst(node), false)
	p = strings.Replace(p, "\n     ", "", -1)
	return ValidationIssue{
		Position: p,
		Message:  textContent(w3cValidatorMsgSel.MatchFirst(node), false),
		Input:    textContent(w3cValidatorInputSel.MatchFirst(node), false),
	}
}

// ----------------------------------------------------------------------------
// HTMLTag

// HTMLTag checks for the existens of HTML elements selected by CSS selectors.
type HTMLTag struct {
	// Selector is the CSS selector of the HTML elements.
	Selector string

	// Count determines the number of occurences to check for:
	//     < 0: no occurence
	//    == 0: one ore more occurences
	//     > 0: exactly that many occurences
	Count int `json:",omitempty"`

	sel cascadia.Selector
}

func (c *HTMLTag) Execute(t *Test) error {
	if c.sel == nil {
		if err := c.Prepare(); err != nil {
			return err
		}
	}
	if t.Response.BodyErr != nil {
		return BadBody
	}

	doc, err := html.Parse(t.Response.Body())
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

// HTMLContains check the text content off HTML elements selected by
// a CSS rule.
//
// The text content found in the HTML document is normalized by roughly the
// following procedure:
//   1.  Newlines are inserted around HTML block elements
//       (actuall any non-inline element)
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

	// The plain text content of each selected element.
	Text []string `json:",omitempty"`

	// Raw turns of white space normalization and returns the unprocessed
	// text content.
	Raw bool `json:",omitempty"`

	// If true: Text contains all matches of Selector.
	Complete bool `json:",omitempty"`

	sel cascadia.Selector
}

func (c *HTMLContains) Execute(t *Test) error {
	if c.sel == nil {
		if err := c.Prepare(); err != nil {
			return err
		}
	}
	if t.Response.BodyErr != nil {
		return BadBody
	}
	doc, err := html.Parse(t.Response.Body())
	if err != nil {
		return CantCheck{err}
	}

	matches := c.sel.MatchAll(doc)

	for i, want := range c.Text {
		if i == len(matches) {
			return WrongCount{Got: len(matches), Want: len(c.Text)}
		}

		got := textContent(matches[i], c.Raw)
		got = normalizeWhitespace(got)
		if want != got {
			return fmt.Errorf("found %q, want %q", got, want)
		}
	}

	if c.Complete && len(c.Text) != len(matches) {
		return WrongCount{Got: len(matches), Want: len(c.Text)}

	}
	return nil
}

func (c *HTMLContains) Prepare() (err error) {
	c.sel, err = cascadia.Compile(c.Selector)
	if err != nil {
		c.sel = nil
		return MalformedCheck{Err: err}
	}
	return nil
}

// textContent returns the full text content of n. With raw processing the
// unprocessed content is returned. If raw==false then whitespace is
// normalized.
func textContent(n *html.Node, raw bool) string {
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

// Links checks links and references in HTML pages for availability
type Links struct {
	// Head triggers HEAD request instead of GET requests.
	Head bool

	// Which links to test; a combination of "a", "img", "link" and "script".
	// E.g. use "a img" to check the href of all a-tags and src of all img-tags.
	Which string

	// Concurrency determines how many of the found links are checked
	// concurrently. A zero value indicats sequential checking.
	Concurrency int `json:",omitempty"`

	// Timeout if different from main test.
	Timeout Duration `json:",omitempty"`

	IgnoredLinks []Condition `json:",omitempty"`

	tags []string
}

func (c *Links) collectURLs(t *Test) (map[string]struct{}, error) {
	if t.Response.BodyErr != nil {
		return nil, BadBody
	}
	doc, err := html.Parse(t.Response.Body())
	if err != nil {
		return nil, CantCheck{err}
	}

	// Collect all non-ignored URL as map keys (for automatic deduplication).
	refs := make(map[string]struct{})
outer:
	for _, tag := range c.tags {
		for _, u := range c.linkURL(doc, tag, t.Request.Request.URL) {
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

	for r, _ := range refs {
		test := &Test{
			Name: r,
			Request: Request{
				Method:          method,
				URL:             r,
				FollowRedirects: true,
			},
			ClientPool: t.ClientPool,
			Checks: CheckList{
				StatusCode{Expect: 200},
			},
			Verbosity: t.Verbosity - 1,
			Timeout:   timeout,
		}
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

func (_ Links) linkURL(doc *html.Node, tag string, reqURL *url.URL) []string {
	attr := map[string]string{
		"a":      "href",
		"img":    "src",
		"script": "src",
		"link":   "href",
	}
	href := cascadia.MustCompile(tag)
	matches := href.MatchAll(doc)
	ak := attr[tag]
	refs := []string{}
	for _, m := range matches {
		for _, a := range m.Attr {
			if a.Key != ak || a.Val == "#" ||
				strings.HasPrefix(a.Val, "mailto:") {
				continue
			}
			u, err := reqURL.Parse(a.Val)
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

func (c *Links) Prepare() (err error) {
	// TODO: compile IgnoredLinks
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
