// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// validhtml.go contains checks to slighty validate a HTML document.

package ht

import (
	"fmt"
	"io"
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
//   * missing DOCTYPE
//   * multiple DOCTYPE
//   * ill-formed general structure (missing head, body, etc.)
//   * ill-formed lang attributes
//   * ill-formed tag nesting / tag closing
//   * duplicate id attribute values
//   * dupplicate class attributes
//   * for attribute in label tag referencing nonexistent ids
//   * unescaped & characters
type ValidHTML struct {
	// Ignore is a space seperated list of issues to ignore.
	Ignore string `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (c ValidHTML) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	state := newHtmlState(t.Response.BodyStr)

	// Parse document and record local errors in state.
	z := html.NewTokenizer(state)
	depth := 0
done:
	for {
		tt := z.Next()
		// fmt.Printf("%s%s: ", strings.Repeat("  ", depth), tt)
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				break done
			}
			return z.Err()
		case html.TextToken:
			// fmt.Printf(" %q\n", z.Text())
			if depth > 0 {
				state.checkEscaping(string(z.Raw()))
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			if tt == html.StartTagToken {
				depth++
			}
			raw := string(z.Raw())
			if len(raw) > 3 {
				state.checkEscaping(raw[1 : len(raw)-1])
			}
			tn, hasAttr := z.TagName()
			if tt != html.SelfClosingTagToken {
				state.push(string(tn))
			}
			tag := string(tn)
			// fmt.Printf(" %s  ", tag)
			state.count(tag)
			attrs := map[string]string{}
			var bkey, bval []byte
			for hasAttr {
				bkey, bval, hasAttr = z.TagAttr()
				key, val := string(bkey), string(bval)
				// fmt.Printf("%s=%s ", key, val)
				if _, ok := attrs[key]; ok {
					state.err(fmt.Errorf("duplicate attribute '%s'", key))
				}
				attrs[key] = val
				switch {
				case key == "id":
					state.checkID(val)
				case tag == "label" && key == "for":
					state.recordLabel(val)
				case key == "lang":
					state.checkLang(val)
				case (tag == "a" && key == "href") ||
					(tag == "img" && key == "src"): // TODO link?
					state.checkURL(val)
				}
			}
			// fmt.Println()
		case html.EndTagToken:
			tn, _ := z.TagName()
			// fmt.Println(" ", string(tn))
			state.pop(string(tn))
			depth--
		case html.CommentToken:
		case html.DoctypeToken:
			state.count("DOCTYPE")
		}
	}

	// Check for global errors.
	state.line++ // Global errors are reported "after the last line".
	if d := state.elementCount["DOCTYPE"]; d != 1 {
		state.err(fmt.Errorf("found %d DOCTYPE", d))
	}
	for _, id := range state.labelFor {
		if _, ok := state.seenIDs[id]; !ok {
			state.err(fmt.Errorf("label references unknown id '%s'", id))
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

// Prepare implements Check's Prepare method.
func (ValidHTML) Prepare() error { return nil }

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

	badNesting bool
}

func newHtmlState(body string) *htmlState {
	return &htmlState{
		body:         body,
		i:            0,
		line:         0,
		elementCount: make(map[string]int, 50),
		seenIDs:      make(map[string]bool),
		openTags:     make([]string, 0, 50),
		labelFor:     make([]string, 0, 10),
		errors:       make(ErrorList, 0),
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
	if _, seen := s.seenIDs[id]; seen {
		s.err(fmt.Errorf("duplicate id '%s'", id))
	}
	s.seenIDs[id] = true
}

// record the id from a <label for="id"> tag.
func (s *htmlState) recordLabel(id string) {
	s.labelFor = append(s.labelFor, id)
}

// checkEscaping of text
func (s *htmlState) checkEscaping(text string) {
	if strings.Index(text, "<") != -1 {
		s.err(fmt.Errorf("unescaped '<'"))
	}
	if strings.Index(text, ">") != -1 {
		s.err(fmt.Errorf("unescaped '>'"))
	}
	for len(text) > 0 {
		if i := strings.Index(text, "&"); i != -1 {
			text = text[i:]
			if strings.HasPrefix(text, "&amp;") {
				text = text[5:]
			} else {
				ue := html.UnescapeString(text)
				if strings.HasPrefix(ue, "&") {
					s.err(fmt.Errorf("unescaped '&' or unknow entity"))
					break
				}
				text = text[1:]
			}
		} else {
			break
		}
	}
}

// checkLang tries to parse the language tag lang.
func (s *htmlState) checkLang(lang string) {
	_, err := language.Parse(lang)
	switch e := err.(type) {
	case nil:
		// No error.
	case language.ValueError:
		s.err(fmt.Errorf("language tag '%s' has bad part %s", lang, e.Subtag()))
	default:
		// A syntax error.
		s.err(fmt.Errorf("language tag '%s' is ill-formed", lang))
	}
}

// checkURL raw to be properly encoded
func (s *htmlState) checkURL(raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		s.err(fmt.Errorf("bad URL '%s': %s", raw, err.Error()))
		return
	}
	if u.Opaque != "" {
		s.err(fmt.Errorf("bad URL part '%s'", u.Opaque))
		return
	}
	// Check path first
	if should := u.EscapedPath(); u.Path != should {
		s.err(fmt.Errorf("URL path '%s' should be '%s'", u.Path, should))
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
		s.err(fmt.Errorf("no open tags left to close %s", tag))
		return
	}
	pop := s.openTags[n-1]
	s.openTags = s.openTags[:n-1]
	if pop != tag && !s.badNesting { // report broken structure just once.
		s.err(fmt.Errorf("tag '%s' closed by '%s'", pop, tag))
		s.badNesting = true
	}
}
