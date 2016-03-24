// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// header.go provides generic checks of HTTP headers.
// Checks for cookie headers reside in cookie.go

package ht

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func init() {
	RegisterCheck(&Header{})
	RegisterCheck(&ContentType{})
	RegisterCheck(&FinalURL{})
	RegisterCheck(&Redirect{})
	RegisterCheck(&RedirectChain{})
}

// Header provides a textual test of single-valued HTTP headers.
type Header struct {
	// Header is the HTTP header to check.
	Header string

	// Condition is applied to the first header value. A zero value checks
	// for the existence of the given Header only.
	Condition `json:",omitempty"`

	// Absent indicates that no header Header shall be part of the response.
	Absent bool `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (h Header) Execute(t *Test) error {
	key := http.CanonicalHeaderKey(h.Header)
	values := t.Response.Response.Header[key]
	if len(values) == 0 && h.Absent {
		return nil
	} else if len(values) == 0 && !h.Absent {
		return fmt.Errorf("header %s not received", h.Header)
	} else if len(values) > 0 && h.Absent {
		return fmt.Errorf("forbidden header %s received", h.Header)
	}
	return h.Fulfilled(values[0])
}

// Prepare implements Check's Prepare method.
func (h *Header) Prepare() error {
	return h.Condition.Compile()
}

// ----------------------------------------------------------------------------
// ContentType

// ContentType checks the Content-Type header.
type ContentType struct {
	// Is is the wanted content type. It may be abrevated, e.g.
	// "json" would match "application/json"
	Is string

	// Charset is an optional charset
	Charset string `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (c ContentType) Execute(t *Test) error {
	if t.Response.Response == nil || t.Response.Response.Header == nil {
		return fmt.Errorf("no proper response available")
	}
	ct := t.Response.Response.Header["Content-Type"]
	if len(ct) == 0 {
		return fmt.Errorf("no Content-Type header received")
	}
	if len(ct) > 1 {
		// This is technically not a failure, but if someone sends
		// mutliple Content-Type headers something is a bit odd.
		return fmt.Errorf("received %d Content-Type headers", len(ct))
	}
	parts := strings.Split(ct[0], ";")
	got := strings.TrimSpace(parts[0])
	want := c.Is
	if strings.Index(want, "/") == -1 {
		want = "/" + want
	}
	if !strings.HasSuffix(got, want) {
		return fmt.Errorf("Content-Type is %s", ct[0])
	}

	if c.Charset != "" {
		if len(parts) < 2 {
			return fmt.Errorf("no charset in %s", ct[0])
		}
		got := strings.TrimSpace(parts[1])
		want := "charset=" + c.Charset
		if got != want {
			return fmt.Errorf("bad charset in %s", ct[0])
		}
	}

	return nil
}

// Prepare implements Check's Prepare method.
func (ContentType) Prepare() error { return nil }

// ----------------------------------------------------------------------------
// FinalURL

// FinalURL checks the last URL after following all redirects.
// This check is useful only for tests with Request.FollowRedirects=true
type FinalURL Condition

// Execute implements Check's Execute method.
func (f FinalURL) Execute(t *Test) error {
	if t.Response.Response == nil || t.Response.Response.Request == nil ||
		t.Response.Response.Request.URL == nil {
		return fmt.Errorf("no request URL to analyze")
	}
	return Condition(f).Fulfilled(t.Response.Response.Request.URL.String())
}

// Prepare implements Check's Prepare method.
func (f *FinalURL) Prepare() error {
	return ((*Condition)(f)).Compile()
}

// ----------------------------------------------------------------------------
// Redirect

// Redirect checks for a singe HTTP redirection.
//
// Note that this check cannot be used on tests with
//     Request.FollowRedirects = true
// as Redirect checks only the final response which will not be a
// redirection if redirections are followed automatically.
type Redirect struct {
	// To is matched against the Location header. It may begin with,
	// end with or contain three dots "..." which indicate that To should
	// match the end, the start or both ends of the Location header
	// value. (Note that only one occurrence of "..." is supported."
	To string

	// If StatusCode is greater zero it is the required HTTP status code
	// expected in this response. If zero, the valid status codes are
	// 301 (Moved Permanently), 302 (Found), 303 (See Other) and
	// 307 (Temporary Redirect)
	StatusCode int `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (r Redirect) Execute(t *Test) error {
	err := ErrorList{}

	if t.Response.Response == nil {
		return errors.New("no response to check")
	}

	sc := t.Response.Response.StatusCode
	if r.StatusCode > 0 {
		if sc != r.StatusCode {
			err = append(err, fmt.Errorf("got status code %d", sc))
		}
	} else {
		if !(sc == 301 || sc == 302 || sc == 303 || sc == 307) {
			err = append(err, fmt.Errorf("got status code %d", sc))
		}
	}

	if location, ok := t.Response.Response.Header["Location"]; !ok {
		err = append(err, fmt.Errorf("no Location header received"))
	} else {
		if len(location) > 1 {
			err = append(err, fmt.Errorf("got %d Location header", len(location)))
		}
		loc := location[0]
		if !dotMatch(loc, r.To) {
			err = append(err, fmt.Errorf("Location = %s", loc))
		}
	}

	if len(err) > 0 {
		return err
	}
	return nil
}

// dotMatch returns whether got ...-equals want, e.g.
// got "foo 123 bar" ...-matches "foo 12..."
func dotMatch(got, want string) bool {
	if strings.HasPrefix(want, "...") {
		return strings.HasSuffix(got, want[3:])
	} else if strings.HasSuffix(want, "...") {
		return strings.HasPrefix(got, want[:len(want)-3])
	} else if i := strings.Index(want, "..."); i != -1 {
		a, e := want[:i], want[i+3:]
		return strings.HasPrefix(got, a) && strings.HasSuffix(got, e)
	}
	return got == want
}

// Prepare implements Check's Prepare method.
func (r Redirect) Prepare() error {
	if r.To == "" {
		return MalformedCheck{fmt.Errorf("To must not be empty")}
	}

	if r.StatusCode > 0 && (r.StatusCode < 300 || r.StatusCode > 399) {
		return MalformedCheck{fmt.Errorf("status code %d out of redirect range", r.StatusCode)}
	}
	return nil
}

// ----------------------------------------------------------------------------
// RedirectChain

// RedirectChain checks steps in a redirect chain.
// The check passes if all stations in Via have been accessed in order; the
// actual redirect chain may hit additional stations.
//
// Note that this check can be used on tests with
//     Request.FollowRedirects = true
type RedirectChain struct {
	// Via contains the necessary URLs accessed during a redirect chain.
	//
	// Any URL may start with, end with or contain three dots "..." which
	// indicate a suffix, prefix or suffix+prefix match like in the To
	// field of Redirect.
	Via []string
}

// Execute implements Check's Execute method.
func (r RedirectChain) Execute(t *Test) error {
	reds := t.Response.Redirections
	if len(reds) == 0 {
		return errors.New("no redirections at all")
	}
	j := 0
	for i, via := range r.Via {
		for ; j < len(reds) && !dotMatch(reds[j], via); j++ {
		}
		if j >= len(reds) {
			return fmt.Errorf("redirect step %d (%s) not hit", i+1, via)
		}
		j++
	}

	return nil
}

// Prepare implements Check's Prepare method.
func (r RedirectChain) Prepare() error {
	if len(r.Via) == 0 {
		return MalformedCheck{fmt.Errorf("Via must not be empty")}
	}
	return nil
}
