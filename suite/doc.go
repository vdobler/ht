// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package suite allows to read tests and collections of tests (suites) from
// disk and execute them in a controlled way or run throughput load test from
// these test/suites.
//
//
// Variable Handling
//
// There are three "scopes" for variables when executing a suite read from disk
// (a RawSuite):
//
//  * Global Scope: Variables set from "the outside", typically via the -D
//    command line flag to cmd/ht. Executing a suite or tests does not modify
//    variables in this scope. This global set of varibales is input to the
//    Execute method of RawSuite.
//
//  * Suite Scope: This is the context of the suite, it can be changed
//    dynamically via variable extraction from executed tests.
//    The initial suite scope is:
//      - a copy of the Global Scope, with
//      - added COUNTER and RANDOM variables and
//      - merged defaults from the suite's Variables section.
//    Variable expansion happens inside the values of the Variables section.
//
//  * Test Scope: The tests scope is the context of a test, actually the
//    set of variables used to make a ht.Test from a RawTest.
//    It is constructed as follows:
//      - To a copy of the actual suite scope,
//      - the automatic variables COUNTER and RANDOM are added (with new values),
//      - merged with the call Variables
//      - merged with the tests Variables section.
//    Variable expansion happens inside the values of the call and test Variables
//    section.
//
// Here "merging" means that the variables in the Variables section are
// added to the scope if not already present. I.e. the variables from outer scope
// dominate variables from inner scopes.
//
// It might be helpful to think from the inside out: Given a test on disk
//     {
//         Request: { URL: "{{A}}{{B}}{{C}}{{D}}" }
//         Variables: {
//             A: "http://"
//             B: "example.org"
//             C: "/path"
//             D: "?query"
//         }
//     }
// which provides defaults for A, B, C and D which are used unless set in
// some outer scope. If this test is used in a suite
//     {
//         Main: [
//             {File: "test.ht" }
//             {File: "test.ht", Variables: { D: "?other" }
//         ]
//         Variables: {
//             A: "https://"
//         }
//     }
// Then A will be "https" unless set in some outer scope and D will be the
// test-default "?query" once and "?other" in the second invocation.
// If such a suite is called with
//     A == "ftp://"
//     B == "localhost"
//     D == "?none"
// in the global scope (e.g. by running cmd/ht with -D B=localhost) then these
// values will dominate any default from the various Variables sections.
//
package suite
