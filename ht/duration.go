// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Duration is a time.Duration but has nicer JSON encoding.
type Duration time.Duration

// String returns a human readable representation of d.
func (d Duration) String() string {
	n := int64(d)
	if n <= 180*1e9 {
		return si(n)
	}
	return clock(n / 1e9)
}

var siUnits = map[int64]string{1e3: "µs", 1e6: "ms", 1e9: "s"}

// si formats n nanoseconds to three significant digits. N must be <= 999 seconds.
func si(n int64) string {
	neg := ""
	if n < 0 {
		neg = "-"
		n = -n
	}
	if n <= 999 {
		return fmt.Sprintf("%s%dns", neg, n)
	}
	scale := int64(1000)
	for 10*n/scale > 9995 {
		scale *= 1000
	}
	f := float64(n/(scale/1000)) / 1000
	prec := 2
	if f > 99.95 {
		prec = 0
	} else if f > 9.995 {
		prec = 1
	}
	return fmt.Sprintf("%s%.*f%s", neg, prec, f, siUnits[scale])
}

// clock formats sec as a minutes and seconds
func clock(sec int64) string {
	min := sec / 60
	sec -= min * 60
	return fmt.Sprintf("%dm%02ds", min, sec)
}

// MarshalJSON provides JSON marshaling of d.
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON provides JSON unmarshaling of data.
func (d *Duration) UnmarshalJSON(data []byte) error {
	s := string(data)
	if strings.HasPrefix(s, `"`) {
		if !strings.HasSuffix(s[1:], `"`) {
			return fmt.Errorf("ht: badly quoted duration")
		}
		s = s[1 : len(s)-1]
		pd, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		*d = Duration(pd)
		return nil
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*d = Duration(f * 1e9)

	return nil
}
