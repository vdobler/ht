package hjson

import (
	"errors"
	"math"
	"strconv"
)

type parseNumber struct {
	data []byte
	at   int  // The index of the current character
	ch   byte // The current character
}

func (p *parseNumber) next() bool {
	// get the next character.
	len := len(p.data)
	if p.at < len {
		p.ch = p.data[p.at]
		p.at++
		return true
	}
	if p.at == len {
		p.at++
		p.ch = 0
	}
	return false
}

func startsWithNumber(text []byte) bool {
	if _, err := tryParseNumber(text, true); err == nil {
		return true
	}
	return false
}

func tryParseNumber(text []byte, stopAtNext bool) (interface{}, error) {
	// Parse a number value.

	isInt, isNeg := true, false

	p := parseNumber{text, 0, ' '}
	leadingZeros := 0
	testLeading := true
	p.next()
	if p.ch == '-' {
		isNeg = true
		p.next()
	}
	for p.ch >= '0' && p.ch <= '9' {
		if testLeading {
			if p.ch == '0' {
				leadingZeros++
			} else {
				testLeading = false
			}
		}
		p.next()
	}
	if testLeading {
		leadingZeros--
	} // single 0 is allowed
	if p.ch == '.' {
		isInt = false
		for p.next() && p.ch >= '0' && p.ch <= '9' {
		}
	}
	if p.ch == 'e' || p.ch == 'E' {
		isInt = false
		p.next()
		if p.ch == '-' || p.ch == '+' {
			p.next()
		}
		for p.ch >= '0' && p.ch <= '9' {
			p.next()
		}
	}

	end := p.at

	// skip white/to (newline)
	for p.ch > 0 && p.ch <= ' ' {
		p.next()
	}

	if stopAtNext {
		// end scan if we find a punctuator character like ,}] or a comment
		if p.ch == ',' || p.ch == '}' || p.ch == ']' ||
			p.ch == '#' || p.ch == '/' && (p.data[p.at] == '/' || p.data[p.at] == '*') {
			p.ch = 0
		}
	}

	if p.ch > 0 || leadingZeros != 0 {
		return 0, errors.New("Invalid number")
	}

	s := string(p.data[0 : end-1])
	if isInt {
		// Well, JSON knows about floats only, but this is a mistake
		// made because of JavaScript.  No need to duplicate this
		// mistake here:  Return int64 where possible, switching to
		// uint64 if needed.
		if isNeg {
			i64, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return int64(0), err
			}
			return i64, nil
		} else {
			ui64, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return int64(0), err
			}
			if ui64 <= math.MaxInt64 {
				return int64(ui64), nil
			}
			return ui64, nil
		}
	}

	number, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if math.IsInf(number, 0) || math.IsNaN(number) {
		return 0, errors.New("Invalid number")
	}
	return number, nil
}
