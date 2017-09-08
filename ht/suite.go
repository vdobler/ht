// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"sync"

	"github.com/vdobler/ht/cookiejar"
)

// Collection is a set of Test for easy bulk execution
type Collection struct {
	// Test contains the actual tests to execute.
	Tests []*Test

	// Populated during execution
	Status Status
	Error  error
}

// ExecuteConcurrent executes tests concurrently.
// But at most maxConcurrent tests of s are executed concurrently.
func (s *Collection) ExecuteConcurrent(maxConcurrent int, jar *cookiejar.Jar) error {
	s.Status = NotRun
	s.Error = nil
	if maxConcurrent > len(s.Tests) {
		maxConcurrent = len(s.Tests)
	}

	c := make(chan *Test, maxConcurrent)
	wg := sync.WaitGroup{}
	wg.Add(maxConcurrent)
	for i := 0; i < maxConcurrent; i++ {
		go func() {
			defer wg.Done()
			for test := range c {
				test.Run()
			}
		}()
	}
	for _, test := range s.Tests {
		if jar != nil {
			test.Jar = jar
		}
		c <- test
	}
	close(c)
	wg.Wait()

	el := ErrorList{}
	for _, test := range s.Tests {
		if test.Result.Status > s.Status {
			s.Status = test.Result.Status
		}
		if test.Result.Status > Pass {
			el = append(el, test.Result.Error)
		}
	}

	if len(el) > 0 {
		s.Error = el
		return el
	}

	return nil
}
