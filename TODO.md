Collection of TODOs and Ideas for HT
====================================

Open Issues
-----------

*  The Latency checks should be made runnable everywhere, e.g. by first
   measuring the "speed" of the system and scaling the tolerable limits
   before jumping into the test. A table would be good enough to start:
     normal limit, limits under race, normal on Travis, Travis under race.


*  Instead of Hjson with variable replacement it could be much nicer to use
   a real configuration language for Checks, Tests and Suite.
   Maybe Skylark github.com/google/skylark might be a nice option as it
   has Python syntax which allows a natural representation of Go structs
   and advanced string procesing routines.

*  There is no builtin documentation for the file://, bash:// and sql://
   pseudo-request at all. The builtin documentation for the "raw" tests
   and suites is lacking. E.g. ht doc RawTest yields something like:
       type RawTest struct {
           *File
           Mixins    []*Mixin          // Mixins of this test.
           Variables map[string]string // Variables are the defaults of the variables.
       }
       RawTest is a raw for of a test as read from disk with its mixins and its
       variables.
   which is not helpful when writing a Test in Hjson disk format. Almost the
   same with 'ht doc Test' which outputs all fields of a Tests, inculding the
   "output" fields which are useless to set. Here the GUI documentation is
   more helpful. But also this is missing documentation for the Mixins. This
   problem is even worse for Suites.
   The documentation is mostly there, but spread over Readme.md files, type
   docs, package docs and sometimes even unit tests.
   Basically 'ht doc' need not output what 'go doc' would print: 'ht doc' is
   used while using cmd/ht and not the libraray ht. So ' go doc Test' should
   display how to write a Hjson based test. I'm comfortable with 'ht doc Test'
   outputing just the direct/toplevel fields of Test and the user has to run
   'ht doc Request' to dig into the fields of Test. For CustomJS check and
   JSExtractor it might be is important to know e.g. the details of how a
   Response is structured. Maybe the documentation of these two has to be
   augmented and should point to the official godoc of ht.Test.
   For writing Hjson tests it is not really needed to know that Body is a
   Condition: It is nice as a reference but you are interesetd in the details
   for Min and Regexp so 'ht doc Body' should display the fields of a
   Condition directly. This is already done in the GUI to some extent.

*  The file://-Pseudorequest work on localhost only which is a bit lame given
   that the Logfile check can access remote files via ssh. The AuthMethods
   could be passed in the HTTP header in a quite natural way.

*  Generating and storing the type doc twice is overkill. The GUI data could be
   used to generate the `go doc` output.

*  Error handling in the GUI is not good. 

*  The tests for phantomjs screenshots fail on Windows because the font (or its
   properties) used differs. Maybe forcing a certain font and fixing height and
   weight via CSS could overcome this.

*  Several types of Checks would be very sensible:
     o Content Efficiency 
         - Is gzip used
         - HTML/CSS minified (at least stripped of comments/blanklines/whitespace)
         - Are logos delivered as svg? Or at least as PNG-8
         - Are images in WepP or JPG XR format
       https://developers.google.com/web/fundamentals/performance/optimizing-content-efficiency/
     o Security:
         - HTTP-header
         - JSON delivered with proper content type and only objects (no arrays,
           no primitives).
       https://httpsecurityreport.com/best_practice.html
       https://www.keycdn.com/blog/http-security-headers/

*  Load-/Throughput testing has no stop condition except the desired
   duration: Stuff like abort once too many error occur is missing.
   --> Done for errors. Failures still missing

*  If FollowRedirects==false and a redirect response is received, then
   the body is not readable (as it got closed by the Client before stopping
   the redirections).  Fixing this would require the use of a raw
   Transport.  This issue highlight a general problem in ht: Handling of
   clients and reuse/sharing them e.g. in Latency checks.  This is done
   headless and without any concept or proper design.


Resolved TODOs
--------------

