// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import "testing"

var exampleHTML = `
<html>
  <head>
    <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
    <meta name="_csrf" content="18f0ca3f-a50a-437f-9bd1-15c0caa28413" />
    <title>Dummy HTML</title>
  </head>
  <body>
    <h1>Headline</h1>
    <div class="token"><span>DEAD-BEEF-0007</span></div>
  </body>
</html>`

func TestExtractor(t *testing.T) {
	test := &Test{
		Response: Response{
			BodyBytes: []byte(exampleHTML),
		},
	}

	ex := Extractor{
		HTMLElementSelector:  `head meta[name="_csrf"]`,
		HTMLElementAttribute: `content`,
	}

	val, err := ex.Extract(test)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	} else if val != "18f0ca3f-a50a-437f-9bd1-15c0caa28413" {
		t.Errorf("Got %q, want 18f0ca3f-a50a-437f-9bd1-15c0caa28413")
	}

	ex = Extractor{
		HTMLElementSelector:  `body div.token > span`,
		HTMLElementAttribute: `~text~`,
	}
	val, err = ex.Extract(test)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	} else if val != "DEAD-BEEF-0007" {
		t.Errorf("Got %q, want DEAD-BEEF-0007", val)
	}

}
