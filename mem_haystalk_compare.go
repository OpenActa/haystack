// OpenActa/Haystack - mem structure compare methods
// Copyright (C) 2023 Arjen Lentz & Lentz Pty Ltd; All Rights Reserved
// <arjen (at) openacta (dot) dev>

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package haystack

import (
	"strconv"
	"strings"
)

// Function to compare an int with a Tuple value
func (p *Haystalk) CompareInt(i int64) (int, bool) {
	switch p.val.valtype {
	case valtype_int:
		// drop out so we can re-use the code for string

	case valtype_float:
		if float64(i) > p.val.floatval {
			return 1, true
		} else if float64(i) > p.val.floatval {
			return -1, true
		} else {
			return 0, true
		}

	case valtype_string:
		i2, err := strconv.Atoi(*p.val.stringval)
		if err != nil {
			return 0, false
		}
		i = int64(i2)
		// drops out of switch to int compare

	default:
		return 0, false
	}

	if i > p.val.intval {
		return 1, true
	} else if i < p.val.intval {
		return -1, true
	} else {
		return 0, true
	}
}

// Function to compare a float with a Tuple value
func (p *Haystalk) CompareFloat(f float64) (int, bool) {
	switch p.val.valtype {
	case valtype_int:
		if f > p.val.floatval {
			return 1, true
		} else if f < p.val.floatval {
			return -1, true
		} else {
			return 0, true
		}

	case valtype_float:
		// drop out so we can re-use the code for string

	case valtype_string:
		f2, err := strconv.ParseFloat(*p.val.stringval, 64)
		if err != nil {
			return 0, false
		}
		f = f2
		// drops out of switch to float compare

	default:
		return 0, false
	}

	if float64(f) > p.val.floatval {
		return 1, true
	} else if float64(f) < p.val.floatval {
		return -1, true
	} else {
		return 0, true
	}
}

// Function to compare a string with a Tuple value
func (p *Haystalk) CompareString(s string) (int, bool) {
	var s2 string

	switch p.val.valtype {
	case valtype_int:
		s2 = strconv.FormatInt(p.val.intval, 10)
		// drops out of switch to string compare

	case valtype_float:
		s2 = strconv.FormatFloat(p.val.floatval, 'f', -1, 64)
		// drops out of switch to string compare

	case valtype_string:
		// Check for exact UTF-8 match (case-insensitive)
		// https://pkg.go.dev/strings#EqualFold
		if strings.EqualFold(s, *p.val.stringval) {
			return 0, true
		}

		// Or do it the long way.
		s = strings.ToLower(s)
		s2 = strings.ToLower(*p.val.stringval)
		// drops out of switch to string compare

	default:
		return 0, false
	}

	if s > s2 {
		return 1, true
	} else if s < s2 {
		return -1, true
	} else {
		return 0, false
	}
}

// EOF
