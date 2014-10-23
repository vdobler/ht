// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package response provides a type for captuiring a HTTP response. Its main
// purpose is breaking an import cycle between ht and ht/check.
package response

import (
	"bytes"
	"net/http"
	"time"
)

// Response captures information about a http response.
type Response struct {
	// Response is the received HTTP response. Its body has bean read and
	// closed allready.
	Response *http.Response

	// Duration to receive response and read the whole body.
	Duration time.Duration

	// The received body and the error got while reading it.
	Body    []byte
	BodyErr error
}

// BodyReader returns a reader of the response body.
func (resp *Response) BodyReader() *bytes.Reader {
	return bytes.NewReader(resp.Body)
}
