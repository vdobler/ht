// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package response provides a type for captuiring a HTTP response. Its main
// purpose is breaking an import cycle between ht and ht/check.
package response

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

// ----------------------------------------------------------------------------
// Duration

// Duration is a time.Duration but has nicer JSON encoding.
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	td := time.Duration(d)

	if td < time.Millisecond {
		return []byte(fmt.Sprintf(`"%dµs"`, td/time.Microsecond)), nil
	}
	if td <= 9999*time.Millisecond {
		return []byte(fmt.Sprintf(`"%dms"`, td/time.Millisecond)), nil
	}
	if td <= 60*time.Second {
		return []byte(fmt.Sprintf(`"%.1fs"`, float64(td/time.Millisecond)/1000)), nil
	}
	if td <= 180*time.Second {
		return []byte(fmt.Sprintf(`"%ds"`, td/time.Second)), nil
	}

	min := td / time.Minute
	td -= min * time.Minute
	sec := td / time.Second
	return []byte(fmt.Sprintf(`"%dm%02ds"`, min, sec)), nil
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	s := string(data)
	scale := int64(1e9)
	if strings.HasPrefix(s, `"`) {
		s = s[1 : len(s)-1]
		if strings.HasSuffix(s, "µs") {
			scale = 1e3
			s = s[:len(s)-3]
		} else if strings.HasSuffix(s, "ms") {
			scale = 1e6
			s = s[:len(s)-2]
		} else if strings.HasSuffix(s, "s") {
			scale = 1e9
			s = s[:len(s)-1]
			if i := strings.Index(s, "m"); i != -1 {
				m, err := strconv.Atoi(s[:i])
				if err != nil {
					return err
				}
				sec, err := strconv.Atoi(s[i+1:])
				if err != nil {
					return err
				}
				sec += 60 * m
				*d = Duration(int64(sec) * 1e9)
				return nil
			}
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*d = Duration(f * float64(scale))

	return nil
}
