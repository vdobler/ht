// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
)

func init() {
	RegisterCheck(&Logfile{})
}

// ----------------------------------------------------------------------------
// Logfile

// Logfile provides checks on files (i.e. it ignores the response).
// During preparation the current file size is determined and the checks
// are run against the bytes written after preparation.
type Logfile struct {
	// Path is the file system path to the logfile."
	Path string

	// Condition the written stuff must fulfill.
	Condition `json:",omitempty"`

	// Disallow states what is forbidden in the written log.
	Disallow []string `json:",omitempty"`

	pos int64
}

// Execute implements Check's Execute method.
func (f *Logfile) Execute(t *Test) error {
	file, err := os.Open(f.Path)
	if err != nil {
		return err
	}
	pos, err := file.Seek(f.pos, os.SEEK_SET)
	if err != nil {
		return err
	}

	if pos != f.pos {
		return fmt.Errorf("ht: cannot seek to %d in file %s, got to %d", f.pos, f.Path, pos)
	}
	written, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	if err := f.FullfilledBytes(written); err != nil {
		return err
	}
	for _, disallow := range f.Disallow {
		if bytes.Contains(written, []byte(disallow)) {
			return fmt.Errorf("found forbidden %q", disallow)
		}
	}
	return nil
}

// Prepare implements Check's Prepare method.
func (f *Logfile) Prepare() error {
	file, err := os.Open(f.Path)
	if err != nil {
		f.pos = 0
		return nil
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	f.pos = stat.Size()
	return nil
}
