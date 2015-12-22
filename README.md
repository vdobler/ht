HTTP Testing Made Easy
======================

End-to-end testing of HTTP requests/responses is easy with Go.

Writing and maintaining high level test is even easier with ht.


Goals
-----

ht tries to achieve the following:
* Make generating all common HTTP requests easy.
* Provide high- and low-level checks on the received response. 
* Make measuring response times easy.
* Make it easy to generate a certain load.


Non-Goals
---------

ht is not the jack of all trades in testing web applications:
* Simulating browsers (evaluating JavaScript or even rendering
  is not done.


Installation
------------

Installing ht should be simple if Go 1.5 (pre beta is okay) and
git are available and working:
* Run `GO15VENDOREXPERIMENT=1 go get github.com/vdobler/ht/cmd/ht`
  which should download, compile and install everything.
* Run `$GOPATH/bin/ht help` to get you started.
* For a quick check of a HTML page do a 

    $GOPATH/bin/ht quick <URL-of-HTML-page>

  and check the generated Report.html file.


Documentation
-------------

For a start have a look at the 

Tutorial https://github.com/vdobler/ht/blob/master/cmd/ht/Tutorial.md

and see the the godoc for reference:

* Tests, Checks and their options:
  https://godoc.org/github.com/vdobler/ht/ht
* Details to Conditions used in Checks:
  https://godoc.org/github.com/vdobler/ht/condition
* Image fingerprinting:
  https://godoc.org/github.com/vdobler/ht/fingerprint
* The ht command itself:
  https://godoc.org/github.com/vdobler/ht/cmd/ht
  Run `ht help` for details
* An example test suite can be found in
  https://github.com/vdobler/ht/blob/master/testdata/sample.suite

