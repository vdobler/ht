// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package suite contains functions to input tests from disk and execute suites
of tests in a controled way.


Variable Handling

There are three "scopes" for variables when executing a suite read from disk
(a RawSuite):

 * Global Scope: Variables set from "the outside", typically via the -D
   command line flag to cmd/ht. Executing a suite or tests does not modify
   variables in this scope. This global set of varibales is input to the
   Execute method of RawSuite.

 * Suite Scope: This is the context of the suite, it can be changed
   dynamically via variable extraction from executed tests.
   The initial suite scope is:
     - a copy of the Global Scope, with
     - added COUNTER and RANDOM variables and
     - merged defaults from the suite's Variables section.
   Variable expansion happens inside the values of the Variables section.

 * Test Scope: The tests scope is the context of a test, actually the
   set of variables used to make a ht.Test from a RawTest.
   It is constructed as follows:
     - To a copy of the actual suite scope,
     - the automatic variables COUNTER and RANDOM are added (with new values),
     - merged with the call Variables
     - merged with the tests Variables section.
   Variable expansion happens inside the values of the call and test Variables
   section.

Here "merging" means that the variables in the Variables section are
added to the scope if not already present. I.e. the variables from outer scope
dominate variables from inner scopes.


*/
package suite
