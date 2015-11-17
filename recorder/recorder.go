// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// recorder is allows to capture request/response pairs via a
// reverse proxy and generate tests for these pairs.
package recorder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/json5"
)

// Events is the global list of recorded events.
var Events []Event

// Event is a request/response pair
type Event struct {
	Request   *http.Request              // The request
	Response  *httptest.ResponseRecorder // The recorded response
	Body      []byte                     // The captured body
	Timestamp time.Time                  // Timestamp when caputred
	Name      string                     // Used during dumping
}

type Options struct {
	// Disarm is the time span after a captured request/response pair
	// in which the capturing is disarmed.
	Disarm time.Duration

	IgnoredContentType *regexp.Regexp
	IgnoredPath        *regexp.Regexp
}

func (o Options) ignore(e Event) bool {
	if o.IgnoredPath != nil && o.IgnoredPath.MatchString(e.Request.URL.Path) {
		log.Println("Ignoring path", e.Request.URL.Path)
		return true
	}
	if o.IgnoredContentType != nil &&
		o.IgnoredContentType.MatchString(e.Response.HeaderMap.Get("Content-Type")) {
		log.Println("Ignoring content type ", e.Response.HeaderMap.Get("Content-Type"))
		return true
	}
	return false
}

var (
	remoteHost string
)

// StartReverseProxy listens on the local port and forwards request to remote
// while capturing the request/response pairs.
func StartReverseProxy(port string, remote *url.URL, opts Options) error {
	remoteHost = remote.Host
	requests := make(chan Event, 10)
	go process(requests, opts)

	proxy := httputil.NewSingleHostReverseProxy(remote)
	http.HandleFunc("/", handler(proxy, requests))
	log.Printf("Staring reverse proxying from localhost:%s to %s", port, remote.String())
	return http.ListenAndServe(port, nil)
}

func handler(p *httputil.ReverseProxy, requests chan Event) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rr := httptest.NewRecorder()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err.Error()) // Harsh but what else?
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		p.ServeHTTP(rr, r)
		requests <- Event{
			Request:   r,
			Response:  rr,
			Body:      body,
			Timestamp: time.Now()}
		for h, v := range rr.HeaderMap {
			w.Header()[h] = v
		}
		w.WriteHeader(rr.Code)
		w.Write(rr.Body.Bytes())
	}
}

func process(requests chan Event, opts Options) {
	log.Println("Started processing")
	last := time.Now()
	for e := range requests {
		delta := e.Timestamp.Sub(last)
		if delta < opts.Disarm {
			continue
		}
		if opts.ignore(e) {
			continue
		}
		last = e.Timestamp
		e.Name = fmt.Sprintf("Event %d", len(Events)+1)
		Events = append(Events, e)
		log.Println("Recorded", e.Request.Method, e.Request.URL, " --> ", e.Response.Code)
	}
}

// Test is a reduced version of ht.Test suitable for serialization to JSON5.
type Test struct {
	Name        string
	Description string   `json:",omitempty"`
	BasedOn     []string `json:",omitempty"`
	Request     ht.Request
	Checks      ht.CheckList `json:",omitempty"`
}

// Suite is a reduced version of ht.Suite suitable to serialization to JSON5.
type Suite struct {
	Name        string
	Description string `json:",omitempty"`
	Tests       []string
	Variables   map[string]string
}

// DumpEvents writes events to directory, it extracts common headers.
func DumpEvents(events []Event, directory string, suitename string) error {
	err := os.MkdirAll(directory, 0777)
	if err != nil {
		return err
	}

	// extract all common headers into mixin
	commonHeaders := ExtractCommonHeaders(events)
	test := &Test{
		Name: fmt.Sprintf("Common Header of %s", suitename),
		Request: ht.Request{
			Header: commonHeaders,
		},
	}

	commonFilename := path.Join(directory, "common-headers.mixin")
	err = writeTest(test, commonFilename)
	if err != nil {
		return err
	}

	suite := Suite{
		Name:        suitename,
		Description: fmt.Sprintf("Generated at %s", time.Now()),
		Variables: map[string]string{
			"HOSTNAME": remoteHost,
		},
	}

	for _, e := range events {
		host := e.Request.URL.Host
		e.Request.URL.Host = "H.O.S.T.N.A.M.E"
		cookies := []ht.Cookie{}
		for _, c := range e.Request.Cookies() {
			cookies = append(cookies, ht.Cookie{Name: c.Name, Value: c.Value})
		}
		e.Request.Header.Del("Cookie")

		params := e.Request.URL.Query()
		e.Request.URL.RawQuery = ""
		urlString := e.Request.URL.String()
		urlString = strings.Replace(urlString, "H.O.S.T.N.A.M.E", "{{HOSTNAME}}", 1)

		// TODO: scan body for parameters and set ParamsAs
		body := e.Body

		checks := extractChecks(e)

		test := &Test{
			Name:        e.Name,
			Description: fmt.Sprintf("Recorded from %s on %s", host, time.Now()),
			BasedOn:     []string{commonFilename},
			Request: ht.Request{
				Method:  e.Request.Method,
				URL:     urlString,
				Cookies: cookies,
				Header:  e.Request.Header,
				Params:  ht.URLValues(params),
				Body:    string(body),
			},
			Checks: checks,
		}

		name := strings.ToLower(strings.Replace(e.Name, " ", "_", -1)) + ".ht"
		suite.Tests = append(suite.Tests, name)
		filename := path.Join(directory, name)
		err = writeTest(test, filename)
		if err != nil {
			return err
		}

		e.Request.URL.Host = host
		log.Println("Generate test for ", e.Request.Method, e.Request.URL, " --> ", filename)
	}

	name := strings.ToLower(strings.Replace(suitename, " ", "_", -1)) + ".suite"
	filename := path.Join(directory, name)
	err = writeSuite(suite, filename)
	if err != nil {
		return err
	}
	log.Println("Generate suite ", filename)

	return nil
}

