// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/scope"
	"github.com/vdobler/ht/suite"
)

// To test against a MySQL database:
//
var runmysql = flag.Bool("run-mysql", false,
	"Run the MySQL tests. Needs docker run -rm -d -e MYSQL_USER=test -e MYSQL_PASSWORD=test -e MYSQL_DATABASE=test -e MYSQL_ALLOW_EMPTY_PASSWORD=true -p 7799:3306 mysql:5.6")

var (
	exampleHTML = []byte(`<!DOCTYPE html>
<html>
<head>
    <title>Sample HTML</title>
</head>
<body>
  <h1>Sample HTML</h1>
  <p>Good Morning. It's 12:45 o'clock. have a good day!</p>
  <ul>
    <li><a href="/other">Other</a></li>
    <li><a href="/json">JSON</a></li>
  </ul>
  <form id="mainform">
    <input type="hidden" name="formkey" value="secret" />
  </form>
</body>
</html>`)

	exampleJSON = []byte(`{
  "Date": "2017-09-20",
  "Numbers": [6, 25, 26, 27, 31, 38],
  "Finished": true,
  "Raw": "{\"coord\":[3,-1,2], \"label\": \"X\"}",
  "a.b": { "wuz": [-3, 9] }
}
`)

	exampleXML = []byte(`<?xml version="1.0" encoding="UTF-8"?>
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
  <book id="299,792,459" available="true">
    <title lang="en">Faster than light</title>
    <character>
      <name>Flash Gordon</name>
    </character>
  </book>
  <book unpublished="true">
    <title lang="en">The year 3826 in pictures</title>
  </book>
</library>`)

	exampleImage = []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00,
		0x00, 0x00, 0x14, 0x00, 0x00, 0x00, 0x14, 0x08, 0x02, 0x00, 0x00, 0x00, 0x02, 0xEB, 0x8A, 0x5A, 0x00,
		0x00, 0x00, 0x04, 0x67, 0x41, 0x4D, 0x41, 0x00, 0x00, 0xB1, 0x8F, 0x0B, 0xFC, 0x61, 0x05, 0x00, 0x00,
		0x00, 0x01, 0x73, 0x52, 0x47, 0x42, 0x00, 0xAE, 0xCE, 0x1C, 0xE9, 0x00, 0x00, 0x00, 0x20, 0x63, 0x48,
		0x52, 0x4D, 0x00, 0x00, 0x7A, 0x26, 0x00, 0x00, 0x80, 0x84, 0x00, 0x00, 0xFA, 0x00, 0x00, 0x00, 0x80,
		0xE8, 0x00, 0x00, 0x75, 0x30, 0x00, 0x00, 0xEA, 0x60, 0x00, 0x00, 0x3A, 0x98, 0x00, 0x00, 0x17, 0x70,
		0x9C, 0xBA, 0x51, 0x3C, 0x00, 0x00, 0x00, 0x06, 0x62, 0x4B, 0x47, 0x44, 0x00, 0xFF, 0x00, 0xFF, 0x00,
		0xFF, 0xA0, 0xBD, 0xA7, 0x93, 0x00, 0x00, 0x00, 0x09, 0x70, 0x48, 0x59, 0x73, 0x00, 0x00, 0x0B, 0x12,
		0x00, 0x00, 0x0B, 0x12, 0x01, 0xD2, 0xDD, 0x7E, 0xFC, 0x00, 0x00, 0x04, 0x3A, 0x49, 0x44, 0x41, 0x54,
		0x38, 0xCB, 0x05, 0xC1, 0x4B, 0x6C, 0x54, 0x55, 0x18, 0x00, 0xE0, 0x73, 0xFE, 0x73, 0xDF, 0x33, 0x77,
		0x66, 0x3A, 0x33, 0x9D, 0x4E, 0xA1, 0x60, 0xCB, 0x88, 0xD0, 0x58, 0xB0, 0xA0, 0x18, 0x85, 0x88, 0x02,
		0x31, 0x31, 0x3E, 0x02, 0x21, 0x21, 0x29, 0x1A, 0xDB, 0x12, 0x58, 0x99, 0xB8, 0x80, 0xC4, 0x85, 0x3B,
		0x65, 0xAF, 0x6B, 0x17, 0xAE, 0x8C, 0x0B, 0x12, 0x4D, 0x88, 0x89, 0xE0, 0x42, 0x88, 0x40, 0x45, 0xD1,
		0x4A, 0x81, 0x8A, 0x94, 0xBE, 0xA6, 0x2F, 0xDA, 0x4E, 0xA7, 0xF3, 0xB8, 0xBD, 0x8F, 0x99, 0x7B, 0xCF,
		0x3D, 0x0F, 0xBF, 0x0F, 0x16, 0xBE, 0xFC, 0xB4, 0xF2, 0xF5, 0xC5, 0xDF, 0xCE, 0x7E, 0xF4, 0xE3, 0x89,
		0x53, 0x63, 0x43, 0x43, 0x0F, 0xCE, 0x8F, 0x3C, 0xBC, 0x30, 0x3A, 0x79, 0x7E, 0x74, 0x62, 0xE4, 0xE3,
		0x07, 0xE7, 0x86, 0x27, 0x2E, 0x8C, 0xFE, 0x35, 0x32, 0x72, 0x63, 0x68, 0x78, 0xFE, 0xF2, 0xC5, 0xC5,
		0xCB, 0x9F, 0x2C, 0x7F, 0x75, 0x69, 0xE1, 0xBB, 0x2F, 0xAE, 0x9F, 0x1E, 0x1A, 0xBF, 0x70, 0xEE, 0x9B,
		0xB7, 0x4E, 0x83, 0x6E, 0x60, 0x10, 0x4C, 0xC6, 0x14, 0x0B, 0x8E, 0xB9, 0x90, 0x4C, 0x60, 0x26, 0xA5,
		0x00, 0x61, 0x5A, 0xB1, 0x95, 0xE0, 0x6D, 0xAE, 0x72, 0x0E, 0x4C, 0x46, 0xBE, 0x00, 0x55, 0x0B, 0x2B,
		0xBE, 0x81, 0xC5, 0xCE, 0x43, 0x3D, 0x77, 0x1F, 0x6D, 0x8E, 0x3D, 0xA9, 0x83, 0xBB, 0xEE, 0x52, 0x37,
		0x94, 0x48, 0xD2, 0x18, 0x09, 0x8E, 0x30, 0xE3, 0x6D, 0x0E, 0x4D, 0xCD, 0x16, 0xA5, 0x92, 0x7D, 0xE2,
		0x90, 0x79, 0xEC, 0x55, 0x4F, 0x6A, 0x2C, 0x68, 0xE3, 0x88, 0xB1, 0x2D, 0x86, 0x19, 0x93, 0xE5, 0xC6,
		0x9A, 0x1B, 0xFF, 0x3E, 0xD5, 0x54, 0x00, 0xC1, 0xE3, 0x87, 0x35, 0x2E, 0x41, 0x48, 0xCC, 0x11, 0x70,
		0x89, 0xDA, 0x54, 0x78, 0x2D, 0x4A, 0x04, 0xC5, 0x9B, 0x9B, 0xC0, 0x71, 0x66, 0xF0, 0xC5, 0xE2, 0xF0,
		0xC9, 0x06, 0x52, 0x54, 0x10, 0x8F, 0x26, 0xAB, 0x32, 0x88, 0xD7, 0x17, 0xDD, 0x2B, 0xDF, 0x4E, 0x98,
		0x09, 0xFD, 0xCD, 0xA3, 0x7B, 0xC0, 0x0F, 0x39, 0x46, 0x20, 0x24, 0x46, 0x9C, 0x81, 0x46, 0x22, 0x84,
		0x35, 0x85, 0xA8, 0x04, 0xD4, 0x5C, 0x86, 0x68, 0x3A, 0x6D, 0x78, 0x76, 0x57, 0x76, 0xF7, 0xF0, 0xFB,
		0x92, 0x46, 0x0B, 0xCD, 0xC8, 0x1F, 0x3C, 0x74, 0xCF, 0xD1, 0x1A, 0x6E, 0x78, 0x70, 0xFF, 0x8E, 0x23,
		0x87, 0x7B, 0x20, 0xA9, 0x03, 0x51, 0x90, 0xA2, 0x62, 0xE0, 0x4C, 0x4B, 0x28, 0x84, 0x60, 0xC9, 0xA4,
		0x6A, 0xE9, 0x66, 0xCA, 0xC2, 0x5B, 0x0E, 0xF8, 0x4E, 0x6B, 0x75, 0x3D, 0xD9, 0x91, 0x42, 0x83, 0x03,
		0xEF, 0x7E, 0x76, 0xA6, 0xF7, 0xED, 0xD7, 0xEA, 0x1C, 0x3A, 0xD3, 0x89, 0xE3, 0x47, 0xFB, 0x38, 0x8D,
		0x01, 0x49, 0x4C, 0x54, 0x00, 0x02, 0x2C, 0x16, 0x24, 0x49, 0x0A, 0xDB, 0x0C, 0xAC, 0xAA, 0x0A, 0x92,
		0xAD, 0x65, 0xA7, 0xB6, 0xE0, 0x2D, 0xDD, 0x5F, 0xAA, 0xFF, 0xB7, 0x02, 0x7E, 0x5D, 0x1F, 0xE8, 0xD7,
		0x53, 0xF6, 0xCC, 0xD5, 0x9B, 0xF3, 0x0F, 0xCA, 0xC7, 0x8F, 0xF5, 0x5B, 0x36, 0x89, 0x62, 0x01, 0xBA,
		0x02, 0x80, 0xA5, 0x44, 0x12, 0x49, 0x49, 0x5B, 0x4C, 0xE9, 0x4C, 0x01, 0x28, 0xAE, 0x23, 0x7F, 0x1D,
		0x5B, 0xBB, 0x72, 0x75, 0xE1, 0xC6, 0x9F, 0x55, 0x9C, 0x56, 0xB1, 0xA5, 0x97, 0xEF, 0xCD, 0xAE, 0xDD,
		0xBC, 0x7B, 0xED, 0x87, 0x3F, 0x2C, 0x20, 0x03, 0x07, 0x8B, 0x21, 0x97, 0x02, 0x11, 0xA0, 0x12, 0x51,
		0x0E, 0x18, 0x03, 0xD1, 0xB5, 0xB0, 0x16, 0xC5, 0x81, 0xF0, 0x5B, 0xFC, 0xFA, 0x78, 0x65, 0xEA, 0xD9,
		0x96, 0x0E, 0xEE, 0xC9, 0x0F, 0x5F, 0xCE, 0xA5, 0xCD, 0x0C, 0xA8, 0xCD, 0xC9, 0xF2, 0xED, 0xB1, 0xE5,
		0xD9, 0xE9, 0xEA, 0xC0, 0x40, 0x37, 0xB1, 0x08, 0x97, 0x92, 0x10, 0x0C, 0x9C, 0x09, 0x81, 0x00, 0x11,
		0x05, 0x63, 0x92, 0x32, 0x60, 0x65, 0xCE, 0xBD, 0xB3, 0x18, 0x2D, 0xD5, 0xDC, 0x0E, 0x1B, 0x9F, 0x19,
		0x39, 0x10, 0x4C, 0xCF, 0xF0, 0xE9, 0x67, 0xA6, 0x22, 0x78, 0x1C, 0x35, 0x9B, 0xED, 0x54, 0xC6, 0xDE,
		0xDB, 0x9F, 0xE7, 0x9C, 0xB5, 0x29, 0x77, 0x7D, 0x0E, 0x11, 0xE5, 0x42, 0x4A, 0x83, 0x70, 0x45, 0x91,
		0xD5, 0x44, 0xFE, 0xD6, 0x92, 0xE6, 0x6B, 0x69, 0xD6, 0xF6, 0x8F, 0xBF, 0x53, 0x8A, 0x57, 0x6B, 0x0B,
		0x0F, 0x57, 0x79, 0x33, 0x98, 0xBA, 0xFD, 0xF4, 0xD1, 0xDF, 0xF3, 0x66, 0x52, 0xCF, 0xE7, 0xAC, 0x7C,
		0xD6, 0xE0, 0x41, 0xC8, 0x43, 0x26, 0x0D, 0x80, 0x75, 0x87, 0x86, 0xB1, 0xD0, 0x55, 0x12, 0xA7, 0x77,
		0xDE, 0x99, 0xB7, 0x6A, 0x6E, 0x68, 0x71, 0xA7, 0x7F, 0x5F, 0x17, 0x6C, 0xB9, 0x79, 0xAF, 0xFD, 0xDE,
		0xA9, 0x03, 0x1A, 0xC6, 0x3B, 0xBA, 0x73, 0xFD, 0xC5, 0x64, 0x44, 0x59, 0xEF, 0xCE, 0x14, 0xD1, 0x49,
		0xC3, 0x95, 0xC9, 0x3C, 0xDE, 0x77, 0x38, 0x05, 0x21, 0x93, 0x92, 0x63, 0x89, 0x49, 0xA5, 0xC9, 0x41,
		0x33, 0x35, 0x42, 0x0D, 0xE4, 0x15, 0xB6, 0x67, 0x9F, 0xDC, 0x9B, 0xD1, 0x04, 0x8D, 0x36, 0xEB, 0x34,
		0x0C, 0x78, 0xCB, 0x5F, 0x74, 0x22, 0x53, 0x45, 0xCF, 0x97, 0xB2, 0x61, 0xC4, 0x88, 0xC2, 0x77, 0xEC,
		0x4D, 0x22, 0xBB, 0x00, 0x00, 0x48, 0x4B, 0x98, 0xA0, 0x68, 0xAD, 0xA8, 0x1D, 0x78, 0x6E, 0xDF, 0xAE,
		0xD2, 0xE0, 0xEB, 0x7B, 0x1E, 0xFF, 0x53, 0xC6, 0x08, 0x63, 0x46, 0xDB, 0x2B, 0x1B, 0x49, 0xDB, 0x9E,
		0x2B, 0x6F, 0xCE, 0x55, 0xBD, 0xAC, 0x05, 0xF9, 0x4E, 0x2B, 0x68, 0x31, 0x2B, 0x89, 0xD5, 0x4C, 0x0E,
		0xAB, 0x26, 0xD4, 0x02, 0xA6, 0x82, 0x08, 0x25, 0xA9, 0x05, 0x91, 0x6A, 0x59, 0xD9, 0x62, 0xD7, 0xD3,
		0xA9, 0x35, 0xAF, 0xD9, 0x5E, 0xF5, 0xA2, 0xF1, 0xB9, 0xAA, 0x04, 0x05, 0x40, 0xF9, 0xE5, 0xD6, 0x34,
		0x91, 0x3C, 0x91, 0xD0, 0x28, 0x28, 0x31, 0x90, 0xD8, 0x48, 0x98, 0x99, 0x14, 0x75, 0x6A, 0x60, 0x75,
		0x66, 0x7C, 0xA7, 0x15, 0x69, 0xAA, 0x13, 0x86, 0x10, 0xBA, 0x1B, 0x81, 0xF0, 0x64, 0xDA, 0x30, 0x94,
		0x06, 0xC5, 0x35, 0x8A, 0x19, 0x21, 0x71, 0xDE, 0x3C, 0x36, 0x7A, 0xA4, 0xF0, 0x5C, 0x17, 0x67, 0xC2,
		0xAD, 0xB5, 0xD7, 0x66, 0x37, 0x35, 0x85, 0x57, 0xA6, 0x96, 0xBF, 0xFF, 0xFC, 0x67, 0x28, 0xBE, 0xB4,
		0x5B, 0xA8, 0x4A, 0x44, 0x69, 0x18, 0x51, 0xAF, 0x5E, 0x69, 0xAC, 0xAC, 0x16, 0x4B, 0x7D, 0xC4, 0x34,
		0xB2, 0x3A, 0xEE, 0xE9, 0xD0, 0x0D, 0x14, 0x73, 0x77, 0xEB, 0x8D, 0x57, 0x8A, 0x67, 0x2F, 0x7D, 0xC0,
		0x15, 0x6C, 0xA6, 0xF4, 0x74, 0x46, 0xEB, 0xDD, 0x65, 0x4D, 0xFE, 0xF4, 0xD8, 0x46, 0x0A, 0xD4, 0x1A,
		0x9E, 0x62, 0x28, 0x2C, 0x8A, 0x05, 0x17, 0x61, 0x18, 0x04, 0x9E, 0xE3, 0x39, 0x3E, 0x20, 0xF9, 0x42,
		0x4E, 0xEB, 0xB0, 0x75, 0xC0, 0x0C, 0xF9, 0xED, 0x8D, 0xD9, 0x6A, 0x4F, 0x69, 0xDB, 0xDE, 0x83, 0xBD,
		0xF5, 0x9A, 0xDB, 0xDD, 0x97, 0x54, 0x11, 0x6F, 0x2D, 0xD6, 0x91, 0x6D, 0x41, 0x26, 0x65, 0x60, 0x95,
		0x20, 0x84, 0x84, 0xE0, 0x48, 0x4A, 0x00, 0x59, 0x59, 0x5C, 0xA6, 0x5B, 0xCD, 0x5C, 0x92, 0x24, 0x13,
		0x2A, 0x26, 0x20, 0xA3, 0x16, 0xAF, 0xD5, 0x45, 0x18, 0xA6, 0x8B, 0x85, 0x89, 0x5B, 0x33, 0x2D, 0xCF,
		0x1D, 0xBF, 0x36, 0xB5, 0xE2, 0x11, 0x27, 0xE2, 0xA0, 0x6A, 0x8A, 0x95, 0x4E, 0xD4, 0xBD, 0x88, 0x10,
		0x25, 0x9D, 0xEF, 0x22, 0x80, 0x69, 0x2B, 0xD8, 0xDF, 0x6D, 0xF4, 0x15, 0x2C, 0x0B, 0xB8, 0xC4, 0x40,
		0x54, 0x15, 0xF9, 0xED, 0x68, 0xA3, 0x9A, 0xE8, 0x2E, 0x74, 0x6E, 0xCF, 0x84, 0x65, 0xE7, 0xDF, 0x09,
		0x17, 0x67, 0x6C, 0xCF, 0x0F, 0xFF, 0x07, 0xB1, 0xF4, 0x5B, 0x23, 0x9A, 0x96, 0xE4, 0x04, 0x00, 0x00,
		0x00, 0x25, 0x74, 0x45, 0x58, 0x74, 0x64, 0x61, 0x74, 0x65, 0x3A, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65,
		0x00, 0x32, 0x30, 0x31, 0x35, 0x2D, 0x30, 0x36, 0x2D, 0x30, 0x31, 0x54, 0x31, 0x35, 0x3A, 0x34, 0x36,
		0x3A, 0x35, 0x33, 0x2B, 0x30, 0x32, 0x3A, 0x30, 0x30, 0xEC, 0xA2, 0xBF, 0x5F, 0x00, 0x00, 0x00, 0x25,
		0x74, 0x45, 0x58, 0x74, 0x64, 0x61, 0x74, 0x65, 0x3A, 0x6D, 0x6F, 0x64, 0x69, 0x66, 0x79, 0x00, 0x32,
		0x30, 0x31, 0x34, 0x2D, 0x31, 0x30, 0x2D, 0x32, 0x33, 0x54, 0x31, 0x38, 0x3A, 0x30, 0x33, 0x3A, 0x30,
		0x37, 0x2B, 0x30, 0x32, 0x3A, 0x30, 0x30, 0x8C, 0x83, 0x14, 0x1D, 0x00, 0x00, 0x00, 0x11, 0x74, 0x45,
		0x58, 0x74, 0x6A, 0x70, 0x65, 0x67, 0x3A, 0x63, 0x6F, 0x6C, 0x6F, 0x72, 0x73, 0x70, 0x61, 0x63, 0x65,
		0x00, 0x32, 0x2C, 0x75, 0x55, 0x9F, 0x00, 0x00, 0x00, 0x20, 0x74, 0x45, 0x58, 0x74, 0x6A, 0x70, 0x65,
		0x67, 0x3A, 0x73, 0x61, 0x6D, 0x70, 0x6C, 0x69, 0x6E, 0x67, 0x2D, 0x66, 0x61, 0x63, 0x74, 0x6F, 0x72,
		0x00, 0x32, 0x78, 0x32, 0x2C, 0x31, 0x78, 0x31, 0x2C, 0x31, 0x78, 0x31, 0x49, 0xFA, 0xA6, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
)

func exampleHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/html":
		w.Header().Set("Content-Type", "text/html")
		http.SetCookie(w, &http.Cookie{Name: "SessionID", Value: "deadbeef1234",
			Path: "/", HttpOnly: true})
		w.WriteHeader(200)
		w.Write(exampleHTML)
	case "/other":
		w.Header().Set("Content-Type", "text/plain")
		w.Header()["Set-Cookie"] = []string{
			"cip=weLoriFTucvNmreuJSh43QM8957; Path=/; Max-Age=3600; HttpOnly",
			"tzu=; Max-Age=-1",
		}
		w.WriteHeader(200)
		w.Write([]byte("Some other document."))
	case "/json":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(exampleJSON)
	case "/xml":
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("X-Licence", "BSD-3")
		w.WriteHeader(200)
		w.Write(exampleXML)
	case "/lena":
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(200)
		w.Write(exampleImage)
	case "/post":
		if r.Method != http.MethodPost {
			http.Error(w, "No, I won't do that.", http.StatusMethodNotAllowed)
		}
		w.WriteHeader(200)
		body, _ := ioutil.ReadAll(r.Body)
		w.Write(body)
	case "/redirect2":
		w.Header().Set("Location", "/redirect1")
		w.WriteHeader(http.StatusSeeOther) // 303
	case "/redirect1":
		w.Header().Set("Location", "/html")
		w.WriteHeader(http.StatusMovedPermanently) // 302
	default:
		http.Error(w, "Oooops", http.StatusNotFound)
	}
}

