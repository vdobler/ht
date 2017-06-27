// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ht provides functions for easy testing of HTTP based protocols.
//
// Testing is done by constructing a request, executing the request and
// performing various checks on the returned response. The type Test captures
// this idea. Each Test may contain an arbitrary list of Checks which
// perform the actual validation work. Tests can be grouped into Suites
// which may provide a common cookie jar for their tests
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
//     * Cache           Cache-Control header
//     * ContentType     Content-Type header
//     * CustomJS        performed by your own JavaScript code
//     * DeleteCookie    for proper deletion of cookies
//     * ETag            presence of working ETag header
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
// The other status of a Test are NotRun for a not jet executed test, Skipped
// for a deliberately skipped test and Pass for a test passing all checks.
//
// Tests can be retried for a given maximum number of times. If all executions
// fail the tests fails. This behaviour and detail timing can be controled with
// Test's Execution field.
//
// Sometimes a response provides information necessary for subsequent requests.
// Cookie handling can be delegated to a cookie jar by providing all Tests with
// the same instance of the Jar.
// Other values can be extracted from the response via a set of Extractors:
//   * BodyExtractor    via regular expression from body
//   * CookieExtractor  a cookie value
//   * HTMLExtractor    value of a HTML attribute or HTML text
//   * JSExtractor      custom via interpreded JavaScript script
//   * JSONExtractor    from a JSON document
//   * SetVariable      not extracted but set manually
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
// Pseudo Request
//
// Ht allows to make several types of pseudo request: A request which is not a
// HTTP1.1 request but generates output which can be checked via the existing
// checks. The schema of the Test.Request.URL determines the request type.
// Normal HTTP request are made with the two schemas "http" and "https".
// Additionally the following types of pseudo request are available:
//   * file://
//       This type of pseudo request can be used to read, write and delete
//       a file on the filesystem
//   * bash://
//       This type of pseudo request executes a bash script and captures its
//       output as the response.
//   * sql://
//       This type of pseudo request executes a database query (using package
//       database/sql.
//
//
// File Pseudo-Requests
//
// File pseudo-requests are initiated with a file:// URL, the following rules
// determine the behaviour:
//   * The GET request method tries to read the file given by the URL.Path
//     and returns the content as the response body.
//   * The PUT requets method tries to store the Request.Body under the
//     path given by URL.Path.
//   * The DELETE request method tries to delete the file given by the
//     URL.Path.
//   * The returned HTTP status code is 200 except if any file operation
//     fails in which the Test has status Error.
//
//
// Bash Pseudo-Requests
//
// A bash pseudo-request is initated with a bash:// URL, the following rules
// apply:
//    * The script is provided in the Request.Body
//    * The working directory is taken to be URL.Path
//    * Environment is populated from Request.Params
//    * The Request.Method and the Request.Header are ignored.
//    * The script execution is canceled after Request.Timout (or the
//      default timeout).
// The outcome is encoded as follows:
//   * The combined output (stdout and stderr) of the script is returned
//     as the response body (Response.BodyStr).
//   * The HTTP status code is
//        - 200 if the script's exit code is 0.
//        - 408 if the script was canceled due to timeout
//        - 500 if the exit code is != 0.
//   * The Response.Header["Exit-Status"] is used to return the exit
//     status in case of 200 and 500 (success and failure).
//
//
// SQL Pseudo-Requests
//
// SQL pseudorequest are initiated via sql:// URLs and come in the two flavours
// query to select rows and execute to execute other SQL stuff.
//    * The database driver is selected via URL.Host
//    * The data source name is taken from Header["Data-Source-Name"]
//    * The SQL query/statements is read from the Request.Body
//    * For a POST method the SQL query is passed to sql.Execute
//      and the response body is a JSON with LastInsertId and RowsAffected.
//    * For a GET method the SQL query is passed to sql.Query
//      and the resulting rows are returned as the response body.
//    * The format of the response body is determined by the Accept header:
//         - "application/json":         a JSON array with the rows as objects
//         - "text/csv; header=present": as a csv file with column headers
//         - "text/csv":                 as a csv file withput header
//         - "text/plain":               plain text file columns separated by \t
//         - "text/plain; fieldsep=X":   plain text file columns separated by X
//     The result if the query is returned in the Response.BodyStr
//
//
// Rendered Webpages
//
// Ht contains several checks which allow to interpret HTML pages like a
// browser does. For these checks ht relies on PhantomJS in version 2:
//   * Screenshot
//   * RenderedHTML
//   * RenderingTime
//
//
package ht
