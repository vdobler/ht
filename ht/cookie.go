// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// cookie.go provides checks for cookies.

package ht

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func init() {
	RegisterCheck(&SetCookie{})
}

// SetCookie checks for cookies being properly set.
// Note that the Path and Domain conditions are checked on the received Path
// and/or Domain and not on the interpreted values according to RFC 6265.
type SetCookie struct {
	Name   string    `json:",omitempty"` // Name is the cookie name.
	Value  Condition `json:",omitempty"` // Value is applied to the cookie value
	Path   Condition `json:",omitempty"` // Path is applied to the path value
	Domain Condition `json:",omitempty"` // Domain is applied to the domain value

	// MinLifetime is the expectetd minimum lifetime of the cookie.
	// A positive value enforces a persistent cookie and a negative
	// value is is equivalent to Delete=true.
	MinLifetime Duration `json:",omitempty"`

	// Absent indicates that the cookie with the given Name must not be received.
	Absent bool `json:",omitempty"`

	// Delete indicates that the Set-Cookie header enforces a deletion
	// of the cookie.
	Delete bool `json:",omitempty"`

	// Type is the type of the cookie. It is a space seperated string of
	// the following (case-insensitive) keywords:
	//   - "session": a session cookie
	//   - "persistent": a persistent cookie
	//   - "secure": a secure cookie, to be sont over https only
	//   - "unsafe", to be sent also over http
	//   - "httpOnly": not accesible from JavaScript
	//   - "exposed": accesible from JavaScript, Flash, etc.
	Type string `json:",omitempty"`
}

// Execute implements Check's Execute method.
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

	if c.Delete {
		err := properlyDeleted(cookie)
		if err != nil {
			return err
		}
		// No need to check MinLifetime and Type is ignored
		// for delete requests.
		return nil
	}

	if c.MinLifetime > 0 {
		if cookie.MaxAge > 0 {
			if int(time.Duration(c.MinLifetime).Seconds()) > cookie.MaxAge {
				return fmt.Errorf("MaxAge %ds of cookie %s too short, want > %s",
					cookie.MaxAge, c.Name, c.MinLifetime)
			}
		} else if !cookie.Expires.IsZero() {
			min := time.Now().Add(time.Duration(c.MinLifetime))
			if min.Before(cookie.Expires) {
				return fmt.Errorf("Expires %ss of cookie %s too early, want > %s",
					cookie.Expires.Format(time.RFC1123), c.Name,
					min.Format(time.RFC1123))
			}
		} else {
			return fmt.Errorf("Cookie %s is session cookie", c.Name)
		}
	}

	return c.checkType(cookie)

}

// Prepare implements Check's Prepare method.
func (c *SetCookie) Prepare() error {
	if err := c.Value.Compile(); err != nil {
		return err
	}
	if err := c.Path.Compile(); err != nil {
		return err
	}
	if err := c.Domain.Compile(); err != nil {
		return err
	}

	if c.MinLifetime < 0 {
		c.MinLifetime = 0
		c.Delete = true
	}

	c.Type = strings.ToLower(c.Type)
	x := c.Type
	for _, t := range strings.Split("session persistent secure unsafe httponly exposed", " ") {
		x = strings.TrimSpace(strings.Replace(x, t, "", -1))
	}
	if x != "" {
		return fmt.Errorf("ht: unknown stuff in cookie type %q", x)
	}

	if c.MinLifetime > 0 {
		if strings.Index(c.Type, "persistent") == -1 {
			c.Type += " persistent"
		}
	}

	return nil
}

// properlyDeleted ensures the received cookie c is a proper
// delete request
func properlyDeleted(cookie *http.Cookie) error {
	if cookie.MaxAge == 0 && cookie.Expires.IsZero() {
		return fmt.Errorf("cookie not deleted", cookie.MaxAge)
	}

	if cookie.MaxAge > 0 {
		return fmt.Errorf("MaxAge=%d > 0, not deleted", cookie.MaxAge)
	}
	if !cookie.Expires.IsZero() {
		// Deletion via Expires value is problematic due to
		// clock variations. Play it safe and require at least
		// 90 seconds backdatedExpires.
		now := time.Now().Add(-90 * time.Second)
		if cookie.Expires.After(now) {
			return fmt.Errorf("Expires %s to late, not deleted", cookie.Expires)
		}
	}

	// Not both of MaxAge and Expires are unset; and none is positive,
	// so at least one is negative. Thus properly deleted.
	return nil
}

func (c *SetCookie) checkType(cookie *http.Cookie) error {
	t := c.Type
	if strings.Index(t, "session") != -1 {
		if cookie.MaxAge > 0 || !cookie.Expires.IsZero() {
			return fmt.Errorf("persistent cookie")
		}
	} else if strings.Index(t, "persistent") != -1 {
		if cookie.MaxAge == 0 && cookie.Expires.IsZero() {
			return fmt.Errorf("session cookie")
		}
	}

	if strings.Index(t, "secure") != -1 && !cookie.Secure {
		return fmt.Errorf("not a secure cookie")
	} else if strings.Index(t, "unsafe") != -1 && cookie.Secure {
		return fmt.Errorf("secure cookie")
	}

	if strings.Index(t, "httponly") != -1 && !cookie.HttpOnly {
		return fmt.Errorf("not a http-only cookie")
	} else if strings.Index(t, "exposed") != -1 && cookie.HttpOnly {
		return fmt.Errorf("http-only cookie")
	}

	return nil
}
