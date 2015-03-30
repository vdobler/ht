// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// time.go contains checks against the response time

package check

import (
	"fmt"
	"time"

	"github.com/vdobler/ht/response"
)

func init() {
	RegisterCheck(ResponseTime{})
}

// ----------------------------------------------------------------------------
// ResponseTime

// ResponseTime checks the response time.
type ResponseTime struct {
	Lower  time.Duration `xml:",attr,omitempty"`
	Higher time.Duration `xml:",attr,omitempty"`
}

func (c ResponseTime) Okay(response *response.Response) error {
	if c.Higher != 0 && c.Lower != 0 && c.Higher >= c.Lower {
		return MalformedCheck{Err: fmt.Errorf("%d<RT<%d unfullfillable", c.Higher, c.Lower)}
	}
	if c.Lower > 0 && c.Lower < response.Duration {
		return fmt.Errorf("Response took %s (allowed max %s).",
			response.Duration.String(), c.Lower.String())
	}
	if c.Higher > 0 && c.Higher > response.Duration {
		return fmt.Errorf("Response took %s (required min %s).",
			response.Duration.String(), c.Higher.String())
	}
	return nil
}
