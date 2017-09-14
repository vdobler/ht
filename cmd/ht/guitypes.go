// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/vdobler/ht/gui"
	"github.com/vdobler/ht/ht"
)

// registerGUITypes set special fields for handselected fields in
// some types. All normal tooltip documentation is registered in
// guidata.go.
func registerGUITypes() {
	setFieldSpecials(
		ht.Test{},
		"Jar,Log",
		"Result",
		"Description",
	)

	setFieldSpecials(ht.Request{}, "",
		"Request,SentBody,SentParams", "Body,SentBody")
	setFieldOnly(ht.Request{}, "Method", ",GET,POST,HEAD,PUT,DELETE,PATCH")
	setFieldOnly(ht.Request{}, "ParamsAs", ",URL,body,multipart")

	setFieldSpecials(
		http.Request{},
		"ProtoMajor,ProtoMinor,Body,Close,Form,PostForm,MultipartForm,"+
			"Trailer,TLS,Response",
		"", "",
	)

	setFieldSpecials(
		http.Response{},
		"Body,Close,Request,TLS",
		"", "",
	)

	setFieldSpecials(ht.CustomJS{}, "", "", "Script")
	setFieldSpecials(ht.JSExtractor{}, "", "", "Script")

	/*
		This is too much. This field in Condition is used rarely
		and the GUI is overloaded with all these checkboxes as
		Condition is a common element in checks.
		setFieldAny(ht.Condition{}, "Is",
			"Alpha Alphanumeric ASCII Base64 CIDR CreditCard DataURI "+
				"DialString DNSName Email FilePath Float FullWidth "+
				"HalfWidth Hexadecimal Hexcolor Host Int IP IPv4 IPv6 "+
				"ISBN10 ISBN13 ISO3166Alpha2 ISO3166Alpha3 JSON "+
				"Latitude Longitude LowerCase MAC MongoID Multibyte "+
				"Null Numeric Port PrintableASCII RequestURI RequestURL "+
				"RFC3339 RGBcolor Semver SSN UpperCase URL UTFDigit "+
				"UTFLetter UTFLetterNumeric UTFNumeric UUID UUIDv3 UUIDv4 "+
				"UUIDv5 VariableWidth")
	*/

	// ---- Checks ---

	setFieldAny(ht.SetCookie{}, "Type",
		"session persistent secure unsafe httpOnly exposed")

	setFieldOnly(ht.Resilience{}, "Method", "GET,POST,HEAD,PUT,DELETE,PATCH")
	resilMod := "all drop none double twice change delete nonsense space " +
		"malicious user empty type large tiny negative"
	setFieldAny(ht.Resilience{}, "ModParam", resilMod)
	setFieldAny(ht.Resilience{}, "ModHeader", resilMod)

	setFieldAny(ht.ValidHTML{}, "Ignore",
		"doctype structure uniqueids lang attr escaping attresc label url")

}

func setFieldOnly(t interface{}, field, values string) {
	typ := reflect.TypeOf(t)
	ti := gui.Typedata[typ]
	fi := ti.Field[field]
	fi.Only = strings.Split(values, ",")
	ti.Field[field] = fi
}

func setFieldAny(t interface{}, field, values string) {
	typ := reflect.TypeOf(t)
	ti := gui.Typedata[typ]
	fi := ti.Field[field]
	fi.Any = strings.Split(values, " ")
	ti.Field[field] = fi
}

func setFieldSpecials(t interface{}, omit, readonly, multiline string) {
	typ := reflect.TypeOf(t)
	ti := gui.Typedata[typ]
	if ti.Field == nil {
		ti.Field = make(map[string]gui.Fieldinfo)
	}
	for _, field := range strings.Split(omit, ",") {
		if field == "" {
			continue
		}
		fi := ti.Field[field]
		fi.Omit = true
		ti.Field[field] = fi
	}
	for _, field := range strings.Split(readonly, ",") {
		if field == "" {
			continue
		}
		fi := ti.Field[field]
		fi.Const = true
		ti.Field[field] = fi
	}
	for _, field := range strings.Split(multiline, ",") {
		if field == "" {
			continue
		}
		fi := ti.Field[field]
		fi.Multiline = true
		ti.Field[field] = fi
	}
}

func registerGUIImplements() {
	names := []string{}
	for name := range ht.CheckRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		typ := ht.CheckRegistry[name]
		gui.RegisterImplementation(
			(*ht.Check)(nil), reflect.Zero(typ).Interface())
	}

	names = names[:0]
	for name := range ht.ExtractorRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		typ := ht.ExtractorRegistry[name]
		gui.RegisterImplementation(
			(*ht.Extractor)(nil), reflect.Zero(typ).Interface())
	}
}
