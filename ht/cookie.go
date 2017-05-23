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
	RegisterCheck(&DeleteCookie{})
}

func findCookiesByName(t *Test, name string) (cookies []*http.Cookie) {
	for _, cp := range t.Response.Response.Cookies() {
		if cp.Name == name {
			cookies = append(cookies, cp)
		}
	}
	return cookies
}

// ----------------------------------------------------------------------------
// SetCookie

// SetCookie checks for cookies being properly set.
// Note that the Path and Domain conditions are checked on the received Path
// and/or Domain and not on the interpreted values according to RFC 6265.
type SetCookie struct {
	Name   string    `json:",omitempty"` // Name is the cookie name.
	Value  Condition `json:",omitempty"` // Value is applied to the cookie value
	Path   Condition `json:",omitempty"` // Path is applied to the path value
	Domain Condition `json:",omitempty"` // Domain is applied to the domain value

	// MinLifetime is the expectetd minimum lifetime of the cookie.
	// A positive value enforces a persistent cookie.
	// Negative values are illegal (use DelteCookie instead).
	MinLifetime time.Duration `json:",omitempty"`

	// Absent indicates that the cookie with the given Name must not be received.
	Absent bool `json:",omitempty"`

	// Type is the type of the cookie. It is a space separated string of
	// the following (case-insensitive) keywords:
	//   - "session": a session cookie
	//   - "persistent": a persistent cookie
	//   - "secure": a secure cookie, to be sont over https only
	//   - "unsafe", aka insecure; to be sent also over http
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

	if err := c.Value.Fulfilled(cookie.Value); err != nil {
		return fmt.Errorf("Bad value: %s", err)
	}
	if err := c.Path.Fulfilled(cookie.Path); err != nil {
		return fmt.Errorf("Bad path: %s", err)
	}
	if err := c.Domain.Fulfilled(cookie.Domain); err != nil {
		return fmt.Errorf("Bad Domain: %s", err)
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

	return c.checkType(cookie)

}
func (c *SetCookie) checkType(cookie *http.Cookie) error {
	t := c.Type
	if strings.Contains(t, "session") {
		if cookie.MaxAge > 0 || !cookie.Expires.IsZero() {
			return fmt.Errorf("persistent cookie")
		}
	} else if strings.Contains(t, "persistent") {
		if cookie.MaxAge == 0 && cookie.Expires.IsZero() {
			return fmt.Errorf("session cookie")
		}
	}

	if strings.Contains(t, "secure") && !cookie.Secure {
		return fmt.Errorf("not a secure cookie")
	} else if strings.Contains(t, "unsafe") && cookie.Secure {
		return fmt.Errorf("secure cookie")
	}

	if strings.Contains(t, "httponly") && !cookie.HttpOnly {
		return fmt.Errorf("not a http-only cookie")
	} else if strings.Contains(t, "exposed") && cookie.HttpOnly {
		return fmt.Errorf("http-only cookie")
	}

	return nil
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
		return fmt.Errorf("ht: illegal negative MinLifetime")
	}

	c.Type = strings.ToLower(c.Type)
	x := strings.Replace(c.Type, ",", " ", -1)
	for _, t := range strings.Split("session persistent secure unsafe httponly exposed", " ") {
		x = strings.TrimSpace(strings.Replace(x, t, "", -1))
	}
	if x != "" {
		return fmt.Errorf("ht: unknown stuff in cookie type %q", x)
	}

	if c.MinLifetime > 0 {
		if !strings.Contains(c.Type, "persistent") {
			c.Type += " persistent"
		}
	}

	return nil
}

// ----------------------------------------------------------------------------
// DeleteCookie

// DeleteCookie checks that the HTTP response properly deletes all cookies
// matching Name, Path and Domain. Path and Domain are optional in which case
// all cookies with the given Name are checked for deletion.
type DeleteCookie struct {
	Name   string
	Path   string `json:",omitempty"`
	Domain string `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (c DeleteCookie) Execute(t *Test) error {
	errors := []string{}
	deleted := false
	for _, cookie := range findCookiesByName(t, c.Name) {
		// Work only on cookies matching the optional Path and Domain.
		if c.Path != "" && cookie.Path != c.Path {
			continue
		}
		if c.Domain != "" && cookie.Domain != c.Domain {
			continue
		}

		// Check for deletion.
		if cookie.MaxAge == 0 && cookie.Expires.IsZero() {
			em := fmt.Sprintf("cookie (%s;%s;%s) not deleted",
				c.Name, cookie.Path, cookie.Domain)
			errors = append(errors, em)
			continue
		}
		if cookie.MaxAge > 0 {
			em := fmt.Sprintf("cookie (%s;%s;%s) not deleted,MaxAge=%d",
				c.Name, cookie.Path, cookie.Domain, cookie.MaxAge)
			errors = append(errors, em)
			continue
		}
		if !cookie.Expires.IsZero() {
			// Deletion via Expires value is problematic due to
			// clock variations. Play it safe and require at least
			// 90 seconds backdated Expires.
			now := time.Now().Add(-90 * time.Second)
			if cookie.Expires.After(now) {
				em := fmt.Sprintf("cookie (%s;%s;%s) not deleted, Expires=%s",
					c.Name, cookie.Path, cookie.Domain, cookie.RawExpires)
				errors = append(errors, em)
				continue
			}
		}
		// Here not both of MaxAge and Expires are unset
		// and none is positive, so at least one is negative.
		// Thus the cookie is properly deleted.

		deleted = true
	}

	if !deleted {
		errors = append(errors, fmt.Sprintf("No cookie (%s;%s;%s) was deleted",
			c.Name, c.Path, c.Domain))
	}

	if len(errors) != 0 {
		return fmt.Errorf("%s", strings.Join(errors, "\n"))
	}
	return nil
}
