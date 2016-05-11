// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// cookie.go provides checks for cookies.

package ht

import (
	"fmt"
	"net/http"
	"net/url"
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

// isProperCookiePath determins whether path is a sensible Path value
// in a Set-Cookie header received from u. E.g. a cookie of the form
//     Set-Cookie: foo=bar; Path=/abc/123
// which is received while accessing
//     http://example.org/some/other/path
// will be rejected by browser because the ath value is not suitable for
// the path in the URL. See RFC 6265 section 5.1.4 for reference.
func isProperCookiePath(u *url.URL, path string) bool {
	if path == "" || u.Path == path {
		return true // Empyt path defaults to actual path, so it is okay.
	}
	if strings.HasPrefix(u.Path, path) {
		if u.Path[len(path)] == '/' || path[len(path)-1] == '/' {
			// "/abc" and "/abc/" both are proper for "/abc/foo" URL path.
			return true
		}
	}
	return false
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
	MinLifetime Duration `json:",omitempty"`

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
		return fmt.Errorf("Bad value: ", err)
	}
	if err := c.Path.Fulfilled(cookie.Path); err != nil {
		return fmt.Errorf("Bad path: ", err)
	}
	if err := c.Domain.Fulfilled(cookie.Domain); err != nil {
		return fmt.Errorf("Bad Domain: ", err)
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
		if strings.Index(c.Type, "persistent") == -1 {
			c.Type += " persistent"
		}
	}

	return nil
}

// ----------------------------------------------------------------------------
// DeleteCookie

// DeleteCookie checks that the HTTP response properly deletes all cookies
// matching Name, Path and Domain. Path and Domain are optional in which case
// all cookies with the given Name are checkd for deletion.
type DeleteCookie struct {
	Name   string
	Path   string `json:",omitempty"`
	Domain string `json:",omitempty"`
}

// Execute implements Check's Execute method.
func (c DeleteCookie) Execute(t *Test) error {
	errors := []string{}
	deleted := false
	u := t.Response.Response.Request.URL
	for _, cookie := range findCookiesByName(t, c.Name) {
		// Report malformed (because of path) cookies.
		if !isProperCookiePath(u, cookie.Path) {
			em := fmt.Sprintf("invalid path %s on cookie %s for URL %s",
				cookie.Path, c.Name, u.String())
			errors = append(errors, em)
			continue
		}
		if c.Path != "" && !isProperCookiePath(u, c.Path) {
			// Well this should be reported during Prepare, not
			// here. Unfortunately Prepare knows nothing about
			// the URL the request will go to. So do it here.
			em := fmt.Sprintf("bogus path %s in check", c.Path)
			errors = append(errors, em)
			continue
		}

		// Work only on cookies matching the optional Path and Domain.
		if c.Path != "" && cookie.Path != c.Path {
			continue
		}
		if c.Domain != "" && cookie.Domain != c.Domain {
			continue
		}

		// Check for deletion.
		if cookie.MaxAge == 0 && cookie.Expires.IsZero() {
			em := fmt.Sprintf("cookie %s;%s;%s not deleted",
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
			// 90 seconds backdatedExpires.
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

		// Sanity check: Do not send values when deleting cookies.
		// It technically works but it is _strange_.
		if cookie.Value != "" {
			em := fmt.Sprintf("cookie (%s;%s;%s) deleted but has value %s",
				c.Name, cookie.Path, cookie.Domain, cookie.Value)
			errors = append(errors, em)
			continue
		}

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

// Prepare implements Check's Prepare method.
func (c *DeleteCookie) Prepare() error {
	return nil
}
