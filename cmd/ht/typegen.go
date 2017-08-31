// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vdobler/ht/gui"
	"github.com/vdobler/ht/ht"
)

func main() {
	/*
		lines, err := typedoc("github.com/vdobler/ht/ht.JSON")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(lines)

		lines, err = fielddoc("github.com/vdobler/ht/ht.JSON.Sep")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(lines)
	*/

	JT := reflect.TypeOf(ht.Latency{})
	ti, err := typeinfo(JT)
	fmt.Println(err)
	fmt.Printf("%+v\n", ti)
}

func typeinfo(t reflect.Type) (gui.Typeinfo, error) {
	tinfo := gui.Typeinfo{}
	pkg, name := t.PkgPath(), t.Name()
	tdoc := typedoc(pkg + "." + name)

	tinfo.Doc = tdoc
	tinfo.Field = make(map[string]gui.Fieldinfo)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i).Name
		fmt.Println(i, field)
		if unexported(field) {
			continue
		}
		fdoc := fielddoc(pkg + "." + name + "." + field)

		tinfo.Field[field] = gui.Fieldinfo{
			Doc: fdoc,
		}
	}

	return tinfo, nil
}

func unexported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return !unicode.IsUpper(r)
}

func typedoc(typ string) string {
	gocmd := exec.Command("go", "doc", typ)
	output, err := gocmd.CombinedOutput()
	if err != nil {
		panic(err)
	}

	output = bytes.Replace(output,
		[]byte("`json:\",omitempty\"`"), []byte(""),
		-1)
	output = bytes.Replace(output,
		[]byte("`json:\"-\"`"), []byte(""),
		-1)

	line := strings.Split(string(output), "\n")
	if strings.HasSuffix(line[0], "{") {
		for line[0] != "}" {
			line = line[1:]
		}
		line = line[1:]
	} else {
		line = line[1:]
	}

	n := 0
	for line[n] == "" || strings.HasPrefix(line[n], "    ") {
		n++
	}
	line = line[:n]

	line = line[:n]
	n = len(line) - 1
	for n >= 0 && line[n] == "" {
		n--
	}
	line = line[:n+1]

	for i, l := range line {
		if strings.HasPrefix(l, "    ") {
			line[i] = l[4:]
		}
	}

	return strings.Join(line, "\n")
}

func fielddoc(field string) string {
	gocmd := exec.Command("go", "doc", field)
	output, err := gocmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		panic(err)
	}

	output = bytes.Replace(output,
		[]byte("`json:\",omitempty\"`"), []byte(""),
		-1)
	output = bytes.Replace(output,
		[]byte("`json:\"-\"`"), []byte(""),
		-1)

	line := strings.Split(string(output), "\n")
	line = line[1:]

	n := 0
	for strings.HasPrefix(line[n], "    //") {
		line[n] = line[n][7:]
		n++
	}
	line = line[:n]

	return strings.Join(line, "\n")
}
