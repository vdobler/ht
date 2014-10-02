// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"reflect"

	"github.com/vdobler/ht/check"
)

func findNV(check check.Check) []string {
	v := reflect.ValueOf(check)
	return findNVRec(v)
}

func findNVRec(v reflect.Value) (a []string) {
	switch v.Kind() {
	case reflect.String:
		m := nowTimeRe.FindAllString(v.String(), 1)
		if m == nil {
			return
		}
		return m
	case reflect.Struct:
		for i := 0; i < v.NumField(); i += 1 {
			a = append(a, findNVRec(v.Field(i))...)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i += 1 {
			a = append(a, findNVRec(v.Index(i))...)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			a = append(a, findNVRec(v.MapIndex(key))...)
		}
	case reflect.Ptr:
		v = v.Elem()
		if !v.IsValid() {
			return nil
		}
		a = findNVRec(v)
	case reflect.Interface:
		v = v.Elem()
		a = findNVRec(v)

	}
	return a
}
