// Copyright 2018 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

// A Action is responsible for filling t.Response during the execution
// of a Test.
type Action interface {
	Schema() string        // Schema used to select this action in the URL.
	Valid(t *Test) error   // Valid checks if t can be executed as such a action.
	Execute(t *Test) error // Execute t as this action.
}

var actionRegistry = map[string]Action{
	"http":  httpAction("http"),
	"https": httpAction("https"),
	"file":  fileAction{},
	"bash":  bashAction{},
	"sql":   sqlAction{},
}
