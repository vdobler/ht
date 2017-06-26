// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import "fmt"

func init() {
	RegisterCheck(AnyOne{})
	RegisterCheck(None{})
}

// Boolean combinations of Checks

// AnyOne checks that at least one Of the embedded checks passes.
// It is the short circuiting boolean OR of the underlying checks.
// Check execution stops once the first passing check is found.
// Example (in JSON5 notation) to check status code for '202 OR 404':
//     {
//         Check: "AnyOne", Of: [
//             {Check: "StatusCode", Expect: 202},
//             {Check: "StatusCode", Expect: 404},
//         ]
//     }
type AnyOne struct {
	// Of is the list of checks to execute.
	Of CheckList
}

// Prepare implements Checks' Prepare method by forwarding to
// the underlying checks.
func (a AnyOne) Prepare(t *Test) error {
	errs := ErrorList{}
	for _, c := range a.Of {
		if prep, ok := c.(Preparable); ok {
			errs = errs.Append(prep.Prepare(t))
		}
	}
	return errs.AsError()
}

var _ Preparable = AnyOne{}

// Execute implements Check's Execute method. It executes the underlying checks
// until the first passes. If all underlying checks fail the whole list of
// failures is returned.
func (a AnyOne) Execute(t *Test) error {
	errs := ErrorList{}
	for _, c := range a.Of {
		err := c.Execute(t)
		if err == nil {
			return nil
		}
		errs = errs.Append(err)
	}
	return errs.AsError()
}

// None checks that none Of the embedded checks passes.
// It is the NOT of the short circuiting boolean AND of the underlying checks.
// Check execution stops once the first passing check is found.
// It
// Example (in JSON5 notation) to check for non-occurrence of 'foo' in body:
//     {
//         Check: "None", Of: [
//             {Check: "Body", Contains: "foo"},
//         ]
//     }
type None struct {
	// Of is the list of checks to execute.
	Of CheckList
}

// Prepare implements Checks' Prepare method by forwarding to
// the underlying checks.
func (n None) Prepare(t *Test) error {
	errs := ErrorList{}
	for _, c := range n.Of {
		if prep, ok := c.(Preparable); ok {
			errs = errs.Append(prep.Prepare(t))
		}
	}
	return errs.AsError()
}

var _ Preparable = None{}

// Execute implements Check's Execute method. It executes the underlying checks
// until the first passes. If all underlying checks fail the whole list of
// failures is returned.
func (n None) Execute(t *Test) error {
	for i, c := range n.Of {
		if err := c.Execute(t); err == nil {
			return fmt.Errorf("Check %d passed", i+1)
		}
	}
	return nil
}
