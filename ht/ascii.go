// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"strings"

	"golang.org/x/text/encoding/unicode"
)

// Underline title with c, indented by prefix.
//     Title
//     ~~~~~
func Underline(title string, c string, prefix string) string {
	// TODO: add at least some math
	return prefix + title + "\n" + prefix + strings.Repeat(c, len(title))[:len(title)]
}

// BoldBox around title, indented by prefix.
//    #################
//    ##             ##
//    ##    Title    ##
//    ##             ##
//    #################
func BoldBox(title string, prefix string) string {
	n := len(title)
	top := prefix + strings.Repeat("#", n+12)
	pad := prefix + "##" + strings.Repeat(" ", n+8) + "##"
	return fmt.Sprintf("%s\n%s\n%s##    %s    ##\n%s\n%s", top, pad, prefix, title, pad, top)
}

// Box around title, indented by prefix.
//    +------------+
//    |    Title   |
//    +------------+
func Box(title string, prefix string) string {
	n := len(title)
	top := prefix + "+" + strings.Repeat("-", n+6) + "+"
	return fmt.Sprintf("%s\n%s|   %s   |\n%s", top, prefix, title, top)
}

// Summary pretty-prints s and trimms it if too long.
// What is considered "too long" depends on the media type which is automatically
// detected from s. Currently only JSON is pretty printed. Media types which do
// not have obvious texttual representation are summariesed as the media type.
func Summary(s string) string {
	ct := http.DetectContentType([]byte(s))
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return ""
	}

	if charset, ok := params["charset"]; ok && (charset == "utf-16-be" || charset == "utf-16le") {
		encoding := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM)
		decoder := encoding.NewDecoder()
		sane, err := decoder.String(s)
		if err == nil {
			s = sane
		} else {
			log.Printf("encoding errors in %s: %s", mt, err)
		}
	}

	if strings.HasPrefix(mt, "image/") || strings.HasPrefix(mt, "audio/") {
		return typeIndicator(strings.ToUpper(mt[6:]))
	}

	switch mt {
	case "application/pdf":
		return typeIndicator("PDF")
	case "text/html":
		w := strings.Trim(s, "\n\r\t ")
		return clipLines(w, 40, 120)
	case "text/xml":
		w := strings.Trim(s, "\n\r\t ")
		// TODO: actuall do pretty print the XML.
		return clipLines(w, 80, 120)
	case "text/plain":
		// Try to prettyprint JSON
		var js map[string]interface{}
		err := json.Unmarshal([]byte(s), &js)
		if err == nil {
			res, err := json.MarshalIndent(js, "", "    ")
			if err == nil {
				return clipLines(string(res), 80, 180)
			}
		}
	}

	if strings.HasPrefix(mt, "text/") {
		return clipLines(s, 40, 150)
	}

	return typeIndicator(mt)
}

// clip lines to maxLines each of maxCols byte long.
func clipLines(s string, maxLines, maxCols int) string {
	lines := strings.Split(s, "\n")
	if n := len(lines); n > maxLines {
		a, e := 1+2*maxLines/3, maxLines/3
		lines[a] = "\u22EE"
		lines = append(lines[:a+1], lines[n-e:n]...)
	}

	for i := range lines {
		if len(lines[i]) > maxCols {
			lines[i] = lines[i][:maxCols] + "\u2026"
		}
	}
	return strings.Join(lines, "\n")
}

func typeIndicator(t string) string { return "\u2014\u2003" + t + "\u2003\u2014" }

// SummaryIsClipped return whether applying Summary to s
// will produce an clipped or pretty-printed output.
func SummaryIsClipped(s string) bool {
	summary := Summary(s)
	if strings.HasPrefix(s, "\u2014\u2003") &&
		strings.HasSuffix(s, "\u2003\u2014") {
		return false // Type indicators do not qualify as "clipped"
	}

	return summary != s
}
