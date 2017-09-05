// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gui

import (
	"reflect"
	"regexp"
)

// ----------------------------------------------------------------------------
// Interface implementations

// Implements is the global lookup of what types implement a given interface
var Implements = make(map[reflect.Type][]reflect.Type)

// RegisterImplementation records that iface is implemented by typ.
// To register that AbcWriter implements the Writer interface use:
//     	RegisterImplementation((*Writer)(nil), AbcWriter{})
func RegisterImplementation(iface interface{}, typ interface{}) {
	ifaceType := reflect.TypeOf(iface).Elem()
	concreteType := reflect.TypeOf(typ)
	for _, impl := range Implements[ifaceType] {
		if impl == concreteType {
			return // already registered
		}
	}
	Implements[ifaceType] = append(Implements[ifaceType], concreteType)
}

// ----------------------------------------------------------------------------
// Type Data

var Typedata = make(map[reflect.Type]Typeinfo)

func RegisterType(typ interface{}, info Typeinfo) {
	Typedata[reflect.TypeOf(typ)] = info
}

// Typeinfo contains metadata for types.
type Typeinfo struct {
	// Doc is the documentation for the type as a whole.
	Doc string

	// Fields contains field metadata indexed by field name.
	Field map[string]Fieldinfo
}

// Fielddata contains metadata to fields of structs.
type Fieldinfo struct {
	Doc       string         // Doc is the field documentation
	Multiline bool           // Multiline allows multiline strings
	Const     bool           // Const values are unchangable (display only)
	Only      []string       // Only contains the set of allowed values
	Validate  *regexp.Regexp // Validate this field
	Omit      bool           // Omit this field
}

// ----------------------------------------------------------------------------
// CSS

// CSS contains some minimal CSS definitions needed to render the HTML properly.
var CSS = `
body {
  margin: 40px;
}

textarea {
  vertical-align: text-top;
}

.Notrun { color: grey; }
.Skipped { color: grey; }
.Pass { color: darkgreen; }
.Fail { color: red; }
.Bogus { color: magenta; }
.Error { color: magenta; }
.error { colro: darkred; }

p.msg-bogus {
  color: fuchsia;
  font-weigth: bold;
  margin: 2px 0px 2px 10px;
}

p.msg-error {
  color: red;
  font-weigth: bold;
  margin: 2px 0px 2px 10px;
}

p.msg-fail {
  color: tomato;
  margin: 2px 0px 2px 10px;
}

p.msg-pass {
  color: green;
  margin: 2px 0px 2px 10px;
}

p.msg-skipped {
  color: dim-grey;
  margin: 2px 0px 2px 10px;
}

p.msg-notrun {
  color: light-grey;
  margin: 2px 0px 2px 10px;
}


table {
  border-collapse: collapse;
}

table.map>tbody>tr>th, table.map>tbody>tr>td {
  border-top: 1px solid #777;
  border-bottom: 1px solid #777;
  padding-top: 4px;  
  padding-bottom: 4px;  
}

th {
    text-align: right;
}

td, th {
  vertical-align: top;
}

pre {
  margin: 4px;
}

.tooltip {
  position: relative;
  display: inline-block;
}

.tooltip .tooltiptext {
  visibility: hidden;
  width: 656px;
  background-color: #404040;
  color: #eeeeee;
  text-align: left;
  border-radius: 6px;
  padding: 6px;

  /* Position the tooltip */
  position: absolute;
  z-index: 1;
  top: 20px;
  left: 20%;
}

.tooltip:hover .tooltiptext {
  visibility: visible;
}

input[type="text"] {
  width: 400px;
}

label {
  display: inline-block;
  width: 7em;
  text-align: right;
  vertical-align: text-top;
}

.actionbutton {
  background-color: #4CAF50;
  border: none;
  color: black;
  padding: 15px 32px;
  text-align: center;
  text-decoration: none;
  display: inline-block;
  width: 200px;
  font-size: 18px;
  font-family: "Arial Black", Gadget, sans-serif;
  margin: 4px 2px;
  cursor: pointer;
}

div.implements-buttons {
  margin-right: 250px;
}
`

// ----------------------------------------------------------------------------
// Favicon

// Favicon is a blue/red "ht" in 16x16 ico format.
var Favicon = []byte{
	0, 0, 1, 0, 1, 0, 16, 16, 16, 0, 1, 0, 4, 0, 40, 1,
	0, 0, 22, 0, 0, 0, 40, 0, 0, 0, 16, 0, 0, 0, 32, 0,
	0, 0, 1, 0, 4, 0, 0, 0, 0, 0, 128, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 2, 2, 179, 0, 184, 6, 14, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 2, 32, 0, 34, 0, 1, 16, 0, 2, 32,
	0, 34, 0, 1, 16, 0, 2, 32, 0, 34, 0, 1, 16, 0, 2, 32,
	0, 34, 0, 1, 16, 0, 2, 32, 0, 34, 0, 1, 16, 0, 2, 32,
	0, 34, 0, 1, 16, 0, 2, 34, 0, 34, 0, 1, 16, 0, 2, 34,
	34, 32, 0, 1, 16, 0, 2, 32, 34, 0, 17, 17, 17, 17, 2, 32,
	0, 0, 17, 17, 17, 17, 2, 32, 0, 0, 0, 1, 16, 0, 2, 32,
	0, 0, 0, 1, 16, 0, 2, 32, 0, 0, 0, 1, 16, 0, 2, 32,
	0, 0, 0, 1, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255,
	0, 0, 156, 231, 0, 0, 156, 231, 0, 0, 156, 231, 0, 0, 156, 231,
	0, 0, 156, 231, 0, 0, 156, 231, 0, 0, 140, 231, 0, 0, 129, 231,
	0, 0, 147, 0, 0, 0, 159, 0, 0, 0, 159, 231, 0, 0, 159, 231,
	0, 0, 159, 231, 0, 0, 159, 231, 0, 0, 255, 255, 0, 0,
}
