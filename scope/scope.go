// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package scope provides functionality to handele variable scopes.
package scope

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
)

// Variables represents a set of (variable-name, variable-value)-pairs.
type Variables map[string]string

// Replacer to replace {{name}} pattern with the given values from vars.
func (vars Variables) Replacer() *strings.Replacer {
	oldnew := []string{}
	for k, v := range vars {
		oldnew = append(oldnew, "{{"+k+"}}") // TODO: make configurable ??
		oldnew = append(oldnew, v)
	}

	return strings.NewReplacer(oldnew...)
}

// Copy returns a copy of vars.
func (vars Variables) Copy() Variables {
	cpy := make(Variables, len(vars))
	for n, v := range vars {
		cpy[n] = v
	}
	return cpy
}

// New merges outer and inner variables into a new scope.
// Variables defined in the outer scope will be copied to the new scope:
// Variables from the inner scope may not overwrite variables from the
// outer scope. Worded differently: The inner scope provides some kind
// of defauklt which gets overwriten from the outside.
//
// Variable values in the inner scope may reference values from the
// outer scope.
//
// If auto variables are requested the new scope will contain the COUNTER
// and RANDOM variable.
func New(outer, inner Variables, auto bool) Variables {
	// 1. Copy of outer scope
	scope := make(Variables, len(outer)+len(inner)+2)
	for gn, gv := range outer {
		scope[gn] = gv
	}
	if auto {
		scope["COUNTER"] = strconv.Itoa(<-GetCounter)
		scope["RANDOM"] = strconv.Itoa(100000 + RandomIntn(900000))
	}
	replacer := scope.Replacer()

	// 2. Merging inner defaults, allow substitutions from outer scope
	for name, val := range inner {
		if _, ok := scope[name]; ok {
			// Variable name exists in outer scope, do not
			// overwrite with suite defaults.
			continue
		}
		scope[name] = replacer.Replace(val)
	}

	return scope
}

// ----------------------------------------------------------------------------
// Random

// Random is the source for all randomness used in packe scope.
var Random *rand.Rand
var randMux sync.Mutex

func init() {
	Random = rand.New(rand.NewSource(34)) // Seed chosen truly random by Sabine.
}

// RandomIntn returns a random int in the rnage [0,n) read from Random.
// It is safe for concurrent use.
func RandomIntn(n int) int {
	randMux.Lock()
	r := Random.Intn(n)
	randMux.Unlock()
	return r
}

// ----------------------------------------------------------------------------
// Counter

// GetCounter returns a strictly increasing sequence of int values.
var GetCounter <-chan int

var counter int = 1

func init() {
	ch := make(chan int)
	GetCounter = ch
	go func() {
		for {
			ch <- counter
			counter += 1
		}
	}()
}
