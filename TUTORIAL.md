Tutorial for Writing Test with ht
=================================

This tutorial describes basic stuff one needs to know to use cmd/ht and write
tests and to combine tests into suites.

Running ht
----------

`ht` is a command line application which does need external infastructure,
no Java/Ruby/Python runtime, no special libraries, no configuration files,
no registry entries, nothing. Download the version for your operating
system from (github)[https://github.com/vdobler/ht/releases].

Invoking it without any arguments describes it's usage and shows the
available subcommands. Help for the subcommands can be displayed in the
obvious way:

    $ ./ht
 
    $ ./ht help run

Make sure you use the proper version:
 
    $ ./ht version


Writing Tests
-------------

A test is stored as a JSON object in a file. The object needs three fields:

    {
        "Name":    "Some descriptive name, but no fancy characters please",
        "Request": { ... },
        "Checks":  [ ... ]
    }

Note the field names start with a Capital Letter and will be CamelCase.
Note that acronyms will be in all caps, e.g. "URL"

 * `Name` is a string and is needed to display and log the test properly.
 * `Request` is an object and contains information about the request to make
    for this test.
 * `Checks` is an array if checks to perform on the received response.


### HJSON actually

The files are not JSON but human json (Hjson)[http://hjson.org): Commas are
optional before linebreaks, quotation marks are optional, comments work and
multiline strings are avialable:

    {
        // This is a line comment
        Name:    Some descriptive name, but no fancy characters please
        Request: { ... }
        Checks:  [ ... ]
        Multiline: '''This is a
                      multiline
                      string.'''
    }


### The Request URL

The main (and mandatory) field of `Request` is the `URL` which must be a
complete URL including schema, host (optional port) and path. The URL may
contain a fragment, but this won't be sent to the server.

    {
        Name:    "Homepage"
        Request: {
            URL: "https://www.example.org/"
        },
        Checks:  [ ]
    }

TLS (SSL) secured request can be made, https:// is supported.
Save this as test1.ht and execute the request:

    $ ./ht run test1.ht

This should work, i.e. produce a request and print some output indicating
success (if you replaced www.example.org with an existing host) but it is
almost pointless as no checks are done.


Files on the local host may be accessed via the `file://` protocol schema:
 - GET method reads the given file and returns its content as
   the "response body".
 - DELTE will try to delete the file.
 - PUT writes the Request Body (see below) to the given file.
Note that several checks are unsuitable for such pseudo requests.


### Checks

Checks provide high- (some low-) level checks on the received response,
some might even trigger additional request and check these.
We'll start with low-level checks as these are easier to understand.

    {
        Name:    "Homepage"
        Request: {
            URL: "https://www.example.org/"
        },
        Checks:  [
            { Check: "StatusCode", Expect: 200 }
            { Check: "Body", Contains: "Hello World!" }
        ]
    }

Which check to execute is given in the `Check` field. Note again the
CamelCase nameing scheme.  A list of available checks is given below.
The other fields determine details of the check and are check dependent.
These other fields names have been chosen to allow "reading" the
check definition almost as clear text:

 * Check the status code and expect a value of 200.
 * Check the request body, it must contain "Hello World".

You may want to run it (after saving to test2.ht):

    $ ./ht run test2.ht

You should see the passed checks (if you replaced the canonical Hello World
with something actually present in the response).


Details of the `Request` object
-------------------------------

In the following we will take a detailed look at all fields of the `Request`
object and how to fine-control the generated request, how to send parameters,
how to add headers and cookies to the requests, etc. pp.


### POST, HEAD, PUT, DELETE...

The default for a request is `GET`.  If you want to create a different
type of request just specify the `Method`:

    {
        Name:    "Homepage"
        Request: {
            Method: "HEAD",   //  Note: ALLCAPS as in the actual request
            URL: "https://www.example.org/"
        },
        Checks:  [
            { Check: "StatusCode", Expect: 200 }
            { Check: "Body", Contains: "Hello World" }
        ]
    }

If `Method` is unset it defaults to `GET`.


### Sending query parameters

Sending parameters is quite simple as `ht` does all the heavy lifting of
encoding the parameters:

    {
        Name:    "Search",
        Request: {
            URL: "https://www.example.com/search",
            Params: {
                q: "Magento",
                foo: "a b+c%d",
            }
        },
        Checks:  [
            { Check: "StatusCode", Expect: 200 },
            { Check: "Body", Contains: "Magento" },
        ],
    }

When running this test you will see that the value of the parameter `foo` is
properly encoded and that the parameters are sent as query parameters in
the URL.


### Sending POST parameters

How the parameters are sent is controlled with the `ParamsAs` field of `Request`:

    {
        Name:    "Search",
        Request: {
            Method: "POST",
            URL: "https://www.example.com/search",
            ParamsAs: "body",  //  -->  application/x-www-form-urlencoded
            Params: {
                q: "Magento",
                foo: "a b+c%d",
            }
        },
        Checks:  [
            { Check: "StatusCode", Expect: 200 },
            { Check: "Body", Contains: "Magento" },
        ],
    }

This will make a POST-request and send the parameters urlencoded in the request
body. It will automatically set the appropriate Content-Type header to
"application/x-www-form-urlencoded".

If `ParamsAs` is unset it defaults to `URL` which indicates to send as query
parameters in the URL.


Builtin Documentation
---------------------

There are several more fileds in a Test or a Request. Ht has a built in
documentation which can be used t ofind out which fields are available
in a Test, what there type is and how the controll test execution.
Just run e.g.

    $ ht doc Test
    $ ht doc Request
    $ ht doc Execution

The list of checks and variable extractors can be printed with 

    $ ht help checks
    $ ht help extractors

and the details can be displayed with `ht doc BodyExtractor`.


Mixins
------


It is painful and error prone to add the common header fields of a "normal"
browser-like request. To facilitate this `ht` provides the possibility to
merge partial tests -- call "mixins" -- into the actual test.

### Including partial tests

Assume you have the following Hjson file which is a partial test (as it
has no URL):

    std-headers.mixin:
    {
        Name:    "Standard headers",
        Request: {
            Header: {
    	        User-Agent: "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2272.101 Safari/537.36"
    	        Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"
    	        Accept-Language: "fr;q=0.2"
    	        Accept-Encoding: "gzip, deflate"
            }
        }
    }

and a "real test":

    real-test.ht
    {
        Name:    "Homepage"
        Mixin:   [ "std-headers.mixin" ],  //  <<-- here
        Request: { URL: "https://www.example.com/" }
        Checks:  [ {Check:"StatusCode",Expect:200}, {Check:"Body",Contains: "Hello"} ]
    }

The settings made in std-headers.mixin are incorporated into the real-test.ht as
if a Header field would be present. The mixins are merged, the real-test.ht
himself may have Header definitions.

As usual a single mixin need not be written as an array.



Combining Tests to Suites
-------------------------

Running several tests in one batch is possible by

    $ ./ht run test1.ht test2.ht test3.ht

But these tests are executed completely unrelated. For more control tests
can be combined into "Suites". 

Suites are stored on disc as Hjson files like tests are. The following
shows everything a suite may contain.  As usual it starts with a Name
and as Description field:

    {
        Name:        "Sample Suite",
        Description: "Optional verbose details for suite",
        KeepCookies: true,   //  handle cookies like a browser
        Verbosity:   2,      //  fix verbosity of all tests to 2

        Setup: [
          {File: "test1.ht"}
        ]

        Main: [
            {File: "test2.ht"}
            {File: "../common/test3.ht"}
        ]

        Teardown: [
           {File: "test4.ht"}
        ]

        Variables: {
           HOST: "www.example.com"
           FOOBAR: "Something else here"
        }
    }   

The Setup, Main and Teardown are arrays of filenames of tests.
The actual tests in Tests are executed only if all tests in Setup pass.
Teardown tests are executed always but their status is ignored.

All tests are executed strictly in serial order, one after the other.
If `KeepCookies` is true than any cookie set by the server will be
stored and (depending on the request details) sent back in subsequent
requests.

All this -- with the exception of the `Variables` field -- should be pretty
straightforward and obvious how it works.


Executing suites
----------------

Suites are nice but they offer structure which you might want to control.
The general way to execute a suite is to run the suite through `exec`:

    $ ./ht exec _somefancy.suite_

You may run several suites in one batch, `ht exec` will execute all suites
given in the command line.

To see which tests are "in" a suite use the `list` subcommand:

    $ ./ht list the.suite

If you want to skip certain tests or run just some test you can use the flags
`-skip` and `-only`. E.g.:

    $ ./ht exec -only 3-9 -skip 6 somefancy.suite

Would run only the actual tests 3, 4, 5, 7, 8 and 9 (counting from 1).


Using Variables in tests
------------------------

The most fancy part in the suite above is the `Variables` field which contains
key/value-pairs: Variable names and the corresponding value.  


### Using variables

Variable replacement can be used in a lot of places, from the request URL, over
parameter values to fields in checks. Variable replacements are written like

    "This is fixed {{VARNAME}} rest is fixed too."

If `VARNAME="foo 123"` the resulting string will be:

    "This is fixed foo 123 rest is fixed too."

This works basically just like using ${VARNAME} in bash.
(Variables may have lowercase letters too.)
Please note that while we call it here "variables" it is just a brain dead
text substitution: If you set a "variable" `FOO` to the value `bar` than any
occurence of "{{FOO}}" will be replaced by "bar". If there is no "variable"
`FOO` defined than "{{FOO}}" will stay "{{FOO}}".

Take a look at the example suite above, 
`HOST` is a good example why variables exist:  You may want to write _one_ test
and have this test executed accessing different environments: From
localhost to development, to integration, to acceptance and even on
production.  Making the tests parametrized on the HOST name makes this
possible,


### Setting variables from the command line

Will I have to write suites for every environment just to provide variable
values? Of course not. `ht` has two command line flags which allow to set
variable values during invocation of `ht`:

 * `-D `_varname_`=`_value_ : Will set the variable _varname_ to _value_.
   E.g. `-D HOST=localhost:9001`

 * `-Dfile` _file.json_ : This will read variable names and values from the
   given JSON5 file _file.json_.

The `-Dfile` flags are handled first, you can overwrite the values with `-D`.

    $ ./ht -Dfile uat.json -D HOST=127.0.0.1:8080 
 

### Extracting values from the response

Variables get their full power from being settable from received responses.
This is done through different "Extractors" which populate variables from
data extracted from the response. The following are available:

    BodyExtractor, CookieExtractor, HTMLExtractor, JSONExtrator,
    JSExtrcator, SetVariable

( Run `ht help extractors` to print this list.)

An example might be helpfull:

    {
        Name: "Unquote the received Body",
        Request: {
            URL:    "http://example.org/some/path",
        },
        Checks: [
            {Check: "StatusCode", Expect: 200},
            {Check: "Body", Prefix: "\"", Suffix: "\""},
        ],
        VarEx: {
            TOKEN: {Extractor: "BodyExtractor", Regexp: "\"(.*)\"", Submatch: 1},
        },
    }

The response to the GET request is checked. The second checks passes if the
body consists of a double quoted string. If both checks pass variable
extraction begins: A BodyExtractor is invoked which extracts what's inside
the quotes; this value is assigned to the variable TOKEN.


### Preset variables

Some variables are preset on a per Test basis if loaded from a .ht file:

 * `TEST_NAME` : The basename of the test file, e.g.
        homepage.ht
 * `TEST_DIR`  : The (relative) directory path the test was loaded from, e.g.
        ./test
 * `SUITE_NAME` : The basename of the suite file, e.g.
        basic.suite
 * `SUITE_DIR`  : The (relative) directory path the suite was loaded from, e.g.
        ./basic


### Special variables

There are two special variables `RANDOM` and `COUNTER` which provide 
6-digit random numbers and an ever increasing counter.


### Replacing variables in data loaded from files

You may upload files in mutlipart request or send the content of a file as the
request body with the special syntax "@file:/path/to/file" (see above).
The file content is sent "as is" without applying variable replacements.

You may perform variable replacements on the loaded file content with the
special syntax "@vfile:/path/to/file".

Combining this with the "normal" variable substitution and the predefined
variables described in the last chapter allows you to use

    "@vfile:{{TEST_DIR}}/file-template"

with a file named `file-template` in the folder where the test lives
which may itself contain variables, e.g. the following file could be
uploaded or sent as the body:

    Start-Time: {{STARTED}} 
    User:       {{USER_ID}}


### Calling Test from Suites

A suite may "call" a test with different set of variables:

    # some.suite
    {
        Main: [
            {File: "atest.ht", Variables: { V1: "foo" }}
            {File: "atest.ht", Variables: { V1: "bar" }}
        ]
    }

    # atest.ht
    {
        Request: { URL: "http://example.com/{{$V1}}" }
        Variables: { V1: "wuz" }
    }

The suite would execute atest.ht three time
 1. with `V1` beeing "foo", 
 2. with `V1` beeing "bar", and
 3. with `V1` beeing _unset_ and thus defaulting to "wuz"

This is a general pattern: If a variable is _unset_ on the higher scope
it will default to wahtever is set in the "Variables" section of a suite
or a test.
Invoking some.suite like `ht exec -D V1=xyz` would result in atest.ht
beeing called three times with V1=xyz.



Splitting Suites
----------------

A Suite wraps several request/check combinations into a single unit, typically
to share cookies (`KeepCookie`) or to use variables extracted from previous
responses. Such a suite can be split into parts and these parts can share
cookies and variables. Like this you can check the first bunch of request,
do something else (run some DB tools, etc.) and "continue" the suite.

 * The final set of variables and/or cookies after executing a suite can be
   written to disk with the `-vardump` and `-cookiedump` command line options.

 * Using the dumped variables in a subseqeunt execution of a different suite
   is possible with the `-Dfile` option (see above).

 * Using dumped cookies is possible with the `-cookies` command line option.

Note that running several suites in parallel works, but the values for cookies
and variables saved might not be what yoi might expect naively.





