// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package check provides useful checks for ht.
package check

import (
	"bytes"
	"encoding/json"
	"fmt"

	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nytlabs/gojee"
	"github.com/vdobler/ht/fingerprint"
	"github.com/vdobler/ht/response"

	"code.google.com/p/cascadia"
	"code.google.com/p/go.net/html"
)

func init() {
	RegisterCheck(BodyContains{})
	RegisterCheck(&BodyMatch{})
	RegisterCheck(Header{})
	RegisterCheck(ResponseTime{})
	RegisterCheck(SetCookie{})
	RegisterCheck(ValidHTML{})
	RegisterCheck(UTF8Encoded{})
	RegisterCheck(&HTMLContains{})
	RegisterCheck(&HTMLContainsText{})
	RegisterCheck(&JSON{})
	RegisterCheck(Image{})
}

// ----------------------------------------------------------------------------
// StatusCode

// StatusCode checks the HTTP statuscode.
type StatusCode struct {
	Expect int `xml:",attr"`
}

func (c StatusCode) Okay(response *response.Response) error {
	if response.Response.StatusCode != c.Expect {
		return fmt.Errorf("got %d, want %d", response.Response.StatusCode, c.Expect)
	}
	return nil
}

// ----------------------------------------------------------------------------
// BodyContains

// BodyContains checks textual occurences in the response body.
type BodyContains struct {
	// Text is the literal text (no wildcards, no regexp) to look for in the body.
	Text string `xml:",attr"`

	// Count determines the number of occurences to check for:
	//     < 0: no occurence
	//    == 0: one ore more occurences
	//     > 0: exactly that many occurences
	Count int `json:",omitempty" xml:",attr,omitempty"`
}

