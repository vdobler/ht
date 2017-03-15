HT: HTTP Testing Made Easy
==========================

[![Go Report Card](https://goreportcard.com/badge/github.com/vdobler/ht)](https://goreportcard.com/report/github.com/vdobler/ht)
[![GoDoc](https://godoc.org/github.com/vdobler/ht?status.svg)](https://godoc.org/github.com/vdobler/ht)
[![Build Status](https://travis-ci.org/vdobler/ht.svg?branch=master)](https://travis-ci.org/vdobler/ht)

End-to-end testing of HTTP requests/responses is easy with Go.

Writing and maintaining high level test is even easier with ht.


Goals
-----

ht tries to achieve the following:
* Make generating all common HTTP requests easy.
* Provide high- and low-level checks on the received response.
* Make measuring response times easy.
* Provide a library/labraries (Go packages) to pragmatically generate requests
  and test the responses.
* Provide a standalone executable which reads requests/checks from disk and
  runs them.
* Provide good diagnostics on the generated request, the received response
  and on potential failed checks for ease of debugging.


Non-Goals
---------

ht is not the jack of all trades in testing web applications:
* Simulating different browsers is not done.
* Simulating interactions in a browser is not done.


Installation
------------

Installing ht should be simple if Go 1.7 and git are available and working:
* Run `go get github.com/vdobler/ht/cmd/ht`
  which should download, compile and install everything.
* Run `$GOPATH/bin/ht help` to get you started.
* For a quick check of a HTML page do a 

    `$GOPATH/bin/ht quick <URL-of-HTML-page>`

  and check the generated `Report.html` file.

If you want to run checks on rendered HTML pages you need a local installation
of (PhantomJS)[http://phantomjs.org] in version >= 2.0.


Nonemclature
------------

The following terms are necessary to know wheter you use ht as a package
or as the executable.

The main concept of `ht` is that of a **`Test`**: A `Test` makes one request and
performs various `Check`s on the received response. If the request worked and
all checks pass the Test is considered as passed.

A **`Check`** is a single property to verify once the request has been made.
It may check simple properties of the response like e.g. "Did we receive
a 201 Status Code?" or "Did we receive the response in less than 200ms?" or
even complex stuff like "What happens to the request if we redo it with
slight changes like different parameters or extra/dropped/modified headers?".

Tests can be grouped into a **`Suite`**. A suite caputres the idea of a sequence
of requests. Tests in a suite may share a common cookie jar so that simmulating
a browser session becomes easy.

A load test is a throughput test which uses a mixture of suites to generate
a distrubution of requests: The load test is a set of `Szenario`s where
each szenario (technically just a suite) contributes a certain percentage to
the whole set of request.

Tests have one of the following status:
 - `NotRun`:  This test has not jet been executed.
 - `Skipped`: Test would have been executed but was skipped deliberately.
 - `Pass`:    Request/response was fine and all checks passed.
 - `Failed`:  Request/response was fine but at least one check failed.
 - `Error`:   Problems receiving the response due to network failures or timeouts.
 - `Bogus`:   The test itself is malformed, e.g. trying to upload a file
              via query parameters. Your tests should not be bogus.


Tests, Checks and Suites
------------------------

The following describes the use of cmd/ht, i.e. the executable and how to
define tests and checks and have them executed. 
All tests, checks, suites and loadtest are stored on disk in
(Hjson)[http://hjson.org] format which is light enough to write and reads
(almost) like a Go struct. Hjson allows comments, quotes and commas are often
optional.

A minimal test to make a GET request to example.org and check for a HTTP
status code of 200 looks like this:

```
// file: minimal.ht
{ 
    Name: Minimal Test
    Request: {
        URL: http://example.org
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
    ]
}
```

A test has more fields and the request field also has much more fields.
Please refer to the built in documentation by running:

    $ ht doc Test
    $ ht doc Request
    $ ht doc Execution
    ...

to see the full documentation.

The following (nonsensical) example show lots of features and syntactical
sugar available when writing tests:

```
// file: swank.ht
{
    Name: Feature Show Off
    Description: '''Multiline strings work well
                    and require less "quoting" than plain JSON.'''
    Request: {
        Method:  POST
        URL:     "http://{{HOST}}/some/path?A=1"  // {{HOST}} is a variable, see below.
        Params:  {
            // 'A' is already part of the URL
            B: "foo"            // single value
            C: [ 3.14, 2.72 ]   // multiple values (duplicated parameter)
        }
        Header: {
            Accept:   xml,html
            X-Custom: [ 345, "ABC" ]
        }
        Body: '''{"value": 9988}'''  // send POST body
        FollowRedirects: "true"  // follow 30x until 'real' response
        BasicAuthUser: "root"    // Convenience: Set proper Authorization: Basic 
        BasicAuthPass: "secret"  // header from username/password.
        Timeout: "2s"    // shorter timeout
        Chunked: "true"  // force chunked POST bodies
    }

    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "None", Of: [  // logical NAND of the following tests
                {Check: "Body", Contains: "something went wrong"}
                {Check: "JSON", Element: "field.2", Contains: "ooops"}
            ]
        }
        {Check: "RedirectChain", Via: [ ".../verifyuser", ".../sso/settoken" ]}
    ]

    Execution: {
        // Wait 100ms before doing actual work in this test, 120ms between
        // receiving the response and execution of the first check and 140ms
        // after the last checks.
        PreSleep:   100ms
        InterSleep: 120ms
        PostSleep:  0.14s

	// Anticipate failures and errors: Retry this test up to 8 times
	// if it fails or errors waiting 1.5 seconds between retries.
        // Test will fail/error if all 8 tries fail/error.
        Tries: 8
        Wait:  1.5s
    }

    // Variables can be set/updated from the received response via
    // several different extractors.
    VarEx: {
        LOGINTOKEN: {Extractor: "JSONExtractor", Element: "0.1.token"}
    }

    // Variables can contain defaults for variable substitutions. These
    // defaults are used only if the value is unset on the surrounding
    // execution context, typically the suite.
    Variables: {
        HOST: "localhost:1234"
    }
}
```

Combine these two test into a suite, e.g. like this

```
// file: suite.suite
{
    Name: Sample Suite
    Description: Demonstration purpose only.
    KeepCookies: true  // behave like a browser

    Setup: [  // failing/errored setup tests will skip main tests
        {File: "minimal.ht"}
    ]

    Main: [
        {File: "swank.ht"}
	// Execute swank ht once more but use localhost:808 as value of HOST variable
        {File: "swank.ht", Variables: {HOST: "localhost:8080"}}
    ]
}
```

Now you can execute the full suite like this:

    # ht exec -vv -output result suite.suite




Documentation
-------------

For a start have a look at the 

Tutorial https://github.com/vdobler/ht/blob/master/cmd/ht/Tutorial.md

or the 

Showcase here https://github.com/vdobler/ht/tree/master/showcase

The showcase is pretty nonsensical but show almost all features
in just a few files.  You might want to `$ go run showcase.go` to
have a dummy server listening on localhost:8080 to run the tests
against: `$ ht exec showcase.suite`

The builting help to cmd/ht is useful too:

    $ ht help checks       #  list short overview of available checks
    $ ht help extractors   #  list short overview of available extractors
    $ ht doc Test          #  show godoc of type Test
    $ ht doc Redirect      #  show godoc of redirect check
    $ ht doc RawTest       #  show details of disk format of a test
    $ ht doc RawSuite      #  show details of disk format of a suite


For details see the the godoc for reference:

* Tests, Checks and their options:
  https://godoc.org/github.com/vdobler/ht/ht
* Image fingerprinting:
  https://godoc.org/github.com/vdobler/ht/fingerprint
* The ht command itself:
  https://godoc.org/github.com/vdobler/ht/cmd/ht
  Run `ht help` for details
* An example test suite can be found in
  https://github.com/vdobler/ht/blob/master/testdata/sample.suite


Copyright
---------

Copyright 2016 Volker Dobler

All rights reserved. Use of this source code is governed by
a BSD-style license that can be found in the LICENSE file.


Attribution
-----------

Ht includes open source from the following sources:

* Go Libraries. Copyright 2012 The Go Authors. Licensed under the BSD license (http://golang.org/LICENSE).
* Bender. Copyright 2014 Pinterest.com. Licensed under the Apache License, Version 2.0
* Cascadia. Copyright (c) 2011 Andy Balholm. Licensed under the BSD 2-clasue license.
* Gojee. Copyright (c) 2013 The New York Times Company. Licensed under the Apache License, Version 2.0
* Otto. Copyright (c) 2012 Robert Krimen. Licensed under the MIT License.
* Xmlpath. Copyright (c) 2013-2014 Canonical Inc. Licensed under the LGPLv3. See https://gopkg.in/xmlpath.v2
* Hjson. Copyright (c) 2016 Christian Zangl. Licensed under the MIT License.
* Sourcemap. Copyright (c) 2016 The github.com/go-sourcemap/sourcemap Contributors. Licensed under the BSD 2-clasue license.
* Go-MySQL-Driver. Copyright 2013 The Go-MySQL-Driver Authors. Licensed under the Mozilla Public License Version 2.0