func writeTest(test *Test, filename string) error {
	data, err := json5.MarshalIndent(test, "", "    ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, data, 0666)
	if err != nil {
		return err
	}
	return nil
}

// TODO: combine with writeTest
func writeSuite(suite Suite, filename string) error {
	data, err := json5.MarshalIndent(suite, "", "    ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, data, 0666)
	if err != nil {
		return err
	}
	return nil
}

func extractChecks(e Event) ht.CheckList {
	list := ht.CheckList{}

	// Allways add StatusCode check.
	list = append(list, ht.StatusCode{Expect: e.Response.Code})

	// Check for Content-Type header.
	contentType := e.Response.Header().Get("Content-Type")
	if contentType != "" {
		contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
		if i := strings.Index(contentType, "/"); i != -1 {
			contentType := contentType[i+1:]
			list = append(list, ht.ContentType{Is: contentType})
		} else {
			contentType = ""
		}
	}

	// Checks for Set-Cookie headers:
	dummy := http.Response{Header: e.Response.Header()}
	now := e.Timestamp
	for _, c := range dummy.Cookies() {
		path := cookiePath(c, e.Request.URL)
		if c.MaxAge < 0 || (!c.Expires.IsZero() && c.Expires.Before(now)) {
			dc := &ht.DeleteCookie{Name: c.Name, Path: path}
			list = append(list, dc)
		} else {
			sc := createSetCookieCheck(c, now)
			sc.Path = ht.Condition{Equals: path}
			list = append(list, sc)
		}
	}

	// Check redirections:
	if loc := e.Response.HeaderMap.Get("Location"); loc != "" && e.Response.Code/100 == 3 { //  Uaaahhrg!
		red := &ht.Redirect{To: loc, StatusCode: e.Response.Code}
		list = append(list, red)
	}

	// Some HTML stuff
	if contentType == "html" || contentType == "xhtml" {

	}

	return list
}

func cookiePath(c *http.Cookie, u *url.URL) string {
	if c.Path != "" {
		return c.Path // assume this is well-formed
	}

	p := u.Path
	i := strings.LastIndex(p, "/")
	if i == 0 {
		return "/" // p ~ "/XYZ"
	}
	return p[:i] // Either p ~ "/XYZ/ABC" or p ~ "/XYZ/ABC/"
}

func createSetCookieCheck(c *http.Cookie, now time.Time) *ht.SetCookie {
	sc := &ht.SetCookie{Name: c.Name, Value: ht.Condition{Equals: c.Value}}

	lt := time.Duration(0)
	if c.MaxAge > 0 {
		lt = time.Second * time.Duration(c.MaxAge)
	} else if !c.Expires.IsZero() && c.Expires.After(now) {
		lt = c.Expires.Sub(now)
	}

	flags := []string{}
	if c.HttpOnly {
		flags = append(flags, "httpOnly")
	} else {
		flags = append(flags, "exposed")
	}
	if c.Secure {
		flags = append(flags, "secure")
	} else {
		flags = append(flags, "unsafe")
	}
	if lt > 0 {
		flags = append(flags, "persistent")
		if lt > 10*time.Second {
			lt -= 10 * time.Second
		}
		sc.MinLifetime = ht.Duration(lt)
	} else {
		flags = append(flags, "session")
	}
	sc.Type = strings.Join(flags, " ")

	return sc
}

// ExtractCommonHeaders from events.
func ExtractCommonHeaders(events []Event) http.Header {
	common := http.Header{}
	h0 := events[0].Request.Header
	for h, v := range h0 {
		vs := fmt.Sprintf("%v", v)
		identical := true
		for j := 2; j < len(events); j++ {
			if vs != fmt.Sprintf("%v", events[j].Request.Header[h]) {
				identical = false
				break
			}
		}
		if identical {
			common[h] = v
			for _, e := range events {
				e.Request.Header.Del(h)
			}
		}
	}
	return common
}
