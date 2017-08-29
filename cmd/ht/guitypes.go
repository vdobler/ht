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

var htTest = map[string]gui.Fieldinfo{
	"Name":        gui.Fieldinfo{Doc: "Short name of the test"},
	"Description": gui.Fieldinfo{Multiline: true},
	"Request":     gui.Fieldinfo{Doc: "The HTTP Request"},
	"Checks":      gui.Fieldinfo{Doc: "The checks to perform"},
	"Execution":   gui.Fieldinfo{Doc: "Control the test execution"},
	"Jar":         gui.Fieldinfo{Omit: true},
	"Log":         gui.Fieldinfo{Omit: true},
	"Variables": gui.Fieldinfo{
		Doc: `Variables contains name/value-pairs used for variable substitution
in files read in, e.g. for Request.Body = \"@vfile:/path/to/file\".`,
	},
	"Response":     gui.Fieldinfo{Doc: "The received response"},
	"Status":       gui.Fieldinfo{Doc: "Test status. 0=NotRun 1=Skipped 2=Pass 3=Fail 4=Error 5=Bogus", Const: true},
	"Error":        gui.Fieldinfo{Doc: "Error", Const: true},
	"Duration":     gui.Fieldinfo{Doc: "Duration of last test execution", Const: true},
	"FullDuration": gui.Fieldinfo{Doc: "Overal duration including retries", Const: true},
	"Tries":        gui.Fieldinfo{Doc: "Number of tries executed", Const: true},
	"CheckResults": gui.Fieldinfo{Doc: "The outcome of the checks", Const: true},
	"VarEx":        gui.Fieldinfo{Doc: "Extract variables"},
	"ExValues":     gui.Fieldinfo{Doc: "Extracted values", Const: true},
}

var htExecution = map[string]gui.Fieldinfo{
	"Tries": gui.Fieldinfo{
		Doc: `Tries is the maximum number of tries made for this test.
Both 0 and 1 mean: "Just one try. No redo."
Negative values indicate that the test should be skipped
altogether.`,
	},
	"Wait":       gui.Fieldinfo{Doc: `Wait time between retries.`},
	"PreSleep":   gui.Fieldinfo{Doc: `Sleep time before request`},
	"InterSleep": gui.Fieldinfo{Doc: `Sleep time between request and checks`},
	"PostSleep":  gui.Fieldinfo{Doc: `Sleep time after checks`},
	"Verbosity":  gui.Fieldinfo{Doc: `Verbosity level in logging.`},
}