// ----------------------------------------------------------------------------
// Tests

func allTestExamples() []string {
	internal := []string{
		"Test",
		"Test.HTML",
		"Test.JSON",
		"Test.POST",
		"Test.POST.FileUpload",
		"Test.POST.ManualBody",
		"Test.POST.BodyFromFile",
		"Test.Redirection",
		"Test.FollowRedirect",
		"Test.Image",
		"Test.Cookies",
		"Test.XML",
		"Test.Mixin",
		"Test.Retry",
		"Test.Extraction",
		"Test.Extraction.JSON",
		"Test.Extraction.HTML",
		"Test.CurrentTime",
		"Test.AndOr",
		"Test.Header",
		"Test.NoneHTTP",
		"Test.NoneHTTP.Bash",
		"Test.NoneHTTP.FileWrite",  // Write must go first...
		"Test.NoneHTTP.FileRead",   // .. then the file can be read
		"Test.NoneHTTP.FileDelete", // and we clean up.
	}

	if *runmysql {
		internal = append(internal, []string{
			"Test.NoneHTTP.SQLExec",
			"Test.NoneHTTP.SQLQuery",
		}...)
	}

	if os.Getenv("TRAVIS_GO_VERSION") == "" {
		internal = append(internal, "Test.Speed")
	}

	return internal
}