*  CSRF token extraction from previous request and injection
   into the actual test-request.
   --> Hack with VarEx

*  Properly documentation for all Checks: Consistent and
   grep-able in godoc.
   --> list-checks.bash

*  Redesign and enhance Variable Extraction mechanism. Now it works only
   for an unprocess HTML attribute (and text nodes via a hack). Maybe allow
   HTTP headers, JSON values, XML elemnts. Possibly even allow to extract
   stuff inside them, e.g. a capturing group in a regexp.
   --> nicer abstract Variable Extraction

*  HEAD request with gziped Content-Encoding fail with EOF which is
   wrong.
   --> Simply ignore the body on HEAD requests totaly

*  Link checking does not properly keep the cookie jar: t.Jar is overwritten
   during suite preparation.  This might be a general bug.

*  Fingerprinting transparent images: Effect of problem is reduced due to
   mor bits representing the histogram.

*  Parameters may be files with the "@file:/path/to/file" syntax.
   Unfortunately relative path names are relative to the current working
   directory of the ht command and not relative to the test file.
   This is dead ugly and probably wrong.  Probably the best solution would
   be to use the cwd of ht if the test was not loaded from JSON and the
   directory of the test JSON otherwise.
   --> The new variable handling and the TEST_DIR mechnism solve this.

*  Load-/Throughput testing should save data during test run, not afterwards.
   --> Now written to live.csv.

*  Analysation of the load test data is currently done before saving.
   It would be much nicer if a new subcommand like `ht analyse <live.csv>`
   which would sort/enrich live.csv to what throughput.csv is now and
   calculate statistics (and do a nonzero exit depending on the outcome).
   --> `ht stat <live.csv>`

*  Suites and Tests have an ugly bidirectional coupling:
     - A Suite is a collection of Tests. This okay, needed and sensible
     - A Test carries the Reporting field that is filled by the
       Suite to generate reports.
   A more sensible structure would be:

       type Suite struct {
           Elements []struct{
               Test *Test
               Meta struct{ ... }
           }
       }
   
   So that a Suite contains a list of Elements which are basically just Tests
   but we can record lots of meta data like sequence numbers, was it a Setup
   or a Teardown test, which Scenario died it come from, sould it be
   counted or reported during determination of overall status, etc. pp.
   This meta-data could contain stuff only needed for report generation like
   HTML id attributes.
   On the other hand it would be nice if such meta data could be attached
   to a Test, so that the test drags the meta data with it while being moved
   around. Something like

       type Test struct {
           Meta map[string]interface{}
       }
   
   Now Suites could stuff in test.Meta["suite.seqno"] = 1467 and a throughput
   test could attach Meta["tpt.repetition"] = 12, a report could glue
   Meta["uuid"] = "1234-5678-9876" to a test, etc.

   --> Test gained a metadata map. Seems to work

*  Prepare on Check is not that useful any more:
   The Prepare() method of type Check serves two purposes
     1. Fail early if a check is bogus, e.g. if a regular expression is
        malformed or if a combination of flags is non-sensical or if
        necessary values are missing.
     2. Do expensive setup work before test execution, e.g. compile
        regular expressions.
   Especially 2. is not that useful any more: Actual Tests and actual
   checks are often created dynamicaly from RawTests and RawChecks as
   read from disk during Suite execution or load testing: The Tests
   are not reused but recreated (with a different set of variables).
   For normal Test execution the overhead of compiling a regular expression
   is probably very small so 2 does no longer justifiy as a requirement
   for Check to have a Prepare method.
   Bailing out early in case of problems typically cannot be done until
   the set of Variables are known as the Test itself is constructed on
   the fly from a RawTest. And we probably would like the Test request
   to be executed, even if a Check was Bogus, so this bailing out early
   is not really "out" in a hard sense anyway.
   The current Prepare() method does not take the Test as an argument
   which makes it impossible to detect miss-used checks, e.g. a Check
   which is unsuitable on a HEAD request.

   Probably the most sensible thing to do would be:
     - Strip the Prepare Method from type Check
     - Introduce a new type ValidatableCheck (or Preparer?) with 
       a method  Validate(*Test) error which does the validation
       work.
   While experimenting with a Prepare-less Check type it turned out that it's
   not that simple: At least one check (Logfile) relies on the fact that its
   Prepare method was called before the actual HTTP request is made. The
   Logfile check compares the logfile before to the logfile after the request.
   So the method should not be named "Validate" but be kept to "Prepare".
   So the refactoring path could look like
     1. Make Prepare a method of Preparable
     2. Call Prepare only on Preparables
     3. Delete dummy Prepare Methods
     4. Add Test as argument to Prepare
     5. Refactor MalformedCheck Error handling. 

  --> Implemented the 5 step plan.

