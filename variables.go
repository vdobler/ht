// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vdobler/ht/check"
)

// Repeat returns count copies of test with variables replaced based
// on vars. The keys of vars are the variable names. The values of a
// variable v are choosen from vars[v] by cycling through the list:
// In the n'th repetition is vars[v][n%N] with N=len(vars[v])).
func Repeat(test *Test, count int, vars map[string][]string) []*Test {
	reps := make([]*Test, count)
	for r := 0; r < count; r++ {
		// Construct appropriate Replacer from variables.
		oldnew := make([]string, 0, 2*len(vars))
		for k, v := range vars {
			oldnew = append(oldnew, "{{"+k+"}}") // TODO: make configurable ??
			n := r % len(v)
			oldnew = append(oldnew, v[n])
		}
		replacer := strings.NewReplacer(oldnew...)
		reps[r] = test.substituteVariables(replacer)
	}
	return reps
}

// lcm computest the least common multiple of m and n.
func lcm(m, n int) int {
	a, b := m, n
	for a != b {
		if a < b {
			a += m
		} else {
			b += n
		}
	}
	return a
}

// lcmOf computes the least common multiple of the length of all valuesin vars.
func lcmOf(vars map[string][]string) int {
	n := 0
	for _, v := range vars {
		if n == 0 {
			n = len(v)
		} else {
			n = lcm(n, len(v))
		}
	}
	return n
}

// substituteVariables returns a copy of t with replacer applied.
func (t *Test) substituteVariables(replacer *strings.Replacer) *Test {
	// Apply to name, description, URL and body.
	c := &Test{
		Name:        replacer.Replace(t.Name),
		Description: replacer.Replace(t.Description),
		Request: Request{
			URL:  replacer.Replace(t.Request.URL),
			Body: replacer.Replace(t.Request.Body),
		},
	}

	// Apply to request parameters.
	c.Request.Params = make(url.Values)
	for param, vals := range t.Request.Params {
		rv := make([]string, len(vals))
		for i, v := range vals {
			rv[i] = replacer.Replace(v)
		}
		c.Request.Params[param] = rv
	}

	// Apply to http header.
	c.Request.Header = make(http.Header)
	for h, vals := range t.Request.Header {
		rv := make([]string, len(vals))
		for i, v := range vals {
			rv[i] = replacer.Replace(v)
		}
		c.Request.Header[h] = rv
	}

	// Apply to cookie values.
	for _, cookie := range t.Request.Cookies {
		rc := Cookie{Name: cookie.Name, Value: replacer.Replace(cookie.Value)}
		c.Request.Cookies = append(c.Request.Cookies, rc)
	}

	// Apply to checks.
	c.Checks = make([]check.Check, len(t.Checks))
	for i := range t.Checks {
		c.Checks[i] = check.SubstituteVariables(t.Checks[i], replacer)
	}

	return c
}

// ----------------------------------------------------------------------------
// Variable substitutions

var nowTimeRe = regexp.MustCompile(`{{NOW *([+-] *[1-9][0-9]*[smhd])? *(\| *"(.*)")?}}`)

// findNowVariables return all occurences of a time-variable.
func (t *Test) findNowVariables() (v []string) {
	add := func(s string) {
		m := nowTimeRe.FindAllString(s, 1)
		if m == nil {
			return
		}
		v = append(v, m[0])
	}

	add(t.Name)
	add(t.Description)
	add(t.Request.URL)
	add(t.Request.Body)
	for _, pp := range t.Request.Params {
		for _, p := range pp {
			add(p)
		}
	}
	for _, hh := range t.Request.Header {
		for _, h := range hh {
			add(h)
		}
	}
	for _, cookie := range t.Request.Cookies {
		add(cookie.Value)
	}
	for _, c := range t.Checks {
		v = append(v, findNV(c)...)
	}
	return v
}

// nowVariables looks through t, extracts all occurences of now variables, i.e.
//     {{NOW + 30s | "2006-Jan-02"}}
// and formats the desired time. It returns a map suitable for merging with
// other, real variable/value-Pairs.
func (t *Test) nowVariables(now time.Time) (vars map[string]string) {
	nv := t.findNowVariables()
	vars = make(map[string]string)
	for _, k := range nv {
		m := nowTimeRe.FindAllStringSubmatch(k, 1)
		if m == nil {
			panic("Unmatchable " + k)
		}
		kk := k[2 : len(k)-2] // Remove {{ and }} to produce the "variable name".
		if _, ok := vars[kk]; ok {
			continue // We already processed this variable.
		}
		var off time.Duration
		delta := m[0][1]
		if delta != "" {
			num := strings.TrimLeft(delta[1:len(delta)-1], " ")
			n, err := strconv.Atoi(num)
			if err != nil {
				panic(err)
			}
			if delta[0] == '-' {
				n *= -1
			}
			switch delta[len(delta)-1] {
			case 'm':
				n *= 60
			case 'h':
				n *= 60 * 60
			case 'd':
				n *= 24 * 26 * 60
			}
			off = time.Duration(n) * time.Second
		}
		format := time.RFC1123
		if m[0][3] != "" {
			format = m[0][3]
		}
		formatedTime := now.Add(off).Format(format)
		vars[kk] = formatedTime
	}
	return vars
}

// MergeVariables merges all variables found in vars.
func MergeVariables(vars ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, e := range vars {
		for k, v := range e {
			result[k] = v
		}
	}
	return result
}

// NewReplacer produces a replacer to perform substitution of the
// given variables with their values. The keys of vars are the variable
// names and the replacer subsitutes "{{k}}" with vars[k] for each key
// in vars.
func NewReplacer(vars map[string]string) *strings.Replacer {
	oldnew := make([]string, 0, 2*len(vars))
	for k, v := range vars {
		oldnew = append(oldnew, "{{"+k+"}}") // TODO: make configurable ??
		oldnew = append(oldnew, v)
	}

	replacer := strings.NewReplacer(oldnew...)
	return replacer
}
