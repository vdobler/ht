// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// cache.go provides check around caching

package ht

import (
	"errors"
	"fmt"
	"net/http"
)

func init() {
	RegisterCheck(ETag{})
}

// ----------------------------------------------------------------------------
// ETag

var (
	errNoETag        = errors.New("missing ETag header")
	errUnquotedETag  = errors.New("ETag value not quoted")
	errEmptyETag     = errors.New("empty ETag value")
	errMultipleETags = errors.New("multiple ETag headers")
	errETagIgnored   = errors.New("ETag not honoured")
	errETagBadMethod = errors.New("ETag check is useful for GET request only")
)

// ETag checks presence of a (stron) ETag header and that a subsequent request with a
// If-None-Match header results in a 304 Not Modified response.
type ETag struct{}

// Execute implements Check's Execute method.
func (ETag) Execute(t *Test) error {
	if t.Request.Method != "" && t.Request.Method != http.MethodGet {
		fmt.Println(t.Request.Method)
		return errETagBadMethod
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
