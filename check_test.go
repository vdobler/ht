// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	_ "image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/vdobler/ht/check"
)

func TestStatusCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	for _, code := range []int{200, 201, 204, 300, 400, 404, 500} {
		s := fmt.Sprintf("%d", code)
		test := Test{
			Request: Request{
				Method:          "GET",
				URL:             ts.URL + "/",
				Params:          url.Values{"status": []string{s}},
				FollowRedirects: false,
			},
			Checks: []check.Check{
				check.StatusCode{Expect: code},
			},
		}

		result := test.Run(nil)
		if result.Status != Pass {
			t.Errorf("Unexpected error for %d: %s", code, result.Error)
		}
	}
}
