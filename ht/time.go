// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// time.go contains checks against the response time

package ht

import (
	"fmt"
	"time"
)

func init() {
	RegisterCheck(ResponseTime{})
}

// ----------------------------------------------------------------------------
// ResponseTime

// ResponseTime checks the response time.
type ResponseTime struct {
	Lower  time.Duration `json:",omitempty"`
	Higher time.Duration `json:",omitempty"`
}

// Execute implements Check's Execute method.
// TODO: fix spelling of unfullfillable.
func (c ResponseTime) Execute(t *Test) error {
	// TODO: remove as checks may rely on beeing pre-prepared
	if err := c.Prepare(t); err != nil {
		return MalformedCheck{err}
	}

	actual := t.Response.Duration
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

// Prepare implements Preparable.Prepare.
func (c ResponseTime) Prepare(*Test) error {
	if c.Higher != 0 && c.Lower != 0 && c.Higher >= c.Lower {
		return fmt.Errorf("%d<RT<%d unfullfillable", c.Higher, c.Lower)
	}
	return nil
}

var _ Preparable = ResponseTime{}
