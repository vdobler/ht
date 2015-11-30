Collection of TODOs and Ideas for HT
====================================

Open Issues
-----------

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
