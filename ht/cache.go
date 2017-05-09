// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// cache.go provides check around caching

package ht

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func init() {
	RegisterCheck(ETag{})
	RegisterCheck(Cache{})
}

var errBadMethod = errors.New("check is useful for GET requests only")

// ----------------------------------------------------------------------------
// ETag

var (
	errNoETag        = errors.New("missing ETag header")
	errUnquotedETag  = errors.New("ETag value not quoted")
	errEmptyETag     = errors.New("empty ETag value")
	errMultipleETags = errors.New("multiple ETag headers")
	errETagIgnored   = errors.New("ETag not honoured")
)

// ETag checks presence of a (stron) ETag header and that a subsequent request with a
// If-None-Match header results in a 304 Not Modified response.
type ETag struct{}

// Execute implements Check's Execute method.
func (ETag) Execute(t *Test) error {
	if t.Request.Method != "" && t.Request.Method != http.MethodGet {
		return errBadMethod
	}

	values := etags(t.Response.Response.Header)
	if len(values) == 0 {
		return errNoETag
	} else if len(values) > 1 {
		return errMultipleETags
	}
	val := values[0]
	if len(val) < 3 {
		return errEmptyETag
	}
	if val[0] != '"' || val[len(val)-1] != '"' {
		return errUnquotedETag
	}
	// Okay, val is of the form "123-a".

	second, err := Merge(t) // Second is a copy of the original t.
	if err != nil {
		return err
	}
	second.Request.Header.Set("If-None-Match", val)
	second.Checks = CheckList{
		&StatusCode{Expect: 304},
	}

	second.Run()
	if second.Status == Fail {
		return errETagIgnored
	}
	return second.Error
}

// Prepare implements Check's Prepare method.
func (ETag) Prepare() error {
	return nil
}

// etags returns "ETag" and "Etag" headers from h. There must be an other solution.
func etags(h http.Header) []string {
	tags := []string{}
	tags = append(tags, h["ETag"]...)
	tags = append(tags, h["Etag"]...)
	return tags
}

// ----------------------------------------------------------------------------
// Cache

var (
	errCacheControlMissing = errors.New("missing Cache-Control header")
	errIllegalCacheControl = errors.New("no-store illegaly combined with no-cache")
	errMissingNoStore      = errors.New("missing no-store")
	errMissingNoCache      = errors.New("missing no-cache")
	errMissingMaxAge       = errors.New("missing max-age")
	errMissingMaxAgeValue  = errors.New("missing max-age value")
)

// Cache allows to test for HTTP Cache-Control headers. The zero value checks
// for the existence of a Cache-Control header only.
// Note that not all combinations are sensible.
type Cache struct {
	// NoStore checks for no-store.
	NoStore bool

	// NoCache checks for no-cache.
	NoCache bool

	// AtLeast checks for the presence of a max-age value at least as long.
	AtLeast time.Duration

	// AtMost checks for the presence of a max-age value at most as long.
	AtMost time.Duration
}

// Execute implements Check's Execute method.
func (c Cache) Execute(t *Test) error {
	if t.Request.Method != "" && t.Request.Method != http.MethodGet {
		// Maybe this is too harsh, HEAD requests might allow Cache-Control
		// but that's probably too much nitpicking: If one wants to check this
		// one can use the Header check directly.
		return errBadMethod
	}

	cc := t.Response.Response.Header.Get("Cache-Control")
	if cc == "" {
		return errCacheControlMissing
	}

	nostore, nocache := strings.Contains(cc, "no-store"), strings.Contains(cc, "no-cache")

	// Sanity check first.
	if nostore && nocache {
		return errIllegalCacheControl
	}

	if c.NoStore && !nostore {
		return errMissingNoStore
	}
	if c.NoCache && !nocache {
		return errMissingNoCache
	}

	if c.AtMost != 0 || c.AtLeast != 0 {
		i := strings.Index(cc, "max-age=")
		if i == -1 {
			return errMissingMaxAge
		}
		ma := cc[i+len("max-age="):]
		digits := len(ma) - len(strings.TrimLeft(ma, "0123456789"))
		ma = ma[:digits]
		seconds, err := strconv.Atoi(ma)
		if err != nil {
			return errMissingMaxAgeValue
		}
		got := time.Duration(seconds) * time.Second

		if c.AtMost != 0 && got > c.AtMost {
			return fmt.Errorf("got max-age of %s, want at most %s",
				got, c.AtMost)
		}
		if c.AtLeast != 0 && got < c.AtLeast {
			return fmt.Errorf("got max-age of %s, want at least %s",
				got, c.AtLeast)
		}
	}

	return nil
}

var (
	errNoStoreNoCache   = errors.New("no-store canot be combined with no-cache or max-age")
	errEmptyMaxAgeRange = errors.New("AtMost must be larger or equal than AtLeast")
)

// Prepare implements Check's Prepare method.
func (c Cache) Prepare() error {
	if c.NoStore && (c.NoCache || c.AtLeast != 0 || c.AtMost != 0) {
		return errNoStoreNoCache
	}

	if c.AtLeast != 0 && c.AtMost != 0 && c.AtMost < c.AtLeast {
		return errEmptyMaxAgeRange
	}

	return nil
}
