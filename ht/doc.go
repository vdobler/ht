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
//     * Body            checks text in the response body
//     * ContentType     checks Content-Type header
//     * DeleteCookie    checks for proper deletion of cookies
//     * FinalURL        checks final URL after a redirect chain
//     * Header          checks presence and values of received HTTP header
//     * HTMLContains    checks text content of CSS-selected elements
//     * HTMLTag         checks occurrence HTML elements chosen via CSS-selectors
//     * Identity        checks the SHA1 hash of the HTTP body
//     * Image           checks image format, size and content
//     * JSON            checks structure and content of a JSON body
//     * Links           checks accesability of hrefs and srcs in HTML
//     * Logfile         checks data written to a logfile
//     * Redirect        checks for redirections
//     * ResponseTime    checks lower and higher bounds on the response time
//     * SetCookie       checks properties of received cookies
//     * StatusCode      checks the received HTTP status code
//     * UTF8Encoded     checks that the HTTP body is UTF-8 encoded
//     * W3CValidHTML    checks if body parses as valid HTML5
//     * XML             checks elements of a XML body
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
// Parametrisations
//
// Hardcoding e.g. the hostname in a test has obvious drawbacks. To overcome
// these the Request and Checks may be parametrised. This is done by a simple
// variable expansion in which occurrences of variables are replaced by their
// values. Variables may occur in all (exported) string fields of Checks and
// all suitable string fields of Request in the form:
//     {{VARNAME}}
// The variable substitution is performed during compilation of a Test which
// includes compilation of the embedded Checks.
//
// The current time with an optional offset can be substituted by a special
// construct:
//     {{NOW}}                       -->  Wed, 01 Oct 2014 12:22:36 CEST
//     {{NOW + 15s}}                 -->  Wed, 01 Oct 2014 12:22:51 CEST
//     {{NOW + 25m | "15:04"}}       -->  12:47
//     {{NOW + 3d | "2006-Jan-02"}}  -->  2014-Oct-04
// Formating the time is done with the usual reference time of package time
// and defaults to RFC1123. Offset can be negative, the known units are "s" for
// seconds, "m" for minutes, "h" for hours and "d" for days.
//
// Some random values can be include by the following syntax:
//     {{RANDOM NUMBER 99}}          -->  22
//     {{RANDOM NUMBER 32-99}}       -->  45
//     {{RANDOM NUMBER 99 %04x}}     -->  002d
//     {{RANDOM TEXT 8}}             -->  que la victoire Accoure à tes mâles
//     {{RANDOM TEXT 2-5}}           -->  Accoure à tes
//     {{RANDOM TEXT de 5}}          -->  Denn die fromme Seele
//     {{RANDOM EMAIL}}              -->  Leon.Schneider@gmail.com
//     {{RANDOM EMAIL web.de}}       -->  Meier.Anna@web.de
//
// Tests
//
// A Test is basically just a Request combined with a list of Checks.
// Running a Test is executing the request and validating the response
// according to the Checks. Before a test can be run the variable substitution
// in the Request and the Checks have to happen, a real HTTP request
// has to be crafted and checks have to be set up. This is done by compiling
// the test, a step which may fail: a) if the Request is malformed (e.g. uses
// a malformed URL) or b) if the checks are malformed (e.g. uses a malformed
// regexp). Such Tests/Checks are labeled Bogus.
//
// There are three ways in which a Tests may fail:
//   1. The test setup is malformed, such tests are called Bogus.
//   2. The request itself fails. This is called an Error
//   3. Any of the checks fail. This is called a Failure
//
//
// Suites of tests
//
// Normaly tests are not run individually but grouped into suites.
// Such suite may share a common cookie jar (and a common logger)
// and may contain setup and teardown actions. Any action on a Suite
// normally requires its setup tests to pass, afterwards the main tests
// are executed and finally the teardown tests are run (but no errors or
// failures are reported for teardown tests).
//
//
package ht
