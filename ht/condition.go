// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"regexp"
	"strings"
)

// Condition is a conjunction of tests against a string. Note that Contains and
// Regexp conditions both use the same Count; most likely one would use either
// Contains or Regexp but not both.
type Condition struct {
	// Equals is the exact value to be expected.
	// No other tests are performed if Equals is non-zero as these
	// other tests would be redundant.
	Equals string `json:",omitempty"`

	// Prefix is the required prefix
	Prefix string `json:",omitempty"`

	// Suffix is the required suffix.
	Suffix string `json:",omitempty"`

	// Contains must be contained in the string.
	Contains string `json:",omitempty"`

	// Regexp is a regular expression to look for.
	Regexp string `json:",omitempty"`

	// Count determines how many occurences of Contains or Regexp
	// are required for a match:
	//     0: Any positive number of matches is okay
	//   > 0: Exactly that many matches required
	//   < 0: No match allowed (invert the condition)
	Count int `json:",omitempty"`

	// Min and Max are the minimum and maximum length the string may
	// have. Two zero values disables this test.
	Min, Max int `json:",omitempty"`

	re *regexp.Regexp
}

// Fullfilled returns whether s matches all requirements of c.
// A nil return value indicates that s matches the defined conditions.
// A non-nil return indicates missmatch.
func (c Condition) Fullfilled(s string) error {
	if c.Equals != "" {
		if s == c.Equals {
			return nil
		}
		ls, le := len(s), len(c.Equals)
		if ls <= (15*le)/10 {
			// Show full value if not 50% longer.
			return fmt.Errorf("Unequal, was %q", s)
		}
		// Show 10 more characters
		end := le + 10
		if end > ls {
			end = ls
			return fmt.Errorf("Unequal, was %q", s)
		}
		return fmt.Errorf("Unequal, was %q...", s[:end])
	}

	if c.Prefix != "" && !strings.HasPrefix(s, c.Prefix) {
		n := len(c.Prefix)
		if len(s) < n {
			n = len(s)
		}
		return fmt.Errorf("Bad prefix, got %q", s[:n])
	}

	if c.Suffix != "" && !strings.HasSuffix(s, c.Suffix) {
		n := len(c.Suffix)
		if len(s) < n {
			n = len(s)
		}
		return fmt.Errorf("Bad suffix, got %q", s[len(s)-n:])
	}

	if c.Contains != "" {
		if c.Count == 0 && strings.Index(s, c.Contains) == -1 {
			return ErrNotFound
		} else if c.Count < 0 && strings.Index(s, c.Contains) != -1 {
			return ErrFoundForbidden
		} else if c.Count > 0 {
			if cnt := strings.Count(s, c.Contains); cnt != c.Count {
				return WrongCount{Got: cnt, Want: c.Count}
			}
		}
	}

	if c.Regexp != "" && c.re == nil {
		c.re = regexp.MustCompile(c.Regexp)
	}

	if c.re != nil {
		if c.Count == 0 && c.re.FindStringIndex(s) == nil {
			return ErrNotFound
		} else if c.Count < 0 && c.re.FindStringIndex(s) != nil {
			return ErrFoundForbidden
		} else if c.Count > 0 {
			if m := c.re.FindAllString(s, -1); len(m) != c.Count {
				return WrongCount{Got: len(m), Want: c.Count}
			}
		}
	}

	if c.Min > 0 {
		if len(s) < c.Min {
			return fmt.Errorf("Too short, was %d", len(s))
		}
	}

	if c.Max > 0 {
		if len(s) > c.Max {
			return fmt.Errorf("Too long, was %d", len(s))
		}
	}

	return nil
}

// FullfilledBytes provides a optimized version for Fullfilled(string(byteSlice)).
// TODO: Make this a non-lie.
func (c Condition) FullfilledBytes(b []byte) error {
	return c.Fullfilled(string(b))
}

// Compile pre-compiles the regular expression if part of c.
func (c *Condition) Compile() (err error) {
	if c.Regexp != "" {
		c.re, err = regexp.Compile(c.Regexp)
		if err != nil {
			c.re = nil
			return MalformedCheck{Err: err}
		}
	}
	return nil
}