*  Image fingerprinting treats transparent background as black which
   make using these fingerprint techniques almost useless in images with
   lots of transparent parts (e.g. the Google logo) as black will dominate
   the color histogram. For BMV hashes the same problem exists.
   The solution for color histograms would be simple: Just ignore transparent
   pixels, but that makes comparison complicated: What is the fingerprint
   was taken with 75% transparent pixel but the image to check is fully
   opaque and uses white for these 75% pixels? For BMV hashes ignoring pixels
   even more strange.
   Replacing transparent with a fixed or user-setable checkboard could work.
   But a fixed one is inflexible and user-defined ones are awfull.

   --> Ignoring pixels with alpha < 64. So normal logos which are totaly
       opaque logo on totaly transparent background work as expected, at
       least for color histogram fingerprinting.

*  The type and field documentation for the GUI needs to be automated.
   --> Automated via gengui.go

*  Export in the GUI does not produce valid Hjson tests:
     - At least time.Duration fields are off by a factor of 10^9 (ns vs s).
       Could be fixed by traversing the Hjson soup and chaning any suspicious
       int64 like 500000000 to a string like "500ms".
     - Variables have been substituted during loading of the test. To roundtrip
       from/to disk it would be nice if the exported test could contain the
       unexpanded Variables. Same solution: traverse the hjson soup and replace
       current values with {{name}}.
   Both "solutions" are just heuristics.
   --> But seems to work "good enough".

*  Examples are missing.
   Maybe this could also fix (or at least soften) the documentation problem:
   Let 'ht example Test' produce an generic example test.ht in Hjson format
   containing mixins, a request, execution, checks, variabels and extractors.
   The user just deletes what seh doesn't need. Also output that specialised
   sub-examples of Test are available like Test.JSON, Test.HTML, Test.MySQL,
   Test.File. Running 'ht example Test.MySQL' would output an example of how
   to do a sql:// pseudo-request. This can be enhannced to Test.MySQL.Query,
   Test.MySQL.Insert and Test.MySQL.Update to provide even more detailed
   examples. An example for Test.HTML could contain lots of checks suitable
   for HTML pages (maybe all of them) and data extractions targeted at
   HTML pages (BodyExtractor, HTMLExtractor, HeaderExtractor).
   Examples could be validated before baking into the ht example subcommand.
   Examples might be much easier to understand than documentation which
   requires to build a mental model of what will happen just from the factual
   description (which is hard).
       $ ht example
       Available top-level examples
         * Test      how to write a test
         * Suite     how to write a suite of tests
         * Scenario  define a scenario for a load test
         * Mixin     example of a test-mixin

       $ ht example Test
       // Test Example
       {
           // Name used during reporting
           Name: "Get Admin Bearer Token"
           // ...
       }
       Available Subtopics to Test:
         * Test.HTML   test a HTML document
         * Test.JSON   test a JSON response
         * Test.SQL    execute SQL and check result
         * Test.Speed  how to check speed of an endpoint
   This would provide valuable howtos and could replace the showcase.
   --> implemented