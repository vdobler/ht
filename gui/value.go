// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gui

import (
	"bytes"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"

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

// PushCurrent stores the Current value in v to the list of Last
// values. This allows to checkpoint the state of v for subsequent
// undoes to one of the Pushed states.
func (v *Value) PushCurrent() {
	v.Last = append(v.Last, v.Current)
}

// Update v with data from the received HTML form. It returns the path of the
// most prominent field (TODO: explain better).
func (v *Value) Update(form url.Values) (string, errorlist.List) {
	val := reflect.ValueOf(v.Current)
	v.Messages = make(map[string][]Message) // clear errors // TODO: really automaticall here?
	firstErrorPath := ""

	updated, err := walk(form, v.Path, val)

	// Process validation errors
	for _, e := range err {
		switch ve := e.(type) {
		case ValueError:
			if firstErrorPath == "" {
				firstErrorPath = ve.Path
			}
			v.Messages[ve.Path] = []Message{{
				Type: "error",
				Text: ve.Err.Error(),
			}}
		case addNoticeError:
			firstErrorPath = string(ve)
		}
	}

	v.Current = updated.Interface()

	return firstErrorPath, err
}

// BinaryData returns the string or byte slice addressed by path.
func (v *Value) BinaryData(path string) ([]byte, error) {
	if !strings.HasPrefix(path, v.Path) {
		return nil, fmt.Errorf("gui: bad path") // TODO error message
	}

	path = path[len(v.Path)+1:]
	return binData(path, reflect.ValueOf(v.Current))
}

func binData(path string, val reflect.Value) ([]byte, error) {
	if path == "" {
		switch val.Kind() {
		case reflect.String:
			return []byte(val.String()), nil
		case reflect.Slice:
			elKind := val.Type().Elem().Kind()
			if elKind == reflect.Uint8 {
				return val.Bytes(), nil
			}
			return nil, fmt.Errorf("gui: bin data of %s slice", elKind.String())
		}
	}
	part := strings.SplitN(path, ".", 2)
	if len(part) == 1 {
		part = append(part, "")
	}
	switch val.Kind() {
	case reflect.Struct:
		field := val.FieldByName(part[0])
		if !field.IsValid() {
			return nil, fmt.Errorf("gui: no such field %s", part[0])
		}
		return binData(part[1], field)
	case reflect.Map:
		name := demangleKey(part[0])
		key := reflect.ValueOf(name)
		// TODO handel maps index by other stuff han strings
		v := val.MapIndex(key)
		if !v.IsValid() {
			return nil, fmt.Errorf("gui: no such key %s", part[0])
		}

		return binData(part[1], v)
	case reflect.Slice:
		i, err := strconv.Atoi(part[0])
		if err != nil {
			return nil, fmt.Errorf("gui: bad index: %s", err)
		}
		if i < 0 || i > val.Len() {
			return nil, fmt.Errorf("gui: index %d out of range", i)
		}
		return binData(part[1], val.Index(i))
	case reflect.Interface, reflect.Ptr:
		if val.IsNil() {
			return nil, fmt.Errorf("gui: cannot traverse nil ptr/interface")
		}
		return binData(part[1], val.Elem())
	}
	return nil, fmt.Errorf("gui: cannot traverse %s in %s", val.Kind().String(), path)
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
