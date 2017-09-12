// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// validhtml.go contains checks to slightly validate a HTML document.

package ht

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/language"
)

func init() {
	RegisterCheck(ValidHTML{})
}

// ValidHTML checks for valid HTML 5; well kinda: It make sure that some
// common but easy to detect fuckups are not present. The following issues
// are detected:
//   * 'doctype':   not exactly one DOCTYPE
//   * 'structure': ill-formed tag nesting / tag closing
//   * 'uniqueids': uniqness of id attribute values
//   * 'lang':      ill-formed lang attributes
//   * 'attr':      duplicate attributes
//   * 'escaping':  unescaped &, < and > characters or unknown entities
//   * 'label':     reference to nonexisting ids in a label tags
//   * 'url':       malformed URLs
//
// Notes:
//  - HTML5 allows unescaped & in several circumstances but ValidHTML
//    reports most stray & as an error.
//  - The lang attributes are parse very lax, e.g. the non-canonical form
//    'de_CH' is considered valid (and equivalent to 'de-CH'). I don't
//    know how browser handle this.
type ValidHTML struct {
	// Ignore is a space separated list of issues to ignore.
	// You normally won't skip detection of these issues as all issues
	// are fundamental flaws which are easy to fix.
	Ignore string `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (v ValidHTML) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	mask, _ := ignoreMask(v.Ignore)
	state := newHTMLState(t.Response.BodyStr, mask)

	// Parse document and record local errors in state.
	z := html.NewTokenizer(state)
	depth := 0
done:
	for {
		tt := z.Next()
		// fmt.Println(tt)
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				break done
			}
			return z.Err()
		case html.TextToken:
			// fmt.Println("Text", string(z.Text()))
			// fmt.Println()
			if depth > 0 {
				// fmt.Println("Checked Text", string(z.Raw()))
				state.checkEscaping(string(z.Raw()))
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			raw := string(z.Raw())
			// <p class="foo">  ==> raw==`p class="foo"`
			if len(raw) > 3 {
				state.checkAttributeEscaping(raw[1 : len(raw)-1])
			}
			tn, hasAttr := z.TagName()
			// Some tags are empty and may be written in the self-closing
			// variant "<br/>" or simply empty like "<br>".
			// TODO: Maybe allow the non-compliant form "<br></br>" too?
			if tt != html.SelfClosingTagToken && !emptyHTMLElement[string(tn)] {
				state.push(string(tn))
				depth++
			}
			tag := string(tn)
			// fmt.Println("Tag", tag)
			state.count(tag)
			attrs := map[string]string{}
			var bkey, bval []byte
			for hasAttr {
				bkey, bval, hasAttr = z.TagAttr()
				key, val := string(bkey), string(bval)
				// fmt.Printf("Attribute %q = %q\n", key, val)
				// fmt.Printf("%s=%s ", key, val)
				if _, ok := attrs[key]; ok {
					if state.ignore&issueAttr == 0 {
						state.err(fmt.Errorf("Duplicate attribute '%s'", key))
					}
				}
				attrs[key] = val
				switch {
				case key == "id":
					state.checkID(val)
				case tag == "label" && key == "for":
					state.recordLabel(val)
				case key == "lang":
					state.checkLang(val)
				case isURLAttr(tag, key):
					state.checkURL(val)
				}
			}
			// fmt.Println()
		case html.EndTagToken:
			tn, _ := z.TagName()
			// fmt.Println(" End ", string(tn))
			state.pop(string(tn))
			depth--
		case html.CommentToken:
		case html.DoctypeToken:
			state.count("DOCTYPE")
		}
	}

	// Check for global errors.
	state.line++ // Global errors are reported "after the last line".
	if state.ignore&issueDoctype == 0 {
		if d := state.elementCount["DOCTYPE"]; d != 1 {
			state.err(fmt.Errorf("Found %d DOCTYPE", d))
		}
	}
	if state.ignore&issueLabelRef == 0 {
		for _, id := range state.labelFor {
			if _, ok := state.seenIDs[id]; !ok {
				state.err(fmt.Errorf("Label references unknown id '%s'", id))
			}
		}
	}

	if len(state.errors) == 0 {
		return nil
	}

	if len(state.errors) == 0 {
		return nil
	}
	return state.errors
}

// return true if attr contains the URL of the tag.
func isURLAttr(tag, attr string) bool {
	if a, ok := linkURLattr[tag]; ok {
		return attr == a.attr
	}
	return false
}

// Prepare implements Check's Prepare method.
func (v ValidHTML) Prepare(*Test) error {
	_, err := ignoreMask(v.Ignore)
	return err
}

var _ Preparable = ValidHTML{}

// emptyHTMLElement is the list of all HTML5 elements which are empty, that
// is they are implecitely self-closing and can be written either in
// XML-style like e.g. "<br/>" or in HTML5-style just "<br>".
// The list was taken from:
//   - http://www.elharo.com/blog/software-development/web-development/2007/01/29/all-empty-tags-in-html/
// Someone should check the whole list here: http://www.w3schools.com/tags/default.asp
var emptyHTMLElement = map[string]bool{
	"br":    true,
	"hr":    true,
	"meta":  true,
	"base":  true,
	"link":  true,
	"img":   true,
	"embed": true,
	"param": true,
	"area":  true,
	"col":   true,
	"input": true,
}

// ----------------------------------------------------------------------------
// Types of issues to ignore

type htmlIssue uint32

const (
	issueIgnoreNone htmlIssue = 0
	issueDoctype    htmlIssue = 1 << (iota - 1)
	issueStructure
	issueUniqIDs
	issueLangTag
	issueAttr
	issueEscaping
	issueLabelRef
	issueURL
)

func ignoreMask(s string) (htmlIssue, error) {
	// what an ugly hack
	const issueNames = "doctype  structureuniqueidslang     attr     escaping label    url"
	mask := issueIgnoreNone
	s = strings.ToLower(s)
	for _, p := range strings.Split(s, " ") {
		if p == "" {
			continue
		}
		i := strings.Index(issueNames, p)
		if i == -1 {
			return mask, fmt.Errorf("no such html issue '%s'", p)
		}
		mask |= 1 << uint(i/9)
	}
	return mask, nil
}

// ----------------------------------------------------------------------------
// htmlState

// htmlState collects information about a HTML document.
type htmlState struct {
	body string
	i    int
	line int

	elementCount map[string]int
	seenIDs      map[string]bool
	openTags     []string
	labelFor     []string
	errors       ErrorList

	ignore     htmlIssue
	badNesting bool
}

func newHTMLState(body string, ignore htmlIssue) *htmlState {
	return &htmlState{
		body:         body,
		i:            0,
		line:         0,
		elementCount: make(map[string]int, 50),
		seenIDs:      make(map[string]bool),
		openTags:     make([]string, 0, 50),
		labelFor:     make([]string, 0, 10),
		errors:       make(ErrorList, 0),
		badNesting:   false,
		ignore:       ignore,
	}
}

func (s *htmlState) Read(buf []byte) (int, error) {
	n := 0
	last := byte(0)
	for n < len(buf) && s.i < len(s.body) && last != '\n' {
		buf[n] = s.body[s.i]
		last = s.body[s.i]
		n++
		s.i++
	}
	s.line++
	if s.i == len(s.body) {
		return n, io.EOF
	}
	return n, nil
}

// err records the error e.
func (s *htmlState) err(e error) {
	s.errors = append(s.errors, PosError{Err: e, Line: s.line})
}

// count the tag
func (s *htmlState) count(tag string) {
	s.elementCount[tag] = s.elementCount[tag] + 1
}

// checkID chesk for duplicate ids.
func (s *htmlState) checkID(id string) {
	if s.ignore&issueUniqIDs == 0 {
		if _, seen := s.seenIDs[id]; seen {
			s.err(fmt.Errorf("Duplicate id '%s'", id))
		}
	}
	s.seenIDs[id] = true
}

// record the id from a <label for="id"> tag.
func (s *htmlState) recordLabel(id string) {
	s.labelFor = append(s.labelFor, id)
}

func isAlphanumericASCII(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '0')
}

// checkEscaping of stuff inside a tag like
//     p class="important" title="Foo & Bar"
// Look for ambiguous apersand as defined in
// https://w3c.github.io/html/syntax.html#ambiguous-ampersand :
//
//   An ambiguous ampersand is a U+0026 AMPERSAND character (&) that is
//   followed by one or more alphanumeric ASCII characters, followed by
//   a U+003B SEMICOLON character (;), where these characters do not match
//   any of the names given in the §8.5 Named character references section.
//
//   The alphanumeric ASCII characters are those that are either uppercase
//   ASCII letters, lowercase ASCII letters, or ASCII digits.
func (s *htmlState) checkAttributeEscaping(text string) {
	if s.ignore&issueEscaping != 0 {
		return
	}

	for len(text) > 0 {
		// fmt.Println(text)
		i := strings.Index(text, "&")
		if i == -1 {
			break
		}
		text = text[i:]
		if strings.HasPrefix(text, "&amp;") {
			text = text[5:]
			continue
		}

		i = 1
		for i < len(text) && isAlphanumericASCII(text[i]) {
			i++
		}
		if i < len(text) && text[i] == ';' {
			i++
			cr := text[0:i] // character reference like &Prime;
			ue := html.UnescapeString(cr)
			if strings.HasPrefix(ue, "&") {
				// Then cr was not defined.
				s.err(fmt.Errorf("Unknown entity %s", cr))
				return
			}
		}
		text = text[i+1:]
	}
}

// checkEscaping of text
func (s *htmlState) checkEscaping(text string) {
	// fmt.Println("checkEscpaing of", text)
	if s.ignore&issueEscaping != 0 {
		return
	}

	// Javascript is full of unescaped <, > and && and content of 'noscript'
	// and 'iframe' seems to be unparsed by package html: Skip check of
	// proper escaping inside these elements.
	if n := len(s.openTags); n > 0 &&
		(s.openTags[n-1] == "script" || s.openTags[n-1] == "noscript" || s.openTags[n-1] == "iframe") {
		return
	}

	if strings.Contains(text, "<") {
		s.err(fmt.Errorf("Unescaped '<'"))
	}
	if strings.Contains(text, ">") {
		s.err(fmt.Errorf("Unescaped '>'"))
	}

	for len(text) > 0 {
		i := strings.Index(text, "&")
		if i == -1 {
			break
		}

		text = text[i:]
		if strings.HasPrefix(text, "&amp;") {
			// &amp; is fine and would produce a & which confuses
			// the check for improper escaping below.
			text = text[5:]
			continue
		}

		ue := html.UnescapeString(text)
		if strings.HasPrefix(ue, "&") {
			s.err(fmt.Errorf("Unescaped '&' or unknow entity in %s",
				trimUnescapedText(text)))
			break
		}
		text = text[1:]
	}
}

func trimUnescapedText(s string) string {
	i := strings.IndexFunc(s, func(r rune) bool {
		if r <= ' ' || r == ';' {
			return true
		}
		return false
	})
	if i != -1 {
		s = s[:i]
	}

	if len(s) < 20 {
		return s
	}
	return s[:20] + "\u2026"
}

// checkLang tries to parse the language tag lang.
func (s *htmlState) checkLang(lang string) {
	if s.ignore&issueLangTag != 0 {
		return
	}
	_, err := language.Parse(lang)
	switch e := err.(type) {
	case nil:
		// No error.
	case language.ValueError:
		s.err(fmt.Errorf("Language tag '%s' has bad part %s", lang, e.Subtag()))
	default:
		// A syntax error.
		s.err(fmt.Errorf("Language tag '%s' is ill-formed", lang))
	}
}

// checkURL raw to be properly encoded. HTML escaping has been checked already.
// for now just dissalow spaces
func (s *htmlState) checkURL(raw string) {
	if s.ignore&issueURL != 0 {
		return
	}

	// mailto: and tel: anchors
	if strings.HasPrefix(raw, "mailto:") {
		if !strings.Contains(raw, "@") {
			s.err(fmt.Errorf("Not an email address"))
		}
		return
	}
	if strings.HasPrefix(raw, "tel:") {
		s.checkTelURL(raw[4:])
		return
	}
	if strings.HasPrefix(raw, "data:") {
		s.checkDataURL(raw[5:])
		return
	}

	u, err := url.Parse(raw)
	if err != nil {
		s.err(fmt.Errorf("Bad URL '%s': %s", raw, err.Error()))
		return
	}
	if u.Opaque != "" {
		s.err(fmt.Errorf("Bad URL part '%s'", u.Opaque))
		return
	}

	if strings.Contains(raw, " ") {
		s.err(fmt.Errorf("Unencoded space in URL"))
	}
}

func (s *htmlState) checkTelURL(raw string) {
	if !strings.HasPrefix(raw, "+") {
		s.err(fmt.Errorf("Telephone numbers must start with +"))
	}
	raw = raw[1:]
	if len(raw) == 0 {
		s.err(fmt.Errorf("Missing actual telephone number"))
	}
	raw = strings.TrimLeft(raw, "0123456789-")
	if len(raw) != 0 {
		s.err(fmt.Errorf("Not a telephone number"))
	}
}

func (s *htmlState) checkDataURL(raw string) {
	// Data URLS have the format:
	//    data:[<mediatype>][;base64],<data>
	// The "data:" prefix has been stripped by the caller.

	comma := strings.Index(raw, ",")
	if comma == -1 {
		s.err(fmt.Errorf("Missing , before actual data"))
		return
	}

	rawMT, data := raw[:comma], raw[comma+1:]

	// TODO: QueryUnescape is not the perfect solution as data: urls
	// should use RFC 2397 URL encoding which encodes spaces as %20.
	// But the unencoding should work (at least for valid data: urls).
	escaped, err := url.QueryUnescape(data)
	if err != nil {
		s.err(fmt.Errorf("Badly escaped data section: %s", err))
		return
	}

	b64 := strings.HasSuffix(rawMT, ";base64")
	if b64 {
		rawMT = rawMT[:len(rawMT)-len(";base64")]
	}

	if rawMT != "" {
		_, _, err = mime.ParseMediaType(rawMT)
		if err != nil {
			s.err(fmt.Errorf("Problems parsing media type in %q: %s",
				rawMT, err))
			return
		}
	}

	if b64 {
		_, err := base64.StdEncoding.DecodeString(escaped)
		if err != nil {
			s.err(fmt.Errorf("%s in %s", err, data))
			return
		}
	}
}

// push tag on stack of open tags
func (s *htmlState) push(tag string) {
	s.openTags = append(s.openTags, tag)
}

// try to pop tag from stack of open tags, record error if failed.
func (s *htmlState) pop(tag string) {
	n := len(s.openTags)
	if n == 0 {
		if s.ignore&issueStructure == 0 {
			s.err(fmt.Errorf("No open tags left to close %s", tag))
		}
		return
	}
	pop := s.openTags[n-1]
	s.openTags = s.openTags[:n-1]
	if s.ignore&issueStructure != 0 {
		return
	}
	if pop != tag && !s.badNesting { // report broken structure just once.
		s.err(fmt.Errorf("Tag '%s' closed by '%s'", pop, tag))
		s.badNesting = true
	}
}
