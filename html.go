// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// html.go contains checks on a HTML body.

package ht

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

func init() {
	RegisterCheck(&HTMLContains{})
	RegisterCheck(&HTMLContainsText{})
	RegisterCheck(ValidHTML{})
	RegisterCheck(W3CValidHTML{})
}

// ----------------------------------------------------------------------------
// ValidHTML and W3CValidHTML

// ValidHTML checks for valid HTML 5. Kinda: It never fails. TODO: make it useful.
type ValidHTML struct{}

func (c ValidHTML) Execute(t *Test) error {
	if t.response.BodyErr != nil {
		return BadBody
	}
	_, err := html.Parse(t.response.BodyReader())
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
	// It would be nice to use a ht.Test here but that would be
	// a circular dependency, so craft the request by hand.

	// Create a multipart body.
	var body *bytes.Buffer = &bytes.Buffer{}
	var mpwriter = multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		`form-data; name="uploaded_file"; filename="sample.html"`)
	h.Set("Content-Type", "text/html")
	fw, err := mpwriter.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(fw, t.response.BodyReader())
	if err != nil {
		return err
	}
	for _, p := range []struct {
		name  string
		value string
	}{
		{"charset", "(detect automatically)"},
		{"fbc", "1"},
		{"doctype", "Inline"},
		{"fbd", "1"},
		{"group", "0"},
	} {
		err = mpwriter.WriteField(p.name, p.value)
		if err != nil {
			return err
		}
	}
	err = mpwriter.Close()
	if err != nil {
		return err
	}

	// Create and do a request to the W3C validator
	valReq, err := http.NewRequest("POST", "http://validator.w3.org/check", body)
	if err != nil {
		return err
	}
	valReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	valReq.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2272.101 Safari/537.36")
	// valReq.Header.Set("Accept-Encoding", "gzip, deflate")
	ct := fmt.Sprintf("multipart/form-data; boundary=%s", mpwriter.Boundary())
	valReq.Header.Set("Content-Type", ct)
	valResp, err := http.DefaultClient.Do(valReq)
	if err != nil {
		return err
	}
	defer valResp.Body.Close()

	// Interprete response from validator
	for h, vv := range valResp.Header {
		if !strings.HasPrefix(vv[0], "X-W3c-Validator") {
			continue
		}
		fmt.Printf("%-30s = %s\n", h, vv[0])
	}

	doc, err := html.Parse(valResp.Body)
	if err != nil {
		return err
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
	w3cValidatorErrSel   = cascadia.MustCompile("li.msg_err")
	w3cValidatorWarnSel  = cascadia.MustCompile("li.msg_warn")
	w3cValidatorEmSel    = cascadia.MustCompile("em")
	w3cValidatorMsgSel   = cascadia.MustCompile("span.msg")
	w3cValidatorInputSel = cascadia.MustCompile("code.input")
)

func extractValidationIssue(node *html.Node) ValidationIssue {
	p := textContent(w3cValidatorEmSel.MatchFirst(node))
	p = strings.Replace(p, "\n     ", "", -1)
	return ValidationIssue{
		Position: p,
		Message:  textContent(w3cValidatorMsgSel.MatchFirst(node)),
		Input:    textContent(w3cValidatorInputSel.MatchFirst(node)),
	}
}

// ----------------------------------------------------------------------------
// HTMLContains

// HTMLContains checks for the existens of HTML elements selected by CSS selectors.
type HTMLContains struct {
	// Selector is the CSS selector of the HTML elements.
	Selector string

	// Count determines the number of occurences to check for:
	//     < 0: no occurence
	//    == 0: one ore more occurences
	//     > 0: exactly that many occurences
	Count int `json:",omitempty"`

	sel cascadia.Selector
}

func (c *HTMLContains) Execute(t *Test) error {
	if c.sel == nil {
		if err := c.Prepare(); err != nil {
			return err
		}
	}
	if t.response.BodyErr != nil {
		return BadBody
	}

	doc, err := html.Parse(t.response.BodyReader())
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

func (c *HTMLContains) Prepare() (err error) {
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
	Selector string

	// The plain text content of each selected element.
	Text []string `json:",omitempty"`

	// If true: Text contains the all matches of Selector.
	Complete bool `json:",omitempty"`

	sel cascadia.Selector
}

func (c *HTMLContainsText) Execute(t *Test) error {
	if c.sel == nil {
		if err := c.Prepare(); err != nil {
			return err
		}
	}
	if t.response.BodyErr != nil {
		return BadBody
	}
	doc, err := html.Parse(t.response.BodyReader())
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

func (c *HTMLContainsText) Prepare() (err error) {
	c.sel, err = cascadia.Compile(c.Selector)
	if err != nil {
		c.sel = nil
		return MalformedCheck{Err: err}
	}
	return nil
}

// textContent returns the full text content of n.
func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}
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

// ----------------------------------------------------------------------------
// Links

// Links checks links and references in HTML pages for availability
type Links struct {
	// Head triggers HEAD request insted of GET requests.
	Head bool

	// Which links to test; a combination of "a", "img", "link" and "script"
	Which string

	tags []string
}

func (c *Links) Execute(response *Response) error {
	if response.BodyErr != nil {
		return BadBody
	}
	doc, err := html.Parse(response.BodyReader())
	if err != nil {
		return CantCheck{err}
	}

	refs := []string{}
	for _, tag := range c.tags {
		refs = append(refs, c.linkURL(doc, tag, response.Response.Request.URL)...)
	}
	for _, r := range refs {
		println(r)
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
			if a.Key != ak || a.Val == "#" {
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
	c.Which = strings.ToLower(c.Which)
	c.Which = strings.Replace(c.Which, ", ", ",", -1)
	c.Which = strings.Replace(c.Which, " ", ",", -1)

	for _, tag := range strings.Split(c.Which, ",") {
		tag = strings.TrimSpace(tag)
		switch tag {
		case "": // ignored
		case "a", "img", "link", "script":
			c.tags = append(c.tags, tag)
		default:
			return fmt.Errorf("Unknown link tag %q", tag)
		}
	}
	c.Which = strings.Join(c.tags, ", ")
	return nil
}
