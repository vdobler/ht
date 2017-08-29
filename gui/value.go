// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gui

import (
	"bytes"
	"fmt"
	"net/url"
	"reflect"

	"github.com/vdobler/ht/errorlist"
)

type Message struct {
	Type string
	Text string
}

// ----------------------------------------------------------------------------
// Value

// Value contains a value to be displayed and updated through a HTML GUI.
type Value struct {
	// Current is the current value.
	Current interface{}

	// Last contains the last values
	Last []interface{}

	// Path is the path prefix applied to this value.
	Path string

	// Messages contains messages to be rendered for paths. E.g.:
	//   Test.Request.Timeout => Message{typ:error, txt=wrong}
	Messages map[string][]Message

	buf *bytes.Buffer

	nextfieldinfo Fieldinfo
}

// NewValue creates a new Value from val.
func NewValue(val interface{}, path string) *Value {
	return &Value{
		Current:  val,
		Path:     path,
		Messages: make(map[string][]Message),
		buf:      &bytes.Buffer{},
	}
}

// Render v's Current value. The returned byte slice must neither be modified
// nor kept longer then up to the next call of Render.
func (v *Value) Render() ([]byte, error) {
	val := reflect.ValueOf(v.Current)
	v.buf.Reset()
	err := v.render(v.Path, 0, false, val) // TODO: type based readonly
	return v.buf.Bytes(), err
}

// Update v with data from the received HTML form. It returns the path of the
// most prominent field (TODO: explain better).
func (v *Value) Update(form url.Values) (string, errorlist.List) {
	val := reflect.ValueOf(v.Current)
	v.Messages = make(map[string][]Message) // clear errors // TODO: really automaticall here?
	firstErrorPath := ""

	updated, err := walk(form, v.Path, val)

	if err == nil {
		v.Last = append(v.Last, v.Current)
	} else {
		// Process validation errors
		for _, e := range err {
			if ve, ok := e.(ValueError); ok {
				if firstErrorPath == "" {
					firstErrorPath = ve.Path
				}
				v.Messages[ve.Path] = []Message{{
					Type: "error",
					Text: ve.Err.Error(),
				}}
			}
		}
	}

	v.Current = updated.Interface()

	fmt.Printf("Value.Update(): err=%v <%T>\n", err, err)
	return firstErrorPath, err
}

// ----------------------------------------------------------------------------

// printf to internal buf.
func (v *Value) printf(format string, val ...interface{}) {
	fmt.Fprintf(v.buf, format, val...)
}

// typeinfo returns name and tooltip for val's type.
func (v *Value) typeinfo(val interface{}) (string, string) {
	var typ reflect.Type
	if v, ok := val.(reflect.Value); ok {
		typ = v.Type()
	} else {
		typ = val.(reflect.Type)
	}
	name := typ.Name()
	if name == "" {
		name = "-anonymous-"
	}

	tooltip := "??"
	if info, ok := Typedata[typ]; ok {
		tooltip = info.Doc
	}

	return name, tooltip
}

// fieldinfo returns the name and optional data for field nr i of val.
func (v *Value) fieldinfo(val reflect.Value, i int) (string, Fieldinfo) {
	typ := val.Type()
	name := typ.Field(i).Name

	if tinfo, ok := Typedata[typ]; ok {
		if finfo, ok := tinfo.Field[name]; ok {
			return name, finfo
		}
	}

	return name, Fieldinfo{Doc: "-??-"}
}
