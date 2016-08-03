// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// otto.go contains checks based on otto, a JavaScript interpreter

package ht

import (
	"errors"

	"github.com/robertkrimen/otto"
	_ "github.com/robertkrimen/otto/underscore"
)

func init() {
	RegisterCheck(&CustomJS{})
}

// ----------------------------------------------------------------------------
// CustomJS

// CustomJS executes the provided JavaScript.
//
// The current Test is present in the JavaScript VM via binding the name
// "Test" at top-level to the current Test beeing checked.
//
// The Script's last value indicates success or failure:
//   - Success: true, 0, ""
//   - Failure: false, any number != 0, any string != ""
//
// CustomJS can be usefull to log an excerpt of response (or the request)
// via console.log.
//
// The JavaScript code is interpreted by otto. See the documentation at
// https://godoc.org/github.com/robertkrimen/otto for details.
type CustomJS struct {
	// Script is JavaScript code to be evaluated.
	//
	// The script may be read from disk with the following syntax:
	//     @file:/path/to/script
	Script string `json:",omitempty"`

	vm     *otto.Otto
	script *otto.Script
}

// Prepare implements Check's Prepare method.
func (s *CustomJS) Prepare() error {
	// Reading the script with fileData is a hack as it accepts
	// the @vfile syntax but cannot do the variable replacements
	// as Prepare is called on the already 'replaced' checked.
	repl, _ := newReplacer(nil)
	script, basename, err := fileData(s.Script, repl)
	if err != nil {
		return err
	}

	if basename == "" {
		basename = "<inline>"
	}

	s.vm = otto.New()
	s.script, err = s.vm.Compile(basename, script)
	return err
}

// Execute implements Check's Execute method.
func (s *CustomJS) Execute(t *Test) error {
	currentTest, err := s.vm.ToValue(*t)
	if err != nil {
		return err
	}
	s.vm.Set("Test", currentTest)
	val, err := s.vm.Run(s.script)
	if err != nil {
		return err
	}

	str, err := val.ToString()
	if err != nil {
		return err
	}
	if str == "0" || str == "true" || str == "" {
		return nil
	}
	return errors.New(str)
}