func (c BodyContains) Okay(response *response.Response) error {
	body, err := response.Body, response.BodyErr
	text := []byte(c.Text)
	if err != nil {
		return BadBody
	}
	switch {
	case c.Count < 0:
		if pos := bytes.Index(body, text); pos != -1 {
			return FoundForbidden
		}
	case c.Count == 0:
		if pos := bytes.Index(body, text); pos == -1 {
			return NotFound
		}
	case c.Count > 0:
		if count := bytes.Count(body, text); count != c.Count {
			return WrongCount{Got: count, Want: c.Count}
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// BodyMatch

// BodyMatch checks the response body by matching a regular expression.
type BodyMatch struct {
	// Regexp is the regular expression to look for in the request body.
	Regexp string `xml:",attr"`

	// Count determines the number of occurences to check for:
	//     < 0: no occurence
	//    == 0: one ore more occurences
	//     > 0: exactly that many occurences
	Count int `xml:",attr,omitempty"`

	re *regexp.Regexp
}

func (c *BodyMatch) Okay(response *response.Response) error {
	if c.re == nil {
		err := c.Compile()
		if err != nil {
			return err
		}
	}
	if response.BodyErr != nil {
		return BadBody
	}

	m := c.re.FindAll(response.Body, -1)
	switch {
	case c.Count < 0 && m != nil:
		return FoundForbidden
	case c.Count == 0 && m == nil:
		return NotFound
	case c.Count > 0 && len(m) != c.Count:
		return WrongCount{Got: len(m), Want: c.Count}
	}
	return nil
}

func (c *BodyMatch) Compile() (err error) {
	c.re, err = regexp.Compile(c.Regexp)
	if err != nil {
		c.re = nil
		return MalformedCheck{Err: err}
	}
	return nil
}

// ----------------------------------------------------------------------------
// UTF8Encoded

// UTF8Encoded checks that the response body is valid UTF-8 without BOMs.
type UTF8Encoded struct{}

func (c UTF8Encoded) Okay(response *response.Response) error {
	p := response.Body
	char := 0
	for len(p) > 0 {
		r, size := utf8.DecodeRune(p)
		if r == utf8.RuneError {
			return fmt.Errorf("Invalid UTF-8 at character %d in body.", char)
		}
		if r == '\ufeff' { // BOMs suck.
			return fmt.Errorf("Unicode BOM at character %d.", char)
		}
		p = p[size:]
		char++
	}
	return nil
}

// ----------------------------------------------------------------------------
// Image

// Image checks image format, size and fingerprint. As usual a zero value of
// a field skipps the chekc of that property.
// Image fingerprinting is done via github.com/vdobler/ht/fingerprint.
// Only one of BMV or ColorHist should be used as there is just one threshold.
//
// Note that you have to register the apropriate image decoder functions
// with package image, e.g. by
//     import _ "image/png"
// if you want to check PNG images.
type Image struct {
	// Format is the format of the image as registered in package image.
	Format string

	// If > 0 check width or height of image.
	Width, Height int

	// BMV is the 16 hex digit long Block Mean Value hash of the image.
	BMV string

	// ColorHist is the 24 hex digit long Color Histogram hash of
	// the image.
	ColorHist string

	// Threshold is the limit up to which the received image may differ
	// from the given BMV or ColorHist fingerprint.
	Threshold float64
}

func (c Image) Okay(response *response.Response) error {
	img, format, err := image.Decode(response.BodyReader())
	if err != nil {
		return CantCheck{err}
	}
	// TODO: Do not abort on first failure.
	if c.Format != "" && format != c.Format {
		return fmt.Errorf("Got %s image, want %s", format, c.Format)
	}

	bounds := img.Bounds()
	if c.Width > 0 && c.Width != bounds.Dx() {
		return fmt.Errorf("Got %d px wide image, want %d",
			bounds.Dx(), c.Width)

	}
	if c.Height > 0 && c.Height != bounds.Dy() {
		return fmt.Errorf("Got %d px heigh image, want %d",
			bounds.Dy(), c.Height)

	}

	if c.BMV != "" {
		targetBMV, err := fingerprint.BMVHashFromString(c.BMV)
		if err != nil {
			return CantCheck{fmt.Errorf("bad BMV hash: %s", err)}
		}
		imgBMV := fingerprint.NewBMVHash(img)
		if fingerprint.BMVDelta(targetBMV, imgBMV) > c.Threshold {
			return fmt.Errorf("Got BMV of %s, want %s",
				imgBMV.String(), targetBMV.String())
		}

	}
	if c.ColorHist != "" {
		targetCH, err := fingerprint.ColorHistFromString(c.ColorHist)
		if err != nil {
			return CantCheck{fmt.Errorf("bad ColorHist hash: %s", err)}
		}
		imgCH := fingerprint.NewColorHist(img)
		if d := fingerprint.ColorHistDelta(targetCH, imgCH); d > c.Threshold {
			return fmt.Errorf("Got ColorHist of %s, want %s (delta=%.4f)",
				imgCH.String(), targetCH.String(), d)
		}
	}

	return nil
}

// ----------------------------------------------------------------------------
// Header

// Header
type Header struct {
	Header string    `xml:",attr"`
	Value  string    `xml:",attr,omitempty"`
	Cond   Condition `xml:",attr,omitempty"`
}

func (c Header) Okay(response *response.Response) error {
	key := http.CanonicalHeaderKey(c.Header)
	values := response.Response.Header[key]
	if len(values) == 0 {
		if c.Cond == Absent {
			return nil
		}
		return fmt.Errorf("Header %s not received", c.Header)
	}
	got := values[0]
	switch c.Cond {
	case Equal:
		if got != c.Value {
			return fmt.Errorf("Header %s==%q, want %q.", c.Header, got, c.Value)
		}
	case HasPrefix:
		if !strings.HasPrefix(got, c.Value) {
			return fmt.Errorf("Header %s==%q, want prefix %q.", c.Header, got, c.Value)
		}
	case HasSuffix:
		if !strings.HasSuffix(got, c.Value) {
			return fmt.Errorf("Header %s==%q, want suffix %q.", c.Header, got, c.Value)
		}
	case Absent:
		return fmt.Errorf("Header %s received", c.Header)
	case Present:

	default:
		panic(c.Cond)
	}
	return nil
}

// ----------------------------------------------------------------------------
// ResponseTime

// ResponseTime checks the response time.
type ResponseTime struct {
	Lower  time.Duration `xml:",attr,omitempty"`
	Higher time.Duration `xml:",attr,omitempty"`
}

func (c ResponseTime) Okay(response *response.Response) error {
	if c.Higher != 0 && c.Lower != 0 && c.Higher >= c.Lower {
		return MalformedCheck{Err: fmt.Errorf("%d<RT<%d unfullfillable", c.Higher, c.Lower)}
	}
	if c.Lower > 0 && c.Lower < response.Duration {
		return fmt.Errorf("Response took %s (allowed max %s).",
			response.Duration.String(), c.Lower.String())
	}
	if c.Higher > 0 && c.Higher > response.Duration {
		return fmt.Errorf("Response took %s (required min %s).",
			response.Duration.String(), c.Higher.String())
	}
	return nil
}

// ----------------------------------------------------------------------------
// SetCookie

// SetCookie checks for cookies beeing properly set
type SetCookie struct {
	Name        string        `xml:",attr"`
	Value       string        `xml:",attr,omitempty"`
	Cond        Condition     `xml:",attr,omitempty"`
	MinLifetime time.Duration `xml:",attr,omitempty"`
}

func (c SetCookie) Okay(response *response.Response) error {
	var cookie *http.Cookie
	for _, cp := range response.Response.Cookies() {
		if cp.Name == c.Name {
			cookie = cp
			break
		}
	}

	if cookie == nil && c.Cond != Absent {
		return fmt.Errorf("Missing cookie %s", c.Name)
	}
	if cookie != nil && c.Cond == Absent {
		return fmt.Errorf("Found cookie %s=%s", c.Name, cookie.Value)
	}

	switch c.Cond {
	case Equal:
		if cookie.Value != c.Value {
			return fmt.Errorf("Cookie %s=%s want %q", c.Name, cookie.Value, c.Value)
		}
	case HasPrefix:
		if !strings.HasPrefix(cookie.Value, c.Value) {
			return fmt.Errorf("Cookie %s=%s want prefix %q",
				c.Name, cookie.Value, c.Value)
		}
	case HasSuffix:
		if !strings.HasSuffix(cookie.Value, c.Value) {
			return fmt.Errorf("Cookie %s=%s want suffix %q",
				c.Name, cookie.Value, c.Value)
		}
	case Present:
	default:
		panic(c.Cond)
	}

	if c.MinLifetime > 0 {
		if cookie.MaxAge > 0 {
			if int(c.MinLifetime.Seconds()) > cookie.MaxAge {
				return fmt.Errorf("MaxAge %ds of cookie %s too short, want > %s",
					cookie.MaxAge, c.Name, c.MinLifetime)
			}
		} else if !cookie.Expires.IsZero() {
			min := time.Now().Add(c.MinLifetime)
			if min.Before(cookie.Expires) {
				return fmt.Errorf("Expires %ss of cookie %s too early, want > %s",
					cookie.Expires.Format(time.RFC1123), c.Name,
					min.Format(time.RFC1123))
			}
		} else {
			return fmt.Errorf("Cookie %s is session cookie", c.Name)
		}

	}

	return nil
}

// ----------------------------------------------------------------------------
// HTML checks

// ValidHTML checks for valid HTML 5. Kinda: It never fails.
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

// ----------------------------------------------------------------------------
// JSON

// JSON checking via github.com/nytlabs/gojee
type JSON struct {
	// Expression is a boolean gojee expression which must evaluate
	// to true for the check to pass.
	Expression string `xml:",attr"`

	tt *jee.TokenTree
}

func (c *JSON) Compile() (err error) {
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

func (c *JSON) Okay(response *response.Response) error {
	if c.tt == nil {
		if err := c.Compile(); err != nil {
			return MalformedCheck{Err: err}
		}
	}

	var bmsg jee.BMsg
	err := json.Unmarshal(response.Body, &bmsg)
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
		return Failed
	}
	return nil
}
