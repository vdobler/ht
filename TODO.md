Collection of TODOs and Ideas for HT
====================================

Open Issues
-----------

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