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
* Provide a library (Go packages) to programmatically generate requests
  and test the responses.
* Provide a standalone executable which reads requests/checks from disk and
  runs them.
* Provide good diagnostics on the generated request, the received response
  and on potential failed checks for ease of debugging.
* Provide basic functionalities to mock responses.


Non-Goals
---------

ht is not the jack of all trades in testing web applications:
* Simulating different browsers is not done.
* Simulating interactions in a browser is not done.
* Load testing capabilities are very limited.


Installation
------------

Installing ht should be simple if Go 1.8 and git are available and working:
* Run `go get github.com/vdobler/ht/cmd/ht`
  which should download, compile and install everything.
* Run `$GOPATH/bin/ht help` to get you started.
* For a quick check of a HTML page do a 

    `$GOPATH/bin/ht quick <URL-of-HTML-page>`

  and check the generated `Report.html` file.

If you want to run checks on rendered HTML pages you need a local installation
of (PhantomJS)[http://phantomjs.org] in version >= 2.0.


Nomenclature
------------

The following terms are necessary to know and understand regardless of whether
you use ht as a package or as the executable.

The main concept of `ht` is that of a **`Test`**: A `Test` makes one request and
performs various `Check`s on the received response. If the request worked and
all checks pass the Test is considered as passed.

A **`Check`** is a single property to verify once the request has been made.
It may check simple properties of the response like e.g. "Did we receive
a 201 Status Code?" or "Did we receive the response in less than 200ms?" or
even complex stuff like "What happens to the request if we redo it with
slight changes like different parameters or extra/dropped/modified headers?".

Tests can be grouped into a **`Suite`**. A suite captures the idea of a sequence
of requests. Tests in a suite may share a common cookie jar so that simulating
a browser session becomes easy.

A load test is a throughput test which uses a mixture of suites to generate
a distribution of requests: The load test is a set of **`Scenario`**s where
each scenario (technically just a suite) contributes a certain percentage to
the whole set of request.

Tests have one of the following status:
 - `NotRun`:  This test has not jet been executed.
 - `Skipped`: Test would have been executed but was skipped deliberately.
 - `Pass`:    Request/response was fine and all checks passed.
 - `Failed`:  Request/response was fine but at least one check failed.
 - `Error`:   Problems receiving the response due to network failures or timeouts.
 - `Bogus`:   The test itself is malformed, e.g. trying to upload a file
              via query parameters. Your tests should not be bogus.

Often a Test needs to be parametrized, e.g. to execute it against different
systems. This is done through **`Variables`**. Variable substitution is a plain
textual replacement of strings of the form `{{VariableName}}` with a value.
Such variables can be set in the Test, during inclusion of a Test in a Suite
or during execution of cmd/ht. Some variables are provided by the system itself.

Variable values can be set from data extracted from previous responses through
**`Extractors`**. These are able to extract information from cookies, HTTP
headers, JSON documents or via regular expression from a HTTP response body.
Variable extraction and substitution in subsequent requests allows to build
test Suites which validate complex business processes.


Tests, Checks and Suites
------------------------

The following describes the use of cmd/ht, i.e. the executable and how to
define tests and checks and have them executed. 
All tests, checks, suites and loadtests are stored on disk in
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

The following example show some more features and syntactical sugar available
when writing tests:

```
// file: example.ht
{ 
    Name: Example of more features
    Request: {
        Method: GET
        URL:    https://{{HOST}}/some/path
        Params: {
            query: "statement of work"
        }
        Header: {
            Accept:   text/html
        }
        FollowRedirects: true
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "ContentType", Is: "text/html"}
        {Check: "UTF8Encoded"}
        {Check: "ETag"}
        {Check: "ValidHTML"}
        {Check: "HTMLContains"
            Selector: h3.legal
            Text: [ "Intro", "Scope", "Duration", "Fees" ]
            InOrder: true
        }
    ]
    Execution: { 
        Tries: 3
        Wait:  1.5s
    }
    Variables: {
        HOST: "localhost:1234"
    }
}
```

Here the URL contains a variable `{{HOST}}`: When the test is read in from disk
this placeholder is replaced by the current value of the HOST variable. HOST
defaults to "localhost:1234". This test is retried two more times if it fails
with a pause of 1.5 seconds between retreies.

Sending query parameters is dead simple, encoding is done automatic. Sending
parameters in a POST request as application/x-www-form-urlencoded or as
multipart/form-data is possible too (see "ParamsAS" in `ht doc Request`).

The checks consist of simple and complex checks.

 * HTTP Status code is compared against the expected value of 200
 * The Content-Type header is checked. This check is not a simple string
   comparison of a HTTP header but realy parses the Content-Type header:
   A `Content-Type: text/html; charset=iso_8859-1` would fulfill this check too.
 * Proper encoding of the HTTP body is checked.
 * Existence and proper handling of a ETag header is checked: This check
   not only checks for the presence of a ETag header but will also issue a
   _second_ request to the URL of the Test with a `If-None-Match` HTTP header
   and check that the second request results in a 304.
 * ValidHTML takes a coars look at a HTML document and reports simple
   problems like duplicate tag IDs.
 * The last check interpretes the HTML and looks for h3 tags classified
   as legal. There must be at least 4 of themwith the given text content
   in the given order (but there might be more).


Combine these two test into a suite, e.g. like this

```
// file: suite.suite
{
    Name: Sample Suite
    Description: Demonstration purpose only.
    KeepCookies: true  // behave like a browser

    // Failing/errored Setup Tests will skip main Tests.
    Setup: [  
        {File: "minimal.ht"}
    ]

    Main: [
        {File: "example.ht"}
        {File: "example.ht", Variables: {HOST: "localhost:8080"}}
    ]
}
```

The `example.ht` Tests is included twice: In the second invocation the value
of the HOST variable will be "localhost:8080".

Now you can execute the full suite like this:

    # ht exec -vv -output result suite.suite



Graphical User Interface
------------------------

The Tests in Hjson format are easy to read (at least that was the intention).
But they can be hard to write as you have to know what types of Checks are
available and which options control their behaviour. Debugging is hard too.

To faciliate this cmd/ht offers a very simple GUI to craft and debug Test,
Checks and Variable Extractions. The GUI provides tooltips to all types, fields
and options and allows to execute Tests and dry run Checks and Extractions.

    # ht gui

Please refer to the help (`ht help gui`) for more details.




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

The builtin help to cmd/ht is useful too:

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


How does it compare to ...
--------------------------

In direct comparison to specialiesd tools like Selenium, Scalatest, JMeter,
httpexpect, Gatlin etc. pp. ht will lack. Ht's ability to drive a headless browser
is totally inferiour to Selenium/Webdriver/YourFaviriteTool. Ht's capabilities
in generating load is far worse than JMeter or Gatlin.

Ht's strength lies in providing high-level checks suitable to detect problems
in HTTP based applications combined with good diagnostics which allow for
efficient debugging of such tests.


Is it any good?
---------------

Yes


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
* Cascadia. Copyright (c) 2011 Andy Balholm. Licensed under the BSD 2-clause license.
* Gojee. Copyright (c) 2013 The New York Times Company. Licensed under the Apache License, Version 2.0
* Otto. Copyright (c) 2012 Robert Krimen. Licensed under the MIT License.
* Xmlpath. Copyright (c) 2013-2014 Canonical Inc. Licensed under the LGPLv3. See https://gopkg.in/xmlpath.v2
* Hjson. Copyright (c) 2016 Christian Zangl. Licensed under the MIT License.
* Sourcemap. Copyright (c) 2016 The github.com/go-sourcemap/sourcemap Contributors. Licensed under the BSD 2-clasue license.
* Go-MySQL-Driver. Copyright 2013 The Go-MySQL-Driver Authors. Licensed under the Mozilla Public License Version 2.0
* Govalidator. Copyright (c) 2014 Alex Saskevich. Licensed under the MIT License.
* Gorilla. Copyright (c) 2012 Rodrigo Moraes. Licensed under the New BSD License
