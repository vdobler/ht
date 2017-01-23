Collection of TODOs and Ideas for HT
====================================

Open Issues
-----------

*  Load-/Throughput testing has no stop condition except the desired
   duration: Stuff like abort once too many error occur is missing.

*  Load-/Throughput testing should save data during test run, not afterwards.

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