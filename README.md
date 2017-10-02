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
* Provide a standalone executable which reads requests/checks from disk and
  runs them.
* Provide good diagnostics on the generated request, the received response
  and on potential failed checks for ease of debugging.
* Make the tests and checks understandable and readable. Ease of writing is
  a secondary goal only.
* Provide a library (Go packages) to programmatically generate requests
  and test the responses.
* Provide basic functionalities to mock responses.


Non-Goals
---------

ht is not the jack of all trades in testing web applications:
* Simulating different browsers is not done.
* Simulating interactions in a browser is not done.
* Load testing capabilities are very limited.
* Recording click-paths in a web application is not the intended way
  of producing tests.


Installation
------------

Installing ht should be simple if Go 1.8 and git are available and working:
* Run `go get github.com/vdobler/ht/cmd/ht`
  which should download, compile and install everything.
* Run `$GOPATH/bin/ht help` to get you started.
* For a quick check of a HTML page do a 

    `$GOPATH/bin/ht quick <URL-of-HTML-page>`

  and check the generated `_Report_.html` file.

If you want to run checks on rendered HTML pages you need a local installation
of (PhantomJS)[http://phantomjs.org] in version >= 2.0.

(Ht vendors it dependencies _except_ dependencies from golang.org/x/...).


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
a distribution of requests: The load test is a set of **`Scenarios`** where
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
   as legal. There must be at least 4 of them with the given text content
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

    $ ht exec -vv -output result suite.suite

The `exec` subcommand allows very fine control over the execution of the
suites/tests. Please consult `ht help exec` for a description of the command
line flags.


Graphical User Interface
------------------------

The Tests in Hjson format are easy to read (at least that was the intention).
But they can be hard to write as you have to know what types of Checks are
available and which options control their behaviour. Debugging is hard too.

To faciliate this cmd/ht offers a very simple GUI to craft and debug Test,
Checks and Variable Extractions. The GUI provides tooltips to all types, fields
and options and allows to execute Tests and dry run Checks and Extractions.

    $ ht gui

Please refer to the help (`ht help gui`) for more details.


How to write proper tests?
--------------------------

The main starting point to write proper automated test with h is to have a
good model of how your application should work, what it is expected to do:
What is the proper response to a certain stimulus, a certain a request?
(A simple "If I click here and there and enter this an click that button
then I see the result" is not enough.)

Let's assume you have to check that the Search functionality "works properly".
Break down this generic "works properly" into testable parts, e.g. like this:

 * Is the search box present in the HTML of the homepage?
 * Will submitting the form trigger a request to the correct URL?
 * Does the search answer in reasonable amout of time, no matter how
   many results are found?
 * Does a search display the expected hits in the expected order?

Tests for this could look like this:

```
// file: searchbox.ht
{
    Name: Look for Searchbox on Homepage
    Request: {
        URL: https://www.example.org/homepage
    }
    Checks: [
        // Start with two basic checks to make sure we got a sensible response.
        {Check: "StatusCode", Expect: 200}
        {Check: "ContentType", Is: "text/html"}

        // Start looking for the search box. Become more and more specific,
	// this makes tracking problems easier.
	{Check: "HTMLTag", Selector: "form.search"}
	{Check: "HTMLTag", Selector: "form.search input[type=search\"]"}
	{Check: "HTMLTag", Selector: "form.search input[type=search][placeholder=\"search website\"]"}

	// Check target of this form: Must hit version 2 of search:
	{Check: "HTMLTag", Selector: "form.search[action=\"/search_v2/\"]"}
    ]
}
```

Speed of search must be tested for different query terms, so make the query
a variable:

```
# searchspeed.ht
{
    Name: Search is fast
    Request: {
        URL: https://www.example.org/search_v2
        Params: {
            // This test can be parametrised with different query arguments.
            q: "{{QUERY}}"
        }
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "ResponseTime", Lower: "1.1s"}
    ]
}
```

To test the results of a search it is convenient to have magic phrase which is
used only in controlled places with: All these places (and none else) should
turn up in the lsit of search results.

```
# searchresults.ht
{
    Name: Search produces sorted results
    Request: {
        URL: https://www.example.org/search_v2
        Params: {
            q: "open sesame"  // has defined usage on site
        }
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: HTMLContains
            Selector: ".results li.hit span.title"
            Complete: true, InOrder: true
            Text: [
                "Ali Baba"
                "Sample Document 1 (indexable)"
                "Public Information 3"
            ]
        }
    ]
}
```

Combine these into a suite:

```
# search.suite
{
    Name: Test basic website search

    Main: [
        {File: "searchbox.ht"}
        {File: "searchspeed.ht", Variables: {QUERY: ""}}
        {File: "searchspeed.ht", Variables: {QUERY: "blaxyz"}}      // no search result
        {File: "searchspeed.ht", Variables: {QUERY: "open sesame"}} // exaclty 3 results
        {File: "searchspeed.ht", Variables: {QUERY: "product"}}     // lots of results
        {File: "searchspeed.ht", Variables: {QUERY: "name"}}        // hundreds of results
        {File: "searchresults.ht"}
    ]
}
```

You can store these four files individually or inside an "archive file" like
[this one](https://github.com/vdobler/ht/blob/master/search.archive). Executing a
suite inside an archive works like `ht exec search.suite@search.archive`.


Documentation
-------------

### Examples

Ht comes with a broad set of examples built in.
To list all available topics or all available examples run:

    $ ht example
    $ ht example -list

The examples are organised hierarchically: To learn how to write a Test start
of with an example of a generic Test:

    $ ht example Test

and work your way through the available sub-topics, e.g. 

    $ ht example Test.POST
    $ ht example Test.POST.FileUpload

The examples are executed during `go test github.com/vdobler/ht/cmd/ht`.
The output reports are written to the folder example-tests and provide
additional insight so try the exampels and take a look at their output.


### Builtin reference

Ht has a `doc` subcommand which shows reference documentation for the types
and fields used to construct Tests and Checks. Use this to find out all the
details not explained in the examples

    $ ht doc Test          #  show godoc of type Test
    $ ht doc Request       #  show godoc of type Request
    $ ht doc Redirect      #  show godoc of Redirect check
    $ ht doc JSON          #  show godoc of JSON check

This doc subcommand (as well as the exaple subcommand) does not rely on the
Go tools or the source of ht beeing installed and can be used from the Docker
image.


### Help subcommand

The `help` subcommand explains the available subcommands and their command
line flags. It can be used to list all available Checks and data Extractors:

    $ ht help exec         #  describe how to use the exec subcommand
    $ ht help checks       #  list short overview of available checks
    $ ht help extractors   #  list short overview of available extractors


### Normative references

The `ht doc` output is generated from the normative source code and can serve
as the go to reference for most questions. For more detailed stuff see the
godoc of the individual packages, especially

* Tests, Checks and their options:
  https://godoc.org/github.com/vdobler/ht/ht
* Image fingerprinting:
  https://godoc.org/github.com/vdobler/ht/fingerprint
* Suites and Loadtesting:
  https://godoc.org/github.com/vdobler/ht/suite


### Outdated stuff (which might be still helpful)

Both the 
  *  Tutorial  https://github.com/vdobler/ht/blob/master/TUTORIAL.md and the
  *  Showcase  https://github.com/vdobler/ht/tree/master/showcase
are no longer maintained. Use the `ht example <topic>` feature.

The showcase is pretty nonsensical but show almost all features
in just a few files.  You might want to `$ go run showcase.go` to
have a dummy server listening on localhost:8080 to run the tests
against: `$ ht exec showcase.suite`



How does it compare to ...
--------------------------

In direct comparison to specialized tools like Selenium, Scalatest, JMeter,
httpexpect, Gatlin etc. pp. ht will lack. Ht's ability to drive a headless browser
is totally inferour to Selenium/Webdriver/YourFavoriteTool. Ht's capabilities
in generating load is far worse than JMeter or Gatlin.

Ht's strength lies in providing high-level checks suitable to detect problems
in HTTP based applications combined with good diagnostics which allow for
efficient debugging of such tests.


Is it any good?
---------------

Yes


Copyright
---------

Copyright 2017 Volker Dobler

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
