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

// Compare dkey,valtype,val (hv) with a stored Haystalk
func (p *Haystalk) Compare(hv Haystalk) int {
	// Check dkey
	//fmt.Fprintf(os.Stderr, "Comparing dkey %d | %d\n", p.dkey, hv.dkey) // DEBUG
	if p.dkey > hv.dkey {
		return 1
	} else if p.dkey < hv.dkey {
		return -1
	}
	// same dkey

	// Check value type
	//fmt.Fprintf(os.Stderr, "Comparing valtype %d | %d\n", p.val.valtype, hv.val.valtype) // DEBUG
	if p.val.valtype > hv.val.valtype {
		return 1
	} else if p.val.valtype < hv.val.valtype {
		return -1
	}
	// same type

	// Check value
	switch p.val.valtype {
	case valtype_int:
		i1 := p.val.GetInt()
		i2 := hv.val.GetInt()
		if i1 > i2 {
			return 1
		} else if i1 < i2 {
			return -1
		} else {
			return 0
		}
	case valtype_float:
		f1 := p.val.GetFloat()
		f2 := hv.val.GetFloat()
		if f1 > f2 {
			return 1
		} else if f1 < f2 {
			return -1
		} else {
			return 0
		}
	case valtype_string:
		sv1 := *p.val.GetString()
		sv2 := *hv.val.GetString()
		//fmt.Fprintf(os.Stderr, "Comparing string %s | %s\n", sv1, sv2) // DEBUG

		// Check for exact UTF-8 match (case-insensitive)
		// https://pkg.go.dev/strings#EqualFold
		if strings.EqualFold(sv1, sv2) {
			return 0
		}

		// Or do it the long way.
		sv1 = strings.ToLower(sv1)
		sv2 = strings.ToLower(sv2)

		if sv1 > sv2 {
			return 1
		} else if sv1 < sv2 {
			return -1
		} else {
			return 0
		}
	default:
		panic("Compare function fail")
	}
}

// Function to compare an int with a Haystalk value
func (p *Haystalk) CompareInt(i int64) (int, bool) {
	switch p.val.valtype {
	case valtype_int:
		// drop out so we can re-use the code for string

	case valtype_float:
		if float64(i) > p.val.GetFloat() {
			return 1, true
		} else if float64(i) < p.val.GetFloat() {
			return -1, true
		} else {
			return 0, true
		}

	case valtype_string:
		i2, err := strconv.Atoi(*p.val.GetString())
		if err != nil {
			return 0, false
		}
		i = int64(i2)
		// drops out of switch to int compare

	default:
		return 0, false
	}

	if i > p.val.GetInt() {
		return 1, true
	} else if i < p.val.GetInt() {
		return -1, true
	} else {
		return 0, true
	}
}

// Function to compare a float with a Haystalk value
func (p *Haystalk) CompareFloat(f float64) (int, bool) {
	switch p.val.valtype {
	case valtype_int:
		if f > p.val.GetFloat() {
			return 1, true
		} else if f < p.val.GetFloat() {
			return -1, true
		} else {
			return 0, true
		}

	case valtype_float:
		// drop out so we can re-use the code for string

	case valtype_string:
		f2, err := strconv.ParseFloat(*p.val.GetString(), 64)
		if err != nil {
			return 0, false
		}
		f = f2
		// drops out of switch to float compare

	default:
		return 0, false
	}

	if float64(f) > p.val.GetFloat() {
		return 1, true
	} else if float64(f) < p.val.GetFloat() {
		return -1, true
	} else {
		return 0, true
	}
}

// Function to compare a string with a Haystalk value
func (p *Haystalk) CompareString(s *string) (int, bool) {
	var sv2 string

	switch p.val.valtype {
	case valtype_int:
		sv2 = strconv.FormatInt(p.val.GetInt(), 10)
		// drops out of switch to string compare

	case valtype_float:
		sv2 = strconv.FormatFloat(p.val.GetFloat(), 'f', -1, 64)
		// drops out of switch to string compare

	case valtype_string:
		//fmt.Fprintf(os.Stderr, "Comparing %s | %s\n", *s, *p.val.GetString()) // DEBUG

		// Check for exact UTF-8 match (case-insensitive)
		// https://pkg.go.dev/strings#EqualFold
		if strings.EqualFold(*s, *p.val.GetString()) {
			return 0, true
		}

		// Or do it the long way.
		sv2 = strings.ToLower(*p.val.GetString())
		// drops out of switch to string compare

	default:
		return 0, false
	}

	sv := strings.ToLower(*s)

	if sv > sv2 {
		return 1, true
	} else if sv < sv2 {
		return -1, true
	} else {
		return 0, false
	}
}

// EOF
