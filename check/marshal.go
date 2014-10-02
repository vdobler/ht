// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package check

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"reflect"
	"strings"
)

func MarshalCheck(c Check) ([]byte, error) {
	data, err := xml.Marshal(c)
	if err != nil {
		return nil, err
	}
	// Most checks have no nested elements and can be selfclosing which is
	// a bit more plesant.
	if bytes.Count(data, []byte{'<'}) == 2 {
		i := bytes.LastIndex(data, []byte{'<'})
		if data[i-1] == '>' {
			data[i-1] = ' '
			data[i] = '/'
			data[i+1] = '>'
			data = data[:i+2]
		}
	}
	return data, nil
}

func UnmarshalCheck(data []byte) (Check, error) {
	d := struct {
		XMLName xml.Name
	}{}
	if err := xml.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	check, err := CreateCheck(d.XMLName.Local, data)
	return check, err
}

// CreateCheck constructs the Check name from data.
func CreateCheck(name string, data []byte) (Check, error) {
	typ, ok := CheckRegistry[name]
	if !ok {
		return nil, fmt.Errorf("no such check registered")
	}
	check := reflect.New(typ)
	err := xml.Unmarshal(data, check.Interface())
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal check: %s", err.Error())
	}
	check = reflect.Indirect(check)
	return check.Interface().(Check), nil
}

// ----------------------------------------------------------------------------
// Helpers

// SubstituteVariables returns a deep copy of check with all exported string
// fields in check modified by applying r.  TODO: applying r is not "variable replacing"
func SubstituteVariables(check Check, r *strings.Replacer) Check {
	src := reflect.ValueOf(check)
	dst := reflect.New(src.Type()).Elem()
	deepCopy(dst, src, r)
	return dst.Interface().(Check)
}

// deepCopy copes src recursively to dst while transforming all string fields
// by applying r.
func deepCopy(dst, src reflect.Value, r *strings.Replacer) {
	if !dst.CanSet() {
		return
	}
	switch src.Kind() {
	case reflect.String:
		// TODO: maybe skip certain fields based on their struct tag?
		dst.SetString(r.Replace(src.String()))
	case reflect.Struct:
		for i := 0; i < src.NumField(); i += 1 {
			deepCopy(dst.Field(i), src.Field(i), r)
		}
	case reflect.Slice:
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Cap()))
		for i := 0; i < src.Len(); i += 1 {
			deepCopy(dst.Index(i), src.Index(i), r)
		}
	case reflect.Map:
		dst.Set(reflect.MakeMap(src.Type()))
		for _, key := range src.MapKeys() {
			srcValue := src.MapIndex(key)
			dstValue := reflect.New(srcValue.Type()).Elem()
			deepCopy(dstValue, srcValue, r)
			dst.SetMapIndex(key, dstValue)
		}
	case reflect.Ptr:
		src = src.Elem()
		if !src.IsValid() {
			return
		}
		dst.Set(reflect.New(src.Type()))
		deepCopy(dst.Elem(), src, r)
	case reflect.Interface:
		// Like Pointer but with one more call to Elem.
		src = src.Elem()
		dstIface := reflect.New(src.Type()).Elem()
		deepCopy(dstIface, src, r)
		dst.Set(dstIface)
	default:
		dst.Set(src)
	}
}