var htRequest = map[string]gui.Fieldinfo{
	"Method": gui.Fieldinfo{
		Doc: `Method is the HTTP method to use.
A empty method is equivalent to "GET"`,
		Only: []string{"GET", "POST", "HEAD", "PUT", "DELETE", "PATCH"},
	},
	"URL": gui.Fieldinfo{Doc: `the URL of the request`},
	"Params": gui.Fieldinfo{
		Doc: `Params contains the parameters and their values to send in
the request.

If the parameters are sent as multipart it is possible to include
files by special formated values.
The following formats are recognized:
   @file:/path/to/thefile
        read in /path/to/thefile and use its content as the
        parameter value. The path may be relative.
   @vfile:/path/to/thefile
        read in /path/to/thefile and perform variable substitution
        in its content to yield the parameter value.
   @file:@name-of-file:direct-data
   @vfile:@name-of-file:direct-data
        use direct-data as the parameter value and name-of-file
        as the filename. (There is no difference between the
        @file and @vfile variants; variable substitution has
        been performed already and is not done twice on direct-data.`,
	},
	"ParamsAs": gui.Fieldinfo{
		Doc: `determines how the parameters in the Param field are sent:
  "URL" or "": append properly encoded to URL
  "body"     : send as application/x-www-form-urlencoded in body.
  "multipart": send as multipart/form-data in body.
The two values "body" and "multipart" must not be used
on a GET or HEAD request.`,
		Only: []string{"URL", "body", "multipart"},
	},
	"Header": gui.Fieldinfo{
		Doc: `Header contains the specific http headers to be sent in this request.
User-Agent and Accept headers are set automaticaly to the global
default values if not set explicitly.`,
	},
	"Cookies": gui.Fieldinfo{Doc: `the cookies to send in the request`},
	"Body": gui.Fieldinfo{
		Doc: `the full body to send in the request. Body must be
empty if Params are sent as multipart or form-urlencoded.
The @file: and @vfile: prefixes are recognised and work like described
in Params`,
		Multiline: true,
	},
	"FollowRedirects": gui.Fieldinfo{Doc: `Check to follow redirect automatically`},
	"BasicAuthUser":   gui.Fieldinfo{Doc: `Username to send in Basic Auth header`},
	"BasicAuthPass":   gui.Fieldinfo{Doc: `Password to send in basic Auth header`},
	"Chunked": gui.Fieldinfo{
		Doc: `turns of setting of the Content-Length header resulting
in chunked transfer encoding of POST bodies`,
	},
	"Timeout":    gui.Fieldinfo{Doc: `of the request, 0 means the defaults to 10s`},
	"Request":    gui.Fieldinfo{Doc: `The underlying Go http.Request`, Const: true},
	"SentBody":   gui.Fieldinfo{Doc: `The actual sent body data`, Multiline: true, Const: true},
	"SentParams": gui.Fieldinfo{Doc: `The actual sent parameters`, Const: true},
}

var httpResponse = map[string]gui.Fieldinfo{
	"Body":    gui.Fieldinfo{Omit: true},
	"Close":   gui.Fieldinfo{Omit: true},
	"Request": gui.Fieldinfo{Omit: true},
	"TLS":     gui.Fieldinfo{Omit: true},
}

var httpRequest = map[string]gui.Fieldinfo{
	"ProtoMajor":    gui.Fieldinfo{Omit: true},
	"ProtoMinor":    gui.Fieldinfo{Omit: true},
	"Body":          gui.Fieldinfo{Omit: true},
	"Close":         gui.Fieldinfo{Omit: true},
	"Form":          gui.Fieldinfo{Omit: true},
	"PostForm":      gui.Fieldinfo{Omit: true},
	"MultipartForm": gui.Fieldinfo{Omit: true},
	"Trailer":       gui.Fieldinfo{Omit: true},
	"TLS":           gui.Fieldinfo{Omit: true},
	"Response":      gui.Fieldinfo{Omit: true},
}

func registerGUITypes() {
	gui.RegisterType(ht.Test{}, gui.Typeinfo{
		Doc:   "The Test",
		Field: htTest},
	)

	gui.RegisterType(ht.Execution{}, gui.Typeinfo{
		Doc:   "Parameters controlling the test execution.",
		Field: htExecution,
	})

	gui.RegisterType(ht.Request{}, gui.Typeinfo{
		Doc:   "The HTTP request.",
		Field: htRequest,
	})

	gui.RegisterType(http.Response{}, gui.Typeinfo{
		Doc:   "The HTTP response.",
		Field: httpResponse,
	})

	gui.RegisterType(http.Request{}, gui.Typeinfo{
		Doc:   "The HTTP request.",
		Field: httpRequest,
	})

	registerCheckAndExtractorTypes()
}

func registerCheckAndExtractorTypes() {
	register := func(name string, typ reflect.Type) {
		name = strings.ToLower(name)
		doc, ok := typeDoc[name]
		if !ok {
			return
		}
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		t := reflect.Zero(typ).Interface()
		gui.RegisterType(t, gui.Typeinfo{Doc: doc})
	}
	for name, typ := range ht.CheckRegistry {
		register(name, typ)
	}
	for name, typ := range ht.ExtractorRegistry {
		register(name, typ)
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
