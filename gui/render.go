// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gui

import (
	"fmt"
	"html/template"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func indent(depth int) string {
	return strings.Repeat("    ", depth)
}

func (v *Value) renderError(path string, depth int) {
	err := v.errors[path]
	if err == nil {
		return
	}

	v.printf("%s<p class=\"error\">%s</p>\n",
		indent(depth),
		template.HTMLEscapeString(err.Error()))
}

// ----------------------------------------------------------------------------
// Recursive rendering of HTML form

// render down val, emitting HTML to buf.
// Path is the prefix to the current input name.
func (v *Value) render(path string, depth int, readonly bool, val reflect.Value) error {
	switch val.Kind() {
	case reflect.Bool:
		return v.renderBool(path, depth, readonly, val)
	case reflect.String:
		return v.renderString(path, depth, readonly, val)
	case reflect.Int64:
		if isDuration(val) {
			return v.renderDuration(path, depth, readonly, val)
		}
		fallthrough
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return v.renderInt(path, depth, readonly, val)
	case reflect.Float32, reflect.Float64:
		return v.renderFloat64(path, depth, readonly, val)
	case reflect.Struct:
		return v.renderStruct(path, depth, readonly, val)
	case reflect.Map:
		return v.renderMap(path, depth, readonly, val)
	case reflect.Slice:
		return v.renderSlice(path, depth, readonly, val)
	case reflect.Ptr:
		return v.renderPtr(path, depth, readonly, val)
	case reflect.Interface:
		return v.renderInterface(path, depth, readonly, val)
	}

	fmt.Println("gui: won't render", val.Kind().String(), "in", path)

	return nil
}

func isDuration(v reflect.Value) bool {
	t := v.Type()
	return (t.PkgPath() == "time" && t.Name() == "Duration") ||
		(t.PkgPath() == "github.com/vdobler/ht/ht" && t.Name() == "Duration")
}

// ----------------------------------------------------------------------------
// Primitive Types

func (v *Value) renderBool(path string, depth int, readonly bool, val reflect.Value) error {
	v.renderError(path, depth)
	v.printf("%s", indent(depth))

	if readonly {
		if val.Bool() {
			v.printf("&#x2611;") // ☑
		} else {
			v.printf("&#x2610;") // ☐
		}
		return nil
	}

	checked := ""
	if val.Bool() {
		checked = " checked"
	}
	v.printf("<input type=\"checkbox\" name=\"%s\" value=\"true\" %s/>\n",
		template.HTMLEscapeString(path),
		checked)

	return nil
}

func (v *Value) renderString(path string, depth int, readonly bool, val reflect.Value) error {
	v.renderError(path, depth)
	v.printf("%s", indent(depth))

	isMultiline := strings.Contains(val.String(), "\n")
	escVal := template.HTMLEscapeString(val.String())
	if readonly {
		if isMultiline {
			v.printf("<pre>")
			for _, line := range strings.Split(val.String(), "\n") {
				v.printf("%s\n", template.HTMLEscapeString(line))
			}
			v.printf("</pre>")
		} else {
			v.printf("<code>%s</code>", escVal)
		}
		return nil
	}

	if v.nextfieldinfo.Multiline || isMultiline {
		v.printf("<textarea cols=\"82\" rows=\"5\" name=\"%s\">%s</textarea>\n",
			template.HTMLEscapeString(path),
			escVal)

	} else if len(v.nextfieldinfo.Only) > 0 {
		v.printf("<select name=\"%s\">\n", template.HTMLEscapeString(path))
		current := val.String()
		for _, only := range v.nextfieldinfo.Only {
			selected := ""
			if current == only {
				selected = ` selected="selected"`
			}
			v.printf("%s<option%s>%s</option>\n",
				indent(depth+1),
				selected,
				template.HTMLEscapeString(only))
		}
		v.printf("%s</select>\n", indent(depth))
	} else {
		v.printf("<input type=\"text\" name=\"%s\" value=\"%s\" />\n",
			template.HTMLEscapeString(path),
			escVal)
	}
	return nil
}

func (v *Value) renderDuration(path string, depth int, readonly bool, val reflect.Value) error {
	v.renderError(path, depth)
	v.printf("%s", indent(depth))

	dv := val.Convert(reflect.TypeOf(time.Duration(0)))
	d := dv.Interface().(time.Duration)
	escDuration := template.HTMLEscapeString(d.String())

	if readonly {
		v.printf("%s", escDuration)
		return nil
	}

	v.printf("<input type=\"text\" name=\"%s\" value=\"%s\" />\n",
		template.HTMLEscapeString(path),
		escDuration)

	return nil
}

func (v *Value) renderInt(path string, depth int, readonly bool, val reflect.Value) error {
	v.renderError(path, depth)
	v.printf("%s", indent(depth))

	if readonly {
		v.printf("%d", val.Int())
		return nil
	}

	v.printf("<input type=\"number\" name=\"%s\" value=\"%d\" />\n",
		template.HTMLEscapeString(path),
		val.Int())

	return nil
}

func (v *Value) renderFloat64(path string, depth int, readonly bool, val reflect.Value) error {
	v.renderError(path, depth)
	v.printf("%s", indent(depth))

	if readonly {
		v.printf("%f", val.Float())
		return nil
	}

	v.printf("<input type=\"number\" name=\"%s\" value=\"%f\" step=\"any\"/>\n",
		template.HTMLEscapeString(path),
		val.Float())

	return nil
}

// ----------------------------------------------------------------------------
// Pointers

func (v *Value) renderPtr(path string, depth int, readonly bool, val reflect.Value) error {
	if val.IsNil() {
		return v.renderNilPtr(path, depth, readonly, val)
	}
	return v.renderNonNilPtr(path, depth, readonly, val)

}

func (v *Value) renderNonNilPtr(path string, depth int, readonly bool, val reflect.Value) error {

	if !readonly {
		op := path + ".__OP__"

		v.printf("%s<button name=\"%s\" value=\"Remove\">-</button>\n",
			indent(depth),
			template.HTMLEscapeString(op),
		)
	}

	return v.render(path, depth, readonly, val.Elem())
}

func (v *Value) renderNilPtr(path string, depth int, readonly bool, val reflect.Value) error {
	if readonly {
		return nil
	}

	op := path + ".__OP__"
	v.printf("%s<button name=\"%s\" value=\"Add\">+</button>\n",
		indent(depth),
		template.HTMLEscapeString(op),
	)
	return nil
}

// ----------------------------------------------------------------------------
// Interface

func (v *Value) renderInterface(path string, depth int, readonly bool, val reflect.Value) error {
	if val.IsNil() {
		return v.renderNilInterface(path, depth, readonly, val)
	}
	return v.renderNonNilInterface(path, depth, readonly, val)

}

func (v *Value) renderNonNilInterface(path string, depth int, readonly bool, val reflect.Value) error {
	if !readonly {
		op := path + ".__OP__"

		v.printf("%s<button name=\"%s\" value=\"Remove\">-</button>\n",
			indent(depth),
			template.HTMLEscapeString(op),
		)
	}

	return v.render(path, depth, readonly, val.Elem())
}

func (v *Value) renderNilInterface(path string, depth int, readonly bool, val reflect.Value) error {
	if readonly {
		return nil
	}

	op := path + ".__TYPE__"
	for _, typ := range Implements[val.Type()] {
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		name, tooltip := v.typeinfo(typ)
		hname := template.HTMLEscapeString(name)

		v.printf("%s<button name=\"%s\" value=\"%s\" class=\"tooltip\">%s<span class=\"tooltiptext\"><pre>%s</pre></span></button> &nbsp; \n",
			indent(depth),
			template.HTMLEscapeString(op),
			hname, hname,
			template.HTMLEscapeString(tooltip),
		)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Slices

func (v *Value) renderSlice(path string, depth int, readonly bool, val reflect.Value) error {
	v.printf("%s<table>\n", indent(depth))
	var err error
	for i := 0; i < val.Len(); i++ {
		field := val.Index(i)
		fieldPath := fmt.Sprintf("%s.%d", path, i)

		v.printf("%s<tr>\n", indent(depth+1))

		// Index number and controls.
		v.printf("%s<td>%d:</td>\n", indent(depth+2), i)
		if !readonly {
			v.printf("%s<td><button name=\"%s\" value=\"Remove\">-</button></td>\n",
				indent(depth+2),
				template.HTMLEscapeString(fieldPath+".__OP__"),
			)
			if false && i > 0 {
				v.printf("<button>↑</button> ")
			}
		}

		// The field itself.
		v.printf("%s<td>\n", indent(depth+2))
		e := v.render(fieldPath, depth+3, readonly, field)
		if e != nil {
			err = e
		}
		v.printf("%s</td>\n", indent(depth+2))

		v.printf("%s</tr>\n", indent(depth+1))
	}
	v.printf("%s<tr>\n", indent(depth+1))
	if !readonly {
		v.printf("%s<td><button name=\"%s\" value=\"Add\">+</button></td>\n",
			indent(depth+2),
			template.HTMLEscapeString(path+".__OP__"),
		)
	}
	v.printf("%s</tr>\n", indent(depth+1))
	v.printf("%s</table>\n", indent(depth))

	return err
}

// ----------------------------------------------------------------------------
// Structures

// Structs are easy: all fields are fixed, nothing to add or delete.
func (v *Value) renderStruct(path string, depth int, readonly bool, val reflect.Value) error {
	var err error

	typename, tooltip := v.typeinfo(val)
	v.printf("\n")
	v.printf("%s<fieldset>\n", indent(depth))
	depth++
	v.printf(`%s<legend class="tooltip">%s<span class="tooltiptext"><pre>%s</pre></span></legend>
`,
		indent(depth),
		template.HTMLEscapeString(typename),
		template.HTMLEscapeString(tooltip))

	v.printf("%s<table>\n", indent(depth))
	for i := 0; i < val.NumField(); i++ {
		name, finfo := v.fieldinfo(val, i)
		if unexported(name) || finfo.Omit || unwalkable(val.Field(i)) {
			continue
		}

		v.printf("%s<tr>\n", indent(depth+1))
		tooltip := finfo.Doc
		v.printf(`%s<th class="tooltip">%s:<span class="tooltiptext"><pre>%s</pre></span></th>`+"\n",
			indent(depth+2),
			template.HTMLEscapeString(name),
			template.HTMLEscapeString(tooltip))
		field := val.Field(i)

		v.printf("%s<td>\n", indent(depth+2))
		v.nextfieldinfo = finfo
		e := v.render(path+"."+name, depth+3, readonly || finfo.Const, field)
		v.nextfieldinfo = Fieldinfo{}
		if e != nil {
			err = e
		}
		v.printf("%s<td>\n", indent(depth+2))

		v.printf("%s</tr>\n", indent(depth+1))
	}
	v.printf("%s</table>\n", indent(depth))
	depth--

	// <div class="Pass">Pass</div>
	v.printf("%s</fieldset>\n", indent(depth))
	v.printf("\n")

	return err
}

func unexported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return !unicode.IsUpper(r)
}

func unwalkable(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Invalid, reflect.Array, reflect.Chan, reflect.Func,
		reflect.Complex64, reflect.Complex128, reflect.UnsafePointer:
		return true
	}
	return false
}

// ----------------------------------------------------------------------------
// Maps

// Major problem with maps: Its elements are not addressable and thus
// not setable.

func (v *Value) renderMap(path string, depth int, readonly bool, val reflect.Value) error {
	v.printf("%s<table class=\"map\">\n", indent(depth))
	var err error
	keys := val.MapKeys()

	sortMapKeys(keys)

	for _, k := range val.MapKeys() {
		mv := val.MapIndex(k)
		name := k.String() // BUG: panics if map is indexed by anything else than strings
		v.printf("%s<tr>\n", indent(depth+1))

		elemPath := path + "." + mangleKey(name)
		if !readonly {
			v.printf("%s<td><button name=\"%s\" value=\"Remove\">-</button></td>\n",
				indent(depth+2), elemPath)
		}
		v.printf("%s<th>%s</th>\n", indent(depth+2),
			template.HTMLEscapeString(name))

		v.printf("%s<td>\n", indent(depth+2))
		e := v.render(elemPath, depth+3, readonly, mv)
		if e != nil {
			err = e
		}
		v.printf("%s</td>\n", indent(depth+2))

		v.printf("%s</tr>\n", indent(depth+1))
	}

	// New entries
	if !readonly {
		v.printf("%s<tr>\n", indent(depth+1))

		v.printf("%s<td colspan=\"2\">\n", indent(depth+2))
		v.printf("%s<input type=\"text\" name=\"%s.__NEW__\" style=\"width: 75px;\"/>\n",
			indent(depth+3), path)
		v.printf("%s</td>\n", indent(depth+2))
		v.printf("%s<td>\n", indent(depth+2))
		v.printf("%s<button name=\"%s.__OP__\" value=\"Add\">+</button>\n",
			indent(depth+3), path)
		v.printf("%s</td>\n", indent(depth+2))

		v.printf("%s</tr>\n", indent(depth+1))
	}

	v.printf("%s</table>\n", indent(depth))

	return err
}

// mangleName takes an arbitrary key of a map and produces a string
// suitable as a HTML form parameter.
func mangleKey(n string) string {
	return n // TODO
}

// demangleKey is the inverse of mangleKey
func demangleKey(n string) string {
	return n // TODO
}

func sortMapKeys(keys []reflect.Value) {
	if len(keys) == 0 {
		return
	}

	if keys[0].Kind() == reflect.String {
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
	}

	// TODO at least ints too.
}
