// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gui

import (
	"fmt"
	"reflect"
	"testing"
)

func TestRegisterImplementation(t *testing.T) {
	Implements = make(map[reflect.Type][]reflect.Type)
	if got := fmt.Sprintln(Implements); got != "map[]\n" {
		t.Fatal(got)
	}
	RegisterImplementation((*Writer)(nil), AbcWriter{})
	if got := fmt.Sprintln(Implements); got != "map[gui.Writer:[gui.AbcWriter]]\n" {
		t.Fatal(got)
	}
}
