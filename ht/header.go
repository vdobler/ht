// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package check provides useful checks for ht.
package ht

import (
	"fmt"
	"net/http"
	"time"
)

func init() {
	RegisterCheck(&Header{})
	RegisterCheck(&SetCookie{})
}

// ----------------------------------------------------------------------------
// Header

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

func (h *Header) Prepare() error {
	return h.Condition.Compile()
}

// ----------------------------------------------------------------------------
// SetCookie

// SetCookie checks for cookies beeing properly set.
// Note that the Path and Domain conditions are checked on the received Path
// and/or Domain and not on the interpreted values according to RFC 6265.
type SetCookie struct {
	Name   string    `json:",omitempty"` // Name is the cookie name.
	Value  Condition `json:",omitempty"` // Value is applied to the cookie value
	Path   Condition `json:",omitempty"` // Path is applied to the path value
	Domain Condition `json:",omitempty"` // Domain is applied to the domain value

	// MinLifetime is the expectetd minimum lifetime of the cookie.
	MinLifetime time.Duration `json:",omitempty"`

	// Absent indicates that the cookie with the given Name must not be received.
	Absent bool `json:",omitempty"`

	// TODO: check httpOnly and secure
}

func (c SetCookie) Execute(t *Test) error {
	var cookie *http.Cookie
	for _, cp := range t.Response.Response.Cookies() {
		if cp.Name == c.Name {
			cookie = cp
			break
		}
	}

	if cookie == nil {
		if c.Absent {
			return nil
		}
		return fmt.Errorf("Missing cookie %s", c.Name)
	}
	if c.Absent {
		return fmt.Errorf("Found cookie %s=%s", c.Name, cookie.Value)
	}

	if err := c.Value.Fullfilled(cookie.Value); err != nil {
		return err
	}
	if err := c.Path.Fullfilled(cookie.Path); err != nil {
		return err
	}
	if err := c.Value.Fullfilled(cookie.Domain); err != nil {
		return err
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

func (c *SetCookie) Prepare() error {
	if err := c.Value.Compile(); err != nil {
		return err
	}
	if err := c.Path.Compile(); err != nil {
		return err
	}
	return c.Domain.Compile()
}
