// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// time.go contains checks against the response time

package check

import (
	"fmt"

	"github.com/vdobler/ht/response"
)

func init() {
	RegisterCheck(ResponseTime{})
}

// ----------------------------------------------------------------------------
// ResponseTime

// ResponseTime checks the response time.
type ResponseTime struct {
	Lower  response.Duration `json:",omitempty"`
	Higher response.Duration `json:",omitempty"`
}

func (c ResponseTime) Execute(r *response.Response) error {
	actual := response.Duration(r.Duration)
	if c.Higher != 0 && c.Lower != 0 && c.Higher >= c.Lower {
		return MalformedCheck{Err: fmt.Errorf("%d<RT<%d unfullfillable", c.Higher, c.Lower)}
	}
	if c.Lower > 0 && c.Lower < actual {
		return fmt.Errorf("Response took %s (allowed max %s).",
			actual.String(), c.Lower.String())
	}
	if c.Higher > 0 && c.Higher > actual {
		return fmt.Errorf("Response took %s (required min %s).",
			actual.String(), c.Higher.String())
	}
	return nil
}

func (_ ResponseTime) Prepare() error { return nil }
