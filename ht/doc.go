// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ht provides functions for easy testing of HTTP based protocols.
//
// Testing is done by constructing a request, executing the request and
// performing various checks on the returned response. The type Test captures
// this idea. Each Test may contain an arbitrary list of Checks which
// perform the actual validation work. Tests can be grouped into Suites
// which may provide a common cookie jar for their tests and may execute setup
// and teardown actions.
//
// All elements like Checks, Request, Tests and Suites are organized in
// a way to allow easy deserialisation from a text format. This allows to load
// and execute whole Suites at runtime.
//
//
// Checks
//
// A typical check validates a certain property of the received response.
// E.g.
//     StatusCode{Expect: 302}
//     Body{Contains: "foobar"}
//     Body{Contains: "foobar", Count: 2}
//     Body{Contains: "illegal", Count: -1}
// The last three examples show how zero values of optional fields are
// commonly used: The zero value of Count means "any number of occurrences".
// Forbidding the occurenc of "foobar" thus requires a negative Count.
//
// The following checks are provided
//     * AnyOne          logical OR of several tests
//     * Body            text in the response body
//     * ContentType     Content-Type header
//     * CustomJS        performed by your own JavaScript code
//     * DeleteCookie    for proper deletion of cookies
//     * FinalURL        final URL after a redirect chain
//     * Header          presence and values of received HTTP header
//     * HTMLContains    text content of CSS-selected elements
//     * HTMLTag         occurrence HTML elements chosen via CSS-selectors
//     * Header          HTTP header fields
//     * Identity        the SHA1 hash of the HTTP body
//     * Image           image format, size and content
//     * JSON            structure and content of a JSON body
//     * JSONExpr        structure and content of a JSON body
//     * Latency         latency distribution of a request
//     * Links           accesability of hrefs and srcs in HTML
//     * Logfile         data written to a logfile
//     * NoServerError   no timeout and no 5xx status code
//     * None            logical NAND
//     * Redirect        redirection
//     * RedirectChain   several redirections
//     * RenderedHTML    HTML after rendering via PhantomJS
//     * RenderingTime   time to render page via PhantomJS
//     * Resilience      how wellbehaved does the server answer modified requests
//     * ResponseTime    lower and higher bounds on the response time
//     * Screenshot      render screen via PhantomJS and compare to reference
//     * SetCookie       properties of received cookies
//     * Sorted          sorted occurrence of text on body
//     * StatusCode      the received HTTP status code
//     * UTF8Encoded     that the HTTP body is UTF-8 encoded
//     * ValidHTML       not obviousely malformed HTML
//     * W3CValidHTML    if body parses as valid HTML5
//     * XML             elements of a XML body
//
//
// Requests
//
// Requests allow to specify a HTTP request in a declarative manner.
// A wide variety of request can be generated from a purly textual
// representation of the Request.
//
// All the ugly stuff like parameter encoding, generation of multipart
// bodies, etc. are hidden from the user.
//
//
// Tests
//
// A Test is basically just a Request combined with a list of Checks.
// Running a Test is executing the request and validating the response
// according to the Checks.
//
// There are three ways in which a Tests may fail:
//   1. The test setup is malformed, such tests are called Bogus.
//   2. The request itself fails, e.g. due to a timeout. This is called an Error.
//   3. Any of the checks fail. This is called a Failure.
//
package ht
