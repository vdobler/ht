// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// error.go contains stuff to combine multiple errors

package ht

import (
	"fmt"
)

// PosError is an error with optinaly attached position information.
type PosError struct {
	Err  error  // Err is the actual error.
	Line int    // Line is the line number counting from 1.
	Col  int    // Col is the column or byte position, also counting from 1!
	File string // Filename is the optional filename.
}

func (e PosError) Error() string {
	s := ""
	if e.File != "" {
		s = e.File + ":"
	}
	if e.Line > 0 {
		if s != "" {
			s += " "
		}
		s += fmt.Sprintf("Line %d:", e.Line)
	}
	if e.Col > 0 {
		if s != "" {
			s += " "
		}
		s += fmt.Sprintf("Detected on column %d:", e.Col)
	}
	if s != "" {
		s += " "
	}
	s += e.Err.Error()
	return s
}