func TestExampleTest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(exampleHandler))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	variablesFlag["HOST"] = u.Host
	outputDir = "example-tests"
	randomSeed = 57 // must be prime
	silent = true
	ssilent = true

	for _, testname := range allTestExamples() {
		t.Run(testname, func(t *testing.T) {
			// Can be read in raw form:
			rawtests, err := loadTests([]string{"./examples/" + testname})
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			// Wrap into autogenerated suite
			s := &suite.RawSuite{
				File: &suite.File{
					Data: "---",
					Name: "<internal>",
				},
				Name:      testname,
				Main:      []suite.RawElement{{}}, // dummy
				Variables: variablesFlag,
			}

			// Suite validates:
			s.AddRawTests(rawtests...)
			err = s.Validate(variablesFlag)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			// Execute the suite:
			prepareHT()
			prepareOutputDir()
			suites := []*suite.RawSuite{s}
			acc, err := executeSuites(suites, variablesFlag, nil)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			if acc.Status != ht.Pass {
				t.Fatalf("Test %s did not pass: %s",
					testname, acc.Status)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Suites

var suiteExampleTests = []struct {
	file string
	want ht.Status
}{
	{"Suite", ht.Pass},
	{"Suite.InlineTest", ht.Pass},
	{"Suite.Mock", ht.Fail},
	{"Suite.Variables", ht.Pass},
}

func TestExampleSuite(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(exampleHandler))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	variablesFlag["HOST"] = u.Host
	outputDir = "example-tests"
	randomSeed = 57 // must be prime
	silent = true
	ssilent = true

	for _, tc := range suiteExampleTests {
		suitename := tc.file
		t.Run(suitename, func(t *testing.T) {
			// Can be read in raw form:
			suites, err := loadSuites([]string{"./examples/" + suitename})
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			// Execute the suite:
			prepareHT()
			prepareOutputDir()
			acc, err := executeSuites(suites, variablesFlag, nil)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			if acc.Status != tc.want {
				t.Fatalf("Got %s, want %s", acc.Status, tc.want)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Mocks

var mockExampleTests = []string{
	"Mock",
	"Mock.Check",
	"Mock.Dynamic",
	"Mock.Dynamic.Body",
	"Mock.Dynamic.Complex",
	"Mock.Dynamic.Timestamps",
}

func TestExampleMock(t *testing.T) {
	for _, mockname := range mockExampleTests {
		t.Run(mockname, func(t *testing.T) {
			raw, err := suite.LoadRawMock("./examples/"+mockname, nil)
			if err != nil {
				t.Fatal(err)
			}
			mockScope := scope.New(nil, raw.Variables, false)
			mockScope["MOCK_DIR"] = raw.Dirname()
			mockScope["MOCK_NAME"] = raw.Basename()
			_, err = raw.ToMock(mockScope, false)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Load

var loadExampleTests = []string{
	"Load",
}

func TestExampleLoad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(exampleHandler))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	variablesFlag["HOST"] = u.Host
	outputDir = "example-tests"
	randomSeed = 57 // must be prime
	silent = true
	ssilent = true

	prepareOutputDir()

	for _, loadname := range loadExampleTests {
		t.Run(loadname, func(t *testing.T) {
			raw, err := readRawLoadtest("./examples/" + loadname)
			if err != nil {
				t.Fatal(err)
			}
			scenarios := raw.ToScenario(variablesFlag)
			prepareHT()
			opts := suite.ThroughputOptions{
				Rate:         50,              //  \
				Duration:     5 * time.Second, //   > ~ 200 request
				Ramp:         2 * time.Second, //  /
				CollectFrom:  ht.Error,
				MaxErrorRate: -1,
			}
			data, failures, lterr := suite.Throughput(scenarios, opts, nil)
			saveLoadtestData(data, failures, scenarios)
			if lterr != nil && os.Getenv("TRAVIS_GO_VERSION") == "" {
				t.Fatal(lterr)
			}
			if len(failures.Tests) > 0 {
				t.Fatal(failures.Tests)
			}
			if len(data) < 130 || len(data) > 280 {
				t.Fatalf("Got %d request", len(data))
			}
		})
	}
}
