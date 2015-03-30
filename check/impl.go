// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package check provides useful checks for ht.
package check

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vdobler/ht/response"
)

func init() {
	RegisterCheck(Header{})
	RegisterCheck(SetCookie{})
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
