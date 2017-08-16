// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gui provides tool for a HTML based modificator for Go values.
//
// Package gui contains code to render an arbitrary Go value as a HTML form
// and update the value from the submitted form data. The generated HTML
// can include documentation in the form of tooltips.
//
// Global State
//
// To render and update a Go value to/from a HTML form package gui needs
// information about the types like documentation of types and fields,
// which type satisfies which interface and which struct fields should not
// be rendered. This information is kept in package scoped variables
// which is ugly but easy:
//
//
//
package gui
