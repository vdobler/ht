// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ht provides functions for easy testing of HTTP based protocols.
//
// Testing is done by constructing a request, executing the request and
// performing various checks on the returned response. The type Test captures
// this idea. Each Test may contain an arbitrary list of Checks which
// perform the actual validation work. Tests can be grouped into Suites
// which provide a common cookie jar for their tests and may execute setup
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
//     BodyContains{Text: "foobar"}
//     BodyContains{Text: "foobar", Count: 1}
//     BodyContains{Text: "illegal", Count: -1}
// The last three examples show how zero values of optional fields are
// commonly used: The zero value of Count means "any number of occurences".
// Forbidding the occurenc of "foobar" thus requires a negative Count.
//
// The following checks are provided
//     * StatusCode        checks the received http status code
//     * ResponseTime      provides lower and higer bounds on the response time
//     * BodyContains      checks for textual occurences in the HTTP body
//     * BodyMatch         regular expression matching of the HTTP body
//     * ValidHTML         checks if body parses as HTML5
//     * HTMLContains      checks occurence HTML elements choosen via CSS-selectors
//     * HTMLContainsText  checks text content of CSS-selected elements
//     * Image             checks image format and size
//     * Header            checks presence and values of received HTTP header
//     * SetCookie         checks properties of received cookies
//     * JSON              checks structure and content of a JSON body
// Upcomming checks (TODO):
//     * XML               some kind of XML checking
//     * LinkValidation    make sure hrefs and srcs in HTML are accesible
//     * LogFile           limit grows and check content of log output
//
// Requests
//
// Requests allow to specify a HTTP request in a declarative manner.
// A wide varity of request can be generated from a purly textual
// representation of the Request.
//
// All the ugly stuff like parameter encoding, generation of multipart
// bodies, etc. are hidden from the user.
//
// Parametrisations
//
// Hardcoding e.g. the hostname in a test has obvious drawbacks. To overcome
// these the Request and Checks may be parametrised. This is done by a simple
// variable expansion in which occurences of variables are replaced by their
// values. Variables may occur in all (exported) string fields of Checks and
// all suitable string fields of Request in the form:
//     {{VARNAME}}
// The variable substitution is performed during compilation of a Test which
// includes compilation of the embeded Checks.
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
// TODO: Add integer substitutions.
// TODO: Add {RAND,UNIQUE}{INT,ID,WORDS} or something similiar.
//
// Tests
//
// A Test is basically just a Request combined with a list of Checks.
// Running a Test is executing the request and validating the response
// according to the Checks. Before a test can be run the variable substutution
// in the Request and the Checks have to happen, a real HTTP request
// has to be crafted and checks have to be set up. This is done by compiling
// the test, a step wich may fail: a) if the Request is malformed (e.g. uses
// a malformed URL) or b) if the checks are malformed (e.g. uses a malformed
// regexp). Such Tests/Checks are labeled Bogus.
//
// There are three ways in which a Tests may fail:
//   1. The test setup is malformed, such tests are called Bogus.
//   2. The request itself fails. This is called an Error
//   3. Any of the checks fail. This is called a Failure
//
// Unrolling a Test
//
// A common szenario is to do a test/check combination several times
// with tiny changes, e.g. a search with different queries. To facilliate
// writing these repeated tests it is possible to treat a Test as a
// template which is instantiated with different parametrizations.
// This process is called unrolling. The field UnrollWith of a test
// controlls this unrolling: It is a map of variabe names to variable
// values. The simplest definition is
//     UnrollWith: map[string][]string{"query": {"foo", "bar", "wuz"}}
// with the test and probably the checks too containing references
// to the query variabel like "{{query}}". Unrolling such a test produces
// three different, new test, one with all occurences of "{{query}}"
// replaced by "foo", one with "bar" as the replacement and a third
// with "wuz". The unrolled tests do no longer contain the "{{query}}"
// variabel. If more than one variable is used during unrolling the
// situation is simple if both value sets have the same size: Variable
// substitution will use the first values first, then the second values
// and so on. If the variable have different length value sets e.g.
//     UnrollWith: map[string][]string{
//         "a": {"1", "2", "3"},
//         "b": {"x", "y"},
//     }
// one would get 6 = 3*2 = the least common multiple of all value set length
// tests wit the first test having (a=1 b=x),the second one (a=2, b=y), the
// third one (a=3 b=x), and so on until the last one which has (a=3 b=y).
//
// It is important to understand that Unrolling a Test produces several
// distinct Tests.
//
// Suites of tests
//
// Normaly tests are not run individually but grouped into suites.
// Such suite may share a common cookie jar (and a common logger)
// and may contain setup and teardown actions. Any action on a Suite
// normaly requires its setup tests to pass, afterwards the main tests
// are executed and finaly the teardown tests are run (but no errors or
// failures are reported for teardown tests).
//
//
package ht
