// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Merge merges all tests into one. The individual fields are merged in the
// following way:
//
//     Name               Join all names
//     Description        Join all descriptions
//     Request
//         Method         All nonempty must be the same
//         URL            Only one may be nonempty
//         Params         Merge by key
//         ParamsAs       All nonempty must be the same
//         Header         Merge by key
//         Cookies        Merge by cookie name
//         Body           Only one may be nonempty
//         FollowRdr      Last wins
//         Timeout        Use largets
//         Chunked        Last wins
//         Authorization  Only one may be nonempty
//     Checks             Append all checks
//     VarEx              Merge, same keys must have same value
//     TestVars           Use values from first only
//     Execution
//         Tries          Use largest
//         Wait           Use longest
//         PreSleep       Summ of all;  same for InterSleep and PostSleep
//         Verbosity      Use highest
//
// All other fields are zero.
//
// Merging a single Test produces a deep copy of the Test.
func Merge(tests ...*Test) (*Test, error) {
	m := Test{}

	// Name and description
	s := []string{}
	for _, t := range tests {
		s = append(s, t.Name)
	}
	m.Name = "Merge of " + strings.Join(s, ", ")
	s = s[:0]
	for _, t := range tests {
		s = append(s, t.Description)
	}
	m.Description = strings.TrimSpace(strings.Join(s, "\n"))

	// Variables
	m.Variables = make(map[string]string, len(tests[0].Variables))
	for n, v := range tests[0].Variables {
		m.Variables[n] = v
	}

	m.Request.Params = make(url.Values)
	m.Request.Header = make(http.Header)
	m.VarEx = make(map[string]Extractor)
	for _, t := range tests {
		err := mergeRequest(&m.Request, t.Request)
		if err != nil {
			return &m, err
		}
		m.Checks = append(m.Checks, t.Checks...)
		if t.Request.Timeout > m.Request.Timeout {
			m.Request.Timeout = t.Request.Timeout
		}

		// Execution
		if t.Execution.Tries > m.Execution.Tries {
			m.Execution.Tries = t.Execution.Tries
		}
		if t.Execution.Wait > m.Execution.Wait {
			m.Execution.Wait = t.Execution.Wait
		}
		if t.Execution.Verbosity > m.Execution.Verbosity {
			m.Execution.Verbosity = t.Execution.Verbosity
		}
		m.Execution.PreSleep += t.Execution.PreSleep
		m.Execution.InterSleep += t.Execution.InterSleep
		m.Execution.PostSleep += t.Execution.PostSleep

		// VarEx
		for name, value := range t.VarEx {
			if old, ok := m.VarEx[name]; ok && old != value {
				return &m, fmt.Errorf("Won't overwrite extractor for %s", name)
			}
			m.VarEx[name] = value
		}
	}

	return &m, nil
}

// mergeRequest implements the merge strategy described in Merge for the Request.
func mergeRequest(m *Request, r Request) error {
	allNonemptyMustBeSame := func(m *string, s string) error {
		if s != "" {
			if *m != "" && *m != s {
				return fmt.Errorf("Cannot merge %q into %q", s, *m)
			}
			*m = s
		}
		return nil
	}
	onlyOneMayBeNonempty := func(m *string, s string) error {
		if s != "" {
			if *m != "" {
				return fmt.Errorf("Won't overwrite %q with %q", *m, s)
			}
			*m = s
		}
		return nil
	}

	if err := allNonemptyMustBeSame(&(m.Method), r.Method); err != nil {
		return err
	}

	if err := onlyOneMayBeNonempty(&(m.URL), r.URL); err != nil {
		return err
	}

	for k, v := range r.Params {
		m.Params[k] = append(m.Params[k], v...)
	}

	if err := allNonemptyMustBeSame(&(m.ParamsAs), r.ParamsAs); err != nil {
		return err
	}

	for k, v := range r.Header {
		m.Header[k] = append(m.Header[k], v...)
	}

outer:
	for _, rc := range r.Cookies {
		for i := range m.Cookies {
			if m.Cookies[i].Name == rc.Name {
				m.Cookies[i].Value = rc.Value
				continue outer
			}
		}
		m.Cookies = append(m.Cookies, rc)
	}

	if err := onlyOneMayBeNonempty(&(m.Body), r.Body); err != nil {
		return err
	}

	m.FollowRedirects = r.FollowRedirects
	m.Chunked = r.Chunked

	if err := mergeAuthorization(&m.Authorization, r.Authorization); err != nil {
		return err
	}

	return nil
}

func mergeAuthorization(m *Authorization, r Authorization) error {
	mb, rb := m.set(), r.set()
	if mb == 0 && rb == 0 {
		return nil
	}
	if mb != 0 && rb != 0 {
		return fmt.Errorf("Cannot merge two Authorization into one")
	}

	if mb == 0 { // Then rb != 0 because of above.
		*m = r
	}

	return nil
}
