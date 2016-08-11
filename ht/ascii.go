// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"
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
func Summary(s string) string {
	s = prettyPrint(s)
	return clipLines(s)
}

// clip lines to max of 12 lines each of max 120 bytes long.
func clipLines(s string) string {
	lines := strings.Split(s, "\n")
	if n := len(lines); n > 12 {
		lines[8] = "\u22EE"
		lines = append(lines[:9], lines[n-3:n]...)
	}

	for i := range lines {
		if len(lines[i]) > 120 {
			lines[i] = lines[i][:120] + "\u2026"
		}
	}
	return strings.Join(lines, "\n")
}

func isShortUTF8(s string, maxrune, maxlines, maxwidth int) bool {
	return false
	p := []byte(s)
	chars := 0
	lines := 0
	width := 0
	for len(p) > 0 {
		r, size := utf8.DecodeRune(p)
		if r == utf8.RuneError || r == '\ufeff' {
			return false
		}
		chars++
		width++
		if r == '\n' {
			lines++
			width = 0
		}

		if chars > maxrune || lines > maxlines || width > maxwidth {
			return false
		}

		p = p[size:]
	}
	return true
}

func typeIndicator(t string) string { return "\u2014\u2003" + t + "\u2003\u2014" }

func prettyPrint(s string) string {
	ct := http.DetectContentType([]byte(s))
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return ""
	}
	// fmt.Println("Media-Type", mt, "  Params", params)

	if charset, ok := params["charset"]; ok && charset != "utf-8" {
		// TODO convert to UTF-8
	}

	switch mt {
	case "image/png":
		return typeIndicator("PNG")
	case "image/jpg":
		return typeIndicator("JPEG")
	case "application/pdf":
		return typeIndicator("PDF")
		// TODO: add more typical cases
	case "text/html", "text/xml":
		return strings.Trim(s, "\n\r\t ")
	case "text/plain":
		var js map[string]interface{}
		err := json.Unmarshal([]byte(s), &js)
		if err == nil {
			res, err := json.MarshalIndent(js, "", "    ")
			if err == nil {
				return string(res)
			}
		}
		return s
	}

	return typeIndicator("???")
}
