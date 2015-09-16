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
	RegisterCheck(&FinalURL{})
	RegisterCheck(&ContentType{})
	RegisterCheck(&Redirect{})
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
	return h.Fullfilled(values[0])
}

// Prepare implements Check's Prepare method.
func (h *Header) Prepare() error {
	return h.Condition.Compile()
}

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
	return Condition(f).Fullfilled(t.Response.Response.Request.URL.String())
}

// Prepare implements Check's Prepare method.
func (f *FinalURL) Prepare() error {
	return ((*Condition)(f)).Compile()
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
// Redirect

// Redirect checks for HTTP redirections.
type Redirect struct {
	// To is matched against the Location header. It may begin with,
	// or end with contain three dots "..." which inicate that To should
	// match the end or the start or both ends of the Location header
	// value. (Note that only one occurence of "..." is supported."
	To string

	// If StatusCode is greater zero it is the required HTTP status code
	// expected in this response. If zero the valid status codes are
	// 301 (Moved Permanently), 302 (Found), 303 (See Other) and
	// 307 (Temporary Redirect)
	StatusCode int `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (r Redirect) Execute(t *Test) error {
	err := []string{}

	if t.Response.Response == nil {
		return errors.New("no response to check")
	}

	sc := t.Response.Response.StatusCode
	if r.StatusCode > 0 {
		if sc != r.StatusCode {
			err = append(err, fmt.Sprintf("got status code %d", sc))
		}
	} else {
		if !(sc == 301 || sc == 302 || sc == 303 || sc == 307) {
			err = append(err, fmt.Sprintf("got status code %d", sc))
		}
	}

	if location, ok := t.Response.Response.Header["Location"]; !ok {
		err = append(err, "no Location header received")
	} else {
		if len(location) > 1 {
			err = append(err, fmt.Sprintf("got %d Location header", len(location)))
		}
		loc := location[0]
		if strings.HasPrefix(r.To, "...") {
			if !strings.HasSuffix(loc, r.To[3:]) {
				err = append(err, fmt.Sprintf("Location = %s", loc))
			}
		} else if strings.HasSuffix(r.To, "...") {
			if !strings.HasPrefix(loc, r.To[:len(r.To)-3]) {
				err = append(err, fmt.Sprintf("Location = %s", loc))
			}
		} else if i := strings.Index(r.To, "..."); i != -1 {
			a, e := r.To[:i], r.To[i+3:]
			if !(strings.HasPrefix(loc, a) && strings.HasSuffix(loc, e)) {
				err = append(err, fmt.Sprintf("Location = %s", loc))
			}
		} else if loc != r.To {
			err = append(err, fmt.Sprintf("Location = %s", loc))
		}
	}

	if len(err) > 0 {
		return errors.New(strings.Join(err, "; "))
	}

	return nil
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
