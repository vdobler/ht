// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
		want := ""
		if tc.err != nil {
			want = "Check ETag: " + tc.err.Error()
		}
		got := ""
		if test.Error != nil {
			got = test.Error.Error()
		}

		if got != want {
			t.Errorf("%d %s: got error %v, want %v", i, tc.path, got, want)
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

func makeCCResp(s string) Response {
	return Response{
		Response: &http.Response{
			Header: http.Header{
				"Cache-Control": []string{s},
			},
		},
	}
}

var cacheTests = []TC{
	{Response{Response: &http.Response{Header: http.Header{}}},
		Cache{}, errCacheControlMissing},
	{makeCCResp("no-store"), Cache{NoStore: true}, nil},
	{makeCCResp("no-cache"), Cache{NoCache: true}, nil},
	{makeCCResp("private"), Cache{Private: true}, nil},
	{makeCCResp("no-cache, no-store"), Cache{}, errIllegalCacheControl},
	{makeCCResp("no-cache"), Cache{Private: true}, errMissingPrivate},
	{makeCCResp("no-cache"), Cache{NoStore: true}, errMissingNoStore},
	{makeCCResp("no-store"), Cache{NoCache: true}, errMissingNoCache},
	{makeCCResp("no-store"), Cache{AtLeast: 3 * time.Minute}, errMissingMaxAge},
	{makeCCResp("no-store"), Cache{AtMost: 3 * time.Minute}, errMissingMaxAge},
	{makeCCResp("no-cache, max-age="), Cache{AtLeast: time.Minute}, errMissingMaxAge},
	{makeCCResp("please-no-cache"), Cache{NoCache: true}, errMissingNoCache},
	{makeCCResp("no-cacheing"), Cache{NoCache: true}, errMissingNoCache},

	{makeCCResp("max-age=123"), Cache{AtLeast: 100 * time.Second}, nil},
	{makeCCResp("max-age=123"), Cache{AtMost: 130 * time.Second}, nil},
	{makeCCResp("max-age=123"), Cache{AtMost: 130 * time.Second}, nil},
	{makeCCResp("max-age=90"),
		Cache{AtMost: 90 * time.Second, AtLeast: 90 * time.Second}, nil},
	{makeCCResp("max-age=123"), Cache{AtMost: 1 * time.Minute}, errCheck},
	{makeCCResp("max-age=123"), Cache{AtLeast: 3 * time.Minute}, errCheck},

	{makeCCResp("max-age=90, no-cache"), Cache{AtLeast: time.Minute}, nil},
	{makeCCResp(" max-age=90 , no-cache , something"),
		Cache{AtLeast: time.Minute, NoCache: true}, nil},
	{makeCCResp("no-cache, max-age=90"), Cache{AtLeast: time.Minute}, nil},
	{makeCCResp("max-age=abc, no-cache"), Cache{AtLeast: time.Minute}, errCheck},
}

func TestCache(t *testing.T) {
	for i, tc := range cacheTests {
		runTest(t, i, tc)
	}
}
