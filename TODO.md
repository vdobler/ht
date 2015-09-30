Collection of TODOs and Ideas for HT
====================================

*  Properly documentation for all Checks: Consistent and
   grep-able in godoc.

*  Redesign and enhance Variable Extraction mechanism. Now it works only
   for an unprocess HTML attribute (and text nodes via a hack). Maybe allow
   HTTP headers, JSON values, XML elemnts. Possibly even allow to extract
   stuff inside them, e.g. a capturing group in a regexp.

*  HEAD request with gziped Content-Encoding fail with EOF which is
   wrong.

*  Link checking does not properly keep the cookie jar: t.Jar is overwritten
   during suite preparation.  This might be a general bug.
   

Resolved TODOs
--------------

*  CSRF token extraction from previous request and injection
   into the actual test-request.
   --> Hack with VarEx