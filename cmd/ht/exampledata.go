// generated by go run genexample.go; DO NOT EDIT

package main

var RootExample = &Example{
	Name:        "",
	Description: "",
	Data:        ``,
	Sub: []*Example{
		&Example{
			Name:        "Mixin",
			Description: "Mixins allow to add stuff to a Test",
			Data: `// Mixins allow to add stuff to a Test
{
    // A Mixin is technically a Test, it has the same fields. But it is
    // not executed directly but mixed into a real Tests to add common
    // stuff like headers or even Checks like this:
    // {
    //     Name: "Some Test"
    //     Mixin: [ "Mixin" ]  // Load this mixin here.
    //     Request: { URL: "http://example.org" }
    // }
    // Mixins are merged into the test through complicated rules.
    // Consult https://godoc.org/github.com/vdobler/ht/ht#Merge for details.

    Name: "German-Chrome"
    Description: "Some headers of a German Chrome Browser"
    Request: {
        Header: {
            User-Agent: "Chrome/41.0.2272.101"
            Accept: "text/html,application/xml;q=0.9,image/webp,*/*;q=0.8"
            Accept-Language: "de-DE,de;q=0.8,en-US;q=0.6,en;q=0.4,fr;q=0.2"
            Accept-Encoding: "gzip, deflate, sdch"
        }
    }
}`,
			Sub: []*Example{
				&Example{
					Name:        "Mixin.Checks",
					Description: "A Mixin consisting for adding Checks to a Test",
					Data: `// A Mixin consisting for adding Checks to a Test
{
    Name: "Standard Checks on HTML pages"
    // Such a mixin makes it easy to have a default set of checks every
    // HTML document is supposed to fulfill.
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "ResponseTime", Lower: "700ms"}
        {Check: "ContentType", Is: "text/html"}
        {Check: "UTF8Encoded"}
        {Check: "ValidHTML"}
    ]
}`,
				}},
		}, &Example{
			Name:        "Suite",
			Description: "A generic Suite",
			Data: `// A generic Suite
{
    Name: "A generic Suite"
    Description: "Explain the Setup, Main, Teardown and Variables fields."
    KeepCookies: true  // Like a browser does, useful for sessions.
    OmitChecks: false  // No, we want the checks to be executed!

    // Setup and Main are the set of Tests executed and considered relevant
    // for this suite's success. The difference is how Tests with failures or
    // error impact suite execution:
    // If any (executed) test in Setup failes, errors (or is bogus) the suite
    // termiantes and none of the following tests from Setup, none from Main
    // and none from Teardown are executed.
    Setup: [
        { File: "Test" }
    ]

    // Main Tests are executed if all Setup Tests pass. In which case all
    // Tests are executed.
    Main: [
        {File: "Test.JSON"}
        {File: "Test.HTML"}
    ]

    // Teardown Tests are executed after Main (i.e. only if all Setup Tests
    // did pass). Their outcome is reported butdo not influence the Suite
    // status: All teardown tests may fail and the Suite still can pass.
    Teardown: [
       {File: "Test.Image"}
    ]

    // Variables provides a set of variables for the individual Tests.
    // variables defined here in the suite overwrite defaults set in the
    // individual Test files. The values here are overwritten byvalues set
    // on the command line.
    Variables: {
       VARNAME: "varvalue"
    }
}`,
			Sub: []*Example{
				&Example{
					Name:        "Suite.InlineTest",
					Description: "Inline Tests in a suite",
					Data: `// Inline Tests in a suite
{
    Name: "Suite with inline tests"
    Main: [
        // Most often a Suite reference a Test stored in a separate file.
        {File: "Test.JSON"}
            
        // But a Test may be included directly into a Suite. (Drawback:
        // such an inline test cannot be reused in a different suite).
        {Test: {
                   Name: "Test of HTML page"
                   Request: { URL: "http://{{HOST}}/html" }
                   Checks: [ {Check: "StatusCode", Expect: 200} ]
               }           
        }
    ]
    // Works the same for Setup and Teardown Tests.
}`,
				}, &Example{
					Name:        "Suite.Variables",
					Description: "Passing variables to Tests, parameterization of Tests",
					Data: `// Passing variables to Tests, parameterization of Tests
{
    Name: "Suite and Variables"

    Main: [
        // Without Variables field: Test.JSON uses it's own defaults if not
        // overwritten from the Suite below if not overwritten from the
        // command line:
        {File: "Test.JSON"}  // FOO == 9876 from below

        // Tests can be called with different values and/or new variabels.
        // In this instance of Test.JSON:  FOO==1234 && BAR=="some other value"
        {File: "Test.JSON"
            Variables: {
                FOO: 1234
                BAR: "some other value"
            }
        }
    ]

    Variables: {
        FOO: 9876
    }
}`,
				}},
		}, &Example{
			Name:        "Test",
			Description: "A generic Test",
			Data: `// A generic Test
{
    // Name is used during reporting. Make it short and printable.
    Name: "Simple Test"

    Description: '''
        This description is not used but it is nice to
        provide same background information on this test.
    '''

    // Details of the Request follow:
    Request: {
        // The HTTP method. Defaults to GET.
        Method: "GET"    

        // The URL of the request. Can contain query parameter (but these
        // must be properly encoded).
	// Notethe use a variable substitution: {{HOST}} is replaced with the
        // current value of the HOST variable.
        URL:    "http://{{HOST}}/html?q=xyz"
	
	// Header contains the specific http headers to be sent in this request.
	// User-Agent and Accept headers are set automatically to what Chrome
	// sends if not set explicitly.
        Header: {
            Accept-Language: "en,fr"
        }

        // Add URL parameter
        Params: {
            w: "why so?"      // automatic URL-encoding
            u: [123, "abc"]   // can send multiple u
        }

        Timeout: "25s"   // Use longer timeout then the default 10s
        FollowRedirects: true  // Follow redirect chain until non 30x is sent.
    }

    // The list of checks to apply.
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "ResponseTime", Lower: "700ms"}
    ]

    // Variables provides default values for the variables. The default
    // values can be overwritten from Suites and from the command line.
    Variables: {
        HOST: "www.example.org"
    }
}`,
			Sub: []*Example{
				&Example{
					Name:        "Test.Cookies",
					Description: "Testing setting and deleting Cookies in Set-Cookie headers",
					Data: `// Testing setting and deleting Cookies in Set-Cookie headers
{
    Name: "Test SeCookie Headers"
    Request: {
        URL: "http://{{HOST}}/other"
        Timeout: 2s
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}

	// Cookie cip was set to any value with any properties:
        {Check: "SetCookie", Name: "cip"}

	// Make sure cip's path is /
        {Check: "SetCookie", Name: "cip", Path: {Equals: "/"}}

        // Value is 20 to 32 alphanumeric characters
        {Check: "SetCookie", Name: "cip", Value: {Regexp: "[[:alnum:]]{20,32}"}}

	// cip is persistent (not a session cookie) with a lifetime of at
	// least 10 minutes and Http-Only
        {Check: "SetCookie", Name: "cip", MinLifetime: "10m"
            Type: "persistent httpOnly"}

        // Make sure cookie tzu gets deleted properly
        {Check: "DeleteCookie", Name: "tzu"}
    ]
}`,
				}, &Example{
					Name:        "Test.CurrentTime",
					Description: "Working with current time or date",
					Data: `// Working with current time or date
{
    Name: "Current time and date"
    Description: '''
        Unfortunately it is not straight forward to include the current date
        or time in a Test. But this can be simulated with Variables:
        You can always inject e.g. the current date via the command line
        as the value of a variable.
        The solution here might be a bit more flexible: The SetTimestamp
        data extractor can "extract" the current date/time (with arbitrary
        offset) into a variable. The value of this variable can be used
        in subsequent tests as the current date/time or some date/time
        in the future or past with defined offset to now.
        The Format string is the reference time Go's time package.
    '''

    // A dummy request: We are interested in the current date/time only.
    Request: { URL: "http://{{HOST}}/html" }

    DataExtraction: {
        // Store the current date and time in NOW
        NOW: {Extractor: "SetTimestamp", Format: "2006-01-02 15:04:05" }

	// Store date of the day after tomorrow in FUTURE
        FUTURE: {Extractor: "SetTimestamp", DeltaDay: 2, Format: "2006-01-02" }
    }
}`,
				}, &Example{
					Name:        "Test.Extraction",
					Description: "Extracting data from a Response",
					Data: `// Extracting data from a Response
{
    Name: "Data Extraction"

    Description: '''
        Combining tests into larger suites is useful only if later requests
        can depend on the result of earlier requests. This is like this in ht:
          1. Extract some data from a response and store it in a variable
          2. Use that variable in subsequent requests/tests
        This examples shows the generic mechanism of step 1.
    '''

    Request: { URL: "http://{{HOST}}/html" }
    Checks: [
        {Check: "StatusCode", Expect: 200}

        // Data extraction does not influence the test state: If the given
        // value could not be extracted the test is still in state Pass.
        // If subsequent tests/request rely on a proper data axtraction: Add a
        // check like the following to make sure the test fails if no suitable
        // value is present
        {Check: "Body", Regexp: "It's ([0-9:]+) o'clock"}
    ]

    // Define how variable values should be extracted from the response   
    DataExtraction: {
        // Set the value of SOME_VARIABLE. Use a generic "BodyExtractor"
        // to extract a value from the response body via a regular expression.
        SOME_VARIABLE: {
            Extractor: "BodyExtractor"

            // The regular expression to extract. This one would match e.g.
            // It's 12:45 o'clock.
            Regexp: "It's ([0-9:]+) o'clock"

            // Do not use the full match but only the first submatch which
            // will be the numerical time (here "12:45")
            Submatch: 1  
        }

        // Extract the session from the Set-Cookie handler.
        SESSION_ID: { Extractor: "CookieExtractor", Name: "SessionID" }
    }

    // Note that Data extraction happens only for Pass'ed test.
}`,
					Sub: []*Example{
						&Example{
							Name:        "Test.Extraction.HTML",
							Description: "Extracting data from HTML documents, e.g. hidden form values",
							Data: `// Extracting data from HTML documents, e.g. hidden form values
{
    Name: "Data extraction from HTML"
    Request: { URL: "http://{{HOST}}/html" }
    /* HTML has the following content:
           <h1>Sample HTML</h1>
           <form id="mainform">
             <input type="hidden" name="formkey" value="secret" />
           </form>
    */
    Checks: [
        {Check: "StatusCode", Expect: 200}
    ]

    DataExtraction: {
        FORM_KEY: {
            Extractor: "HTMLExtractor"
            // CSS selector of tag to extract data from
            Selector: "#mainform input[name=\"formkey\"]"
            Attribute: "value" // Extract content of this attribute.
        }
        TITLE: {
            Extractor: "HTMLExtractor"
            Selector: "h1"
            // Do not extract data from attribute but the text content
            // from the h1 tag
            Attribute: "~text~"
        }
    }

}`,
						}, &Example{
							Name:        "Test.Extraction.JSON",
							Description: "Extracting data from a JSON document",
							Data: `// Extracting data from a JSON document
{
    Name: "Data Extraction from JSON"

    Request: { URL: "http://{{HOST}}/json" }
    /* The returned JSON looks like this:
         {
            "Date": "2017-09-20",
            "Numbers": [6, 25, 26, 27, 31, 38],
            "Finished": true,
         }
    */     
    Checks: [
        {Check: "StatusCode", Expect: 200}
        // The following checks make sure that the tests fails if the
        // extraction woudn't succeed.
        {Check: "JSON", Element: "Date", Prefix: "\"", Suffix: "\"" }
        {Check: "JSON", Element: "Finished", Regexp: "true|false" }
        {Check: "JSON", Element: "Numbers.3", Is: "Int" }
    ]

    DataExtraction: {
        DATE:     {Extractor: "JSONExtractor", Element: "Date" }  // 2017-09-20
        FINISHED: {Extractor: "JSONExtractor", Element: "Finished" }   // true
        THIRDNUM: {Extractor: "JSONExtractor", Element: "Numbers.3" }  // 27
    }
}`,
						}},
				}, &Example{
					Name:        "Test.FollowRedirect",
					Description: "Automatic follow of redirects and suitable tests ",
					Data: `// Automatic follow of redirects and suitable tests 
{
    Name: "Follow Redirections"
    Request: {
        URL: "http://{{HOST}}/redirect2"
        FollowRedirects: true
    }
    Checks: [
        // FollowRedirects follows redirects until 200 is received.
        {Check: "StatusCode", Expect: 200}

        // Check intermediate locations.
        {Check: "RedirectChain"
            Via: [ ".../redirect1", ".../html" ]
        }

	// Apply condition to final URL. Here full equality.
        {Check: "FinalURL", Equals: "http://{{HOST}}/html"}
    ]
}`,
				}, &Example{
					Name:        "Test.HTML",
					Description: "Testing HTML documents",
					Data: `// Testing HTML documents
{
    Name: "Test of HTML page"
    Request: {
        URL: "http://{{HOST}}/html"
        Timeout: 2s
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "ResponseTime", Lower: "700ms"}
        {Check: "ContentType", Is: "text/html"}
        {Check: "UTF8Encoded"}
        {Check: "ValidHTML"}

        // Uncomment if it's okay to send response to W3C Validator. 
        // {Check: "W3CValidHTML", AllowedErrors: 5}  

        // Make sure resources linked from the HTML document are accessable.
        {Check: "Links"
            Which: "a link img script"  // check only these tags
            Head: true                  // HEAD request is enough
            Concurrency: 8              // check 8 links in parallel
            IgnoredLinks: [
                // No need to check these links
                {Contains: "facebook.com"},
                {Equals: "http://www.twitter.com/foo/bar"}
            ]
            FailMixedContent: true
        }
    ]
}`,
				}, &Example{
					Name:        "Test.Image",
					Description: "Testing images",
					Data: `// Testing images
{
    Name: "Test of a PNG image"
    Request: {
        URL: "http://{{HOST}}/lena"
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "Image"}  // response is an image
        {Check: "Image", Format: "png"}  // it's a PNG image
        {Check: "Image", Width: 20, Height: 20}  // proper size

	// Check color fingerprint of image.
        {Check: "Image", Fingerprint: "-P000000Zn0000l0100a030a", Threshold: 0.0025}

	// Check block-mean-value (BMV) fingerprint of image
        {Check: "Image", Fingerprint: "be1cbd8d0b0b0f8c"}

        // Combined
        {Check: "Image", Fingerprint: "be1cbd8d0b0b0f8c", Width: 20, Height: 20, Format: "png"}

        // Check full binary identity:
        {Check: "Identity", SHA1: "f2534d702f0b18907162d7017357608ab2a40e2b"}
    ]
}`,
				}, &Example{
					Name:        "Test.JSON",
					Description: "Testing JSON documents",
					Data: `// Testing JSON documents
{
    Name: "Test of a JSON document"
    Request: {
        URL: "http://{{HOST}}/json"
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "UTF8Encoded"}
        {Check: "ContentType", Is: "application/json"}

        // Valid JSON, don't care about anything else.
        {Check: "JSON"}

        // Presence of field "Date", any value of any type is okay.
        {Check: "JSON", Element: "Date"}

        // Check value of Date fields. Pay attention to quotes of strings.
        {Check: "JSON", Element: "Date", Equals: "\"2017-09-20\""}
        {Check: "JSON", Element: "Date", Contains: "2017-09-20"}
        {Check: "JSON", Element: "Finished", Equals: "true"}

        // Access to deeply nested elements.
        {Check: "JSON", Element: "Numbers.0", Equals: "6"}
        {Check: "JSON", Element: "Numbers.1", GreaterThan: 6, LessThan: 45}
        // Change field seperator if your field names contain the default "."
        {Check: "JSON", Sep: "_@_",  Element: "a.b_@_wuz_@_1", Equals: "9"}

        // Check structure of JSON and type of data with Schema.
        {Check: "JSON", Schema: '''
            {
               "Date":     "",
               "Numbers":  [0,0,0,0,0,0],
               "Finished": false,
               "Raw":      "",
               "a.b":      { "wuz": [] }
            }'''
        }

        // Interpret and check strings which contain embedded JSON:
        {Check: "JSON", Element: "Raw", Embedded: {Element: "coord.1", Equals: "-1"}}
        {Check: "JSON", Element: "Raw", Embedded: {Element: "label", Equals: "\"X\""}}

        // There's a different check for JSON: JSONExpr
        {Check: "JSONExpr", Expression: "$len(.Numbers) > 4"}
        {Check: "JSONExpr", Expression: "$max(.Numbers) == 38"}
    ]
}`,
				}, &Example{
					Name:        "Test.Mixin",
					Description: "A Test including Mixins",
					Data: `// A Test including Mixins
{
    Name: "Test with Mixins"
    Mixin: [
        "Mixin"
        "Mixin.Checks"
    ]
    Description: "Most parts of this Test com from the two Mixins above." 
    Request: {
        URL:    "http://{{HOST}}/html"
        // Additional Request fields are loaded from Mixin
    }
    Checks: [
        {Check: "Body", Contains: "e"}
        // Additional Checks are loaded from Mixin.Checks
    ] 
}`,
				}, &Example{
					Name:        "Test.POST",
					Description: "Generating POST requests",
					Data: `// Generating POST requests
{
    Name: "Test a POST request"
    Request: {
        Method:   "POST"
        URL:      "http://{{HOST}}/post"
        ParamsAs: "body"   // send as application/x-www-form-urlencoded in body
        Params: {
            "action":      "update"  // simple
            "data[1][7]":  12        // fancy parameter name
            "what":        "automatic encoding+escaping!" // let ht do the hard stuff
            "several": [ "foo", "bar", 123 ]  // multiple values
        }
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
    ]
}`,
					Sub: []*Example{
						&Example{
							Name:        "Test.POST.BodyFromFile",
							Description: "Reading a POST body from a file",
							Data: `// Reading a POST body from a file
{
    Name: "Test body read from file"
    Request: {
        Method:  "POST"
        URL:     "http://{{HOST}}/post"
	Header:  { "Content-Type": "application/json" }

        // Body can use @file and @vfile just like Params:
        // The @vfile version will perform variable substitution in the
        // content of somefile. Note how somefile is read realtive to
        // directory of this test-file.
        Body: "@vfile:{{TEST_DIR}}/somefile"

        // Use the @file form if no variable substitution inside somefile
	// shal be performed.	
        // Body: "@file:{{TEST_DIR}}/somefile"
    }

    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "Body", Contains: "TheFoo"}
    ]

    Variables: { FOO: "TheFoo" }
}`,
						}, &Example{
							Name:        "Test.POST.FileUpload",
							Description: "Uploading files as multipart data",
							Data: `// Uploading files as multipart data
{
    Name: "Test file uploads"
    Request: {
        Method:   "POST"
        URL:      "http://{{HOST}}/post"
        ParamsAs: "multipart"   // send as multipart/form-data
        Params: {
            // action is a simple parameter
            "action":  "update"
 
            // upload exact content of Test.HTML from current folder as file1
            "file1":  "@file:{{TEST_DIR}}/somefile"

            // substitute variables in Test.HTML before uploading
            "file2":  "@vfile:{{TEST_DIR}}/somefile"
        }
    }

    Checks: [
        {Check: "StatusCode", Expect: 200}
    ]

    Variables: { FOO: "TheFoo" }
}`,
						}, &Example{
							Name:        "Test.POST.ManualBody",
							Description: "Manualy defining a POST body.",
							Data: `// Manualy defining a POST body.
{
    Name: "Test POST body"
    Request: {
        Method:  "POST"
        URL:     "http://{{HOST}}/post"
	Header:  { "Content-Type": "application/json" }

	// Manualy crafted request body. 
        Body: '''  {"status": "success"}  '''

        // Body can use @file and @vfile just like Params:
        // The @vfile version will perform variable substitution in the
        // content of somefile. Note how somefile is read realtive to
        // directory of this test-file.
        // Body: "@vfile:{{TEST_DIR}}/somefile"

        // Use the @file form if no variable substitution inside somefile
	// shal be performed.	
        // Body: "@file:{{TEST_DIR}}/somefile"
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "Body", Contains: "success"}
    ]
}`,
						}},
				}, &Example{
					Name:        "Test.Redirection",
					Description: "Testing redirect responses",
					Data: `// Testing redirect responses
{
    Name: "Redirections"
    Request: {
        URL: "http://{{HOST}}/redirect1"
    }
    Checks: [
        {Check: "StatusCode", Expect: 301}
        {Check: "Redirect", To: ".../html"}
        {Check: "Redirect", To: ".../html", StatusCode: 301}
    ]
}`,
				}, &Example{
					Name:        "Test.Retry",
					Description: "Retrying failed tests and polling services.",
					Data: `// Retrying failed tests and polling services.
{
    Name: "Retry a test several times"
    
    Request: { URL: "http://{{HOST}}/html" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]

    // Execution controls timing and retrying of a test
    Execution: {
        // Try this test up to 7 times. If all 7 tries fail report a failure.
        // Report pass after the first passing run.
        Tries: 7
        Wait: "800ms"   // Wait 0.8 seconds between retries.
    }
    // Retrying a test can also be used to poll a service-endpoint which takes
    // some time to provide information: Instead of sleeping 60 seconds before
    // querying the service poll it every 5 seconds for up to 15 tries.
}`,
				}, &Example{
					Name:        "Test.Speed",
					Description: "Testing the response speed of an application",
					Data: `// Testing the response speed of an application
{
    Name: "Test response time and latency"
    Request: {
        URL: "http://{{HOST}}/html"
        Timeout: 2s
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}

	// Response time of request from above
        {Check: "ResponseTime", Lower: "100ms", Higher: "35ns"}
        
	// Make 200 extra request to the same URL, 4 in parallel.
        {Check: "Latency", N: 200, Concurrent: 4, SkipChecks: true,
            // Check percentiles of response time
            Limits: "50% ≤ 100ms; 80% ≤ 150ms; 95% ≤ 200ms; 0.995 ≤ 0.75s"
        }

	// Dump data 
        {Check: "Latency", N: 20, Concurrent: 4, SkipChecks: true,
            DumpTo: "stdout",
            Limits: "50% ≤ 100ms; 80% ≤ 150ms; 95% ≤ 200ms; 0.995 ≤ 0.75s"
        }


    ]
}`,
				}, &Example{
					Name:        "Test.XML",
					Description: "Testing XML documents",
					Data: `// Testing XML documents
{
    Name: "Test of a XML document"
    Request: {
        URL: "http://{{HOST}}/xml"
    }
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "UTF8Encoded"}
        {Check: "ContentType", Is: "application/xml"}

        // Presence of element, no condition imposed on value.
        {Check: "XML", Path: "/library/book/character[2]/name" },

        // Check existance and value.
        {Check: "XML"
            Path: "/library/book/character[2]/name"
            Equals: "Snoopy"
         }

        // Check several Conditions on the value:
        {Check: "XML"
            Path: "//book[author/@id='CMS']/title"
            Prefix: "Being"
            Contains: "Dog"
         }
    ]
}`,
				}},
		}},
}