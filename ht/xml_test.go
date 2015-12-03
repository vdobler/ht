// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import "testing"

var xmlr = Response{
	BodyStr: `<?xml version="1.0" encoding="UTF-8"?>
<library>
  <!-- Great book. -->
  <book id="b0836217462" available="true">
    <isbn>0836217462</isbn>
    <title lang="en">Being a Dog Is a Full-Time Job</title>
    <quote>I'd dog paddle the deepest ocean.</quote>
    <author id="CMS">
      <?echo "go rocks"?>
      <name>Charles M Schulz</name>
      <born>1922-11-26</born>
      <dead>2000-02-12</dead>
    </author>
    <character id="PP">
      <name>Peppermint Patty</name>
      <born>1966-08-22</born>
      <qualification>bold, brash and tomboyish</qualification>
    </character>
    <character id="Snoopy">
      <name>Snoopy</name>
      <born>1950-10-04</born>
      <qualification>extroverted beagle</qualification>
    </character>
  </book>
</library>
`}

var xmlTests = []TC{
	{xmlr, &XML{Path: "/library/book/isbn", Condition: Condition{Equals: "0836217462"}}, nil},
	{xmlr, &XML{Path: "/library/book/character[2]/name", Condition: Condition{Equals: "Snoopy"}}, nil},
	{xmlr, &XML{Path: "//book[author/@id='CMS']/title", Condition: Condition{Contains: "Dog"}}, nil},
	{xmlr, &XML{Path: "/library/book/notthere", Condition: Condition{Contains: "a"}}, someError},
}

func TestXML(t *testing.T) {
	for i, tc := range xmlTests {
		runTest(t, i, tc)
	}
}
