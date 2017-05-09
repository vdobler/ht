// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var wrongETagTests = []TC{
	{Response{Response: &http.Response{Header: http.Header{}}},
		ETag{}, errNoETag},
	{Response{Response: &http.Response{Header: http.Header{
		"ETag": []string{`blank1`}}}},
		ETag{}, errUnquotedETag},
	{Response{Response: &http.Response{Header: http.Header{
		"Etag": []string{`blank2`}}}},
		ETag{}, errUnquotedETag},
	{Response{Response: &http.Response{Header: http.Header{
		"ETag": []string{`""`}}}},
		ETag{}, errEmptyETag},
	{Response{Response: &http.Response{Header: http.Header{
		"Etag": []string{`""`}}}},
		ETag{}, errEmptyETag},
	{Response{Response: &http.Response{Header: http.Header{
		"ETag": []string{`"m1"`, `"m2"`}}}},
		ETag{}, errMultipleETags},
	{Response{Response: &http.Response{Header: http.Header{
		"Etag": []string{`"m1"`, `"m2"`}}}},
		ETag{}, errMultipleETags},
}

func TestETagMissing(t *testing.T) {
	for i, tc := range wrongETagTests {
		runTest(t, i, tc)
	}
}

var etagTests = []struct {
	path string
	err  error
}{
	{"/notag", errNoETag},
	{"/tagonly", errETagIgnored},
	{"/okay", nil},
}

func TestETag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(etagHandler))
	defer ts.Close()

	for i, tc := range etagTests {
		test := Test{
			Name: "Original Test.",
			Request: Request{
				Method: "GET",
				URL:    ts.URL + tc.path,
			},
			Checks: []Check{
				ETag{},
			},
		}

		test.Run()
		if test.Error != tc.err {
			t.Errorf("%d %s: got error %v, want %v", i, tc.path,
				test.Error, tc.err)
		}
	}
}

// echoHandler answers the request based on the parameters status (HTTP status
// code), text (the response body) and header and value (any HTTP header).
// The handler sleeps for a random duration between smin and smax milliseconds.
// If echoHandler is called with parameter fail it timeout with the given
// probability. The parameter bad controls the probability if bad responses.
func etagHandler(w http.ResponseWriter, r *http.Request) {
	tag := `"halleluja-12345"`
	path := r.URL.Path
	switch path {
	case "/notag":
		http.Error(w, "Okay", http.StatusOK)
	case "/tagonly":

		w.Header()["ETag"] = []string{tag}
		http.Error(w, "Okay", http.StatusOK)
	case "/okay":
		if r.Header.Get("If-None-Match") == tag {
			w.WriteHeader(304)
		} else {
			w.Header()["ETag"] = []string{tag}
			http.Error(w, "You are welcome!", http.StatusOK)
		}
	default:
		http.Error(w, "Unknow path "+path, http.StatusBadRequest)
	}
}
