// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/vdobler/ht/ht"
)

var cmdHelp = &Command{
	RunArgs:     runHelp,
	Usage:       "help [command | topic]",
	Description: "print help information",
	Flag:        flag.NewFlagSet("help", flag.ContinueOnError),
	Help: `
Help shows help for ht as well as for the different commands and selected
topics. The available help topics are:
    checks       displays the list of builtin checks
    extractors   displays the builtin variable extractors
    archive      explain archive files
    examples     display example suites and tests
`,
}

func runHelp(cmd *Command, args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(0)
	}

	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	arg := args[0]

	// Special case of list of checks/extractors and archives
	switch arg {
	case "check", "checks", "extractor", "extractors":
		displayChecksOrExtractors(arg)
		os.Exit(0)
	case "archive", "archives":
		displayArchiveHelp()
		os.Exit(0)
	case "example", "exampless":
		displayExamples()
		os.Exit(0)
	}

	for _, cmd := range commands {
		if cmd.Name() == arg {
			fmt.Printf(`Usage:

    ht %s
%s
Flags:
`, cmd.Usage, cmd.Help)
			cmd.Flag.PrintDefaults()
			os.Exit(0)
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown help topic %#q.  Run 'ht help'.\n", arg)
	os.Exit(9) // failed at 'go help cmd'

}

func displayChecksOrExtractors(which string) {
	names := []string{}
	if which[0] == 'c' {
		for name := range ht.CheckRegistry {
			names = append(names, name)
		}
	} else {
		for name := range ht.ExtractorRegistry {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	fmt.Println(strings.Join(names, "\n"))
}

func displayArchiveHelp() {
	fmt.Println(`
Several commands accept archive files which combine everything into one
large file. Such an archive file consists of the concatenation of the
different Hjson documents where each document is preceeded by a comment
stating the filename of the document. Any Hjson document can be accessed
with the syntax <filename>@<archive>. Inside the archive the plain filename
is sufficient.

The following example demonstrates using this feature:

    $ cat archive
    # some.suite
    {
        Name: "Some Suite"
        Main: [ {File: sometest.ht}, {File: other.ht} ]
    }

    # sometest.ht
    {
        Request: { URL: "http://localhost/foo" }
        Checks: [ {Check: "StatusCode", Expect: 200} ]
    }

    # other.ht
    {
        Request: { URL: "http://localhost/foo" }
        Checks: [ {Check: "StatusCode", Expect: 505} ]
    }

    $ ht exec some.suite@archive
`)
}

var helpExamples = []string{
	`## A Basic Suite
## =============
{
    Name: "Basic Suite"
    Description: "Show main features and basic usage"
    KeepCookies: true   #  Tests below share a common cookie jar
    OmitChecks:  false
    Verbosity:   2

    # Provide default values for variables. These variables will be present
    # during test execution with the given values. The values can be reset
    # for individual tests executions.
    # Variable values provided on the command line (e.g. via -D FOO=top)
    # take precedence over the defaults here.
    Variables: {
        VARNAME: "varvalue"
        FOO:     "waz"
    }

    # If any setup test failes the suite skips the remaining setup and all
    # main tests and starts executing the teardown tests.
    Setup: [
        { File: "a.ht" }               #  test file a.ht is searched in working dir of ht
        { File: "../rel/path/c.ht" }   #  relative paths are okay
        { File: "/abs/path/b.ht" }     #  absolute paths are okay
        # The variable SUITE_DIR is the directory from which this suite was loaded
        { File: "{{SUITE_DIR}}/c.ht" } #  relative to suite is the most common case
    ]

    # The main test are all executed (if setup passed).
    Main: [
        # Execute the test d.ht with variable FOO set to "bar".
        {File: "d.ht", Variables: {"FOO": "bar"}}

        # Provide mocks m1 and m2 before executing e.ht.
        {File: "e.ht", Mocks: ["m1.mock", "m2.mock"]}

        # Tests can be put inline into the suite. Such inline tests cannot
        # be reused in other suites.
        {Test: {
            Name: "Google Homepage"
            Request: {
                URL: "http://www.google.com/"
                FollowRedirects: true
            }
            Checks: [
                {Check: "StatusCode", Expect: 200}
            ]
        }}
    ]

    # The teardown tests are executed and reported but have no influence on
    # the status of the suite: They may all fail
    Teardown: [
        {File: "x.ht"}
        {File: "y.ht"}
    ]
}
`,

	`## A Basic Test
## ============
{
    Name: "Basic Test"
    Description: "A POST request, redirections and a final page"

    # Mixins allow to mix in common test fragment into this test.
    Mixin: [ "german-chrome.mixin" ]

    # The most important part, the request.
    Request: {
        Method:   "POST"    #  default is GET
        URL:      "http://www.host.org/some/path?foo=bar&waz=1"

        # Parameters can be but into the URL manually (see above) or ht can
        # handle parameters found in Params below
        Params: {
            when:   "today"            #  simple parameters
            primes: [ "2", "7", "13"]  #  multiple values are possible
            info:   "A<?>B €"          #  proper encoding is done automatically
            # File uploads are possible. Read blob.json from same dir where
            # this test is located. Perform variable substitution in blob.json.
            # To inclide blob.json "as is" use @file:
            file:   "@vfile:{{TEST_DIR}}/blob.json"          
        }
        # HTTP allows to sent parameters in the URL, or in the body or as multipart:
        ParamsAs: "multipart"  #  will result in ultipart/form-data

        # Make the request and do follow redirections to the final URL.
        FollowRedirects: true
    }

    Checks: [
        // We expect status OK as we followed the redirections
        {Check: "StatusCode", Expect: 200} 
        {Check: "ContentType", Is: "html"} // short for text/html
        {Check: "UTF8Encoded"}             // what else
    ]
}
`,
	`## A Mixin
## ========
{
    // This is a mixin "test": It cannot be executed (e.g. it is missing a URL)
    // but it defines some common headers sent from a Chrome browser.
    // Other (real) tests can include these headers via
    //   Mixin: [ "german-chrome.mixin" ]
    Name: "German-Chrome",
    Description: "Some headers of a German Chrome Browser",
    Request: {
        Header: {
            User-Agent: "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2272.101 Safari/537.36",
            Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"
            Accept-Language: "de-DE,de;q=0.8,en-US;q=0.6,en;q=0.4,fr;q=0.2"
            Accept-Encoding: "gzip, deflate, sdch"
        }
    }

    # Mixins are very flexible, they can even contain a list of checks
    # which get added to the test this mixin is mixed into.
}
`}

func displayExamples() {
	for i, example := range helpExamples {
		if i > 0 {
			fmt.Println()
			fmt.Println()
		}
		fmt.Println(example)
	}
}
