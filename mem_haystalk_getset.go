/// OpenActa/Haystack - mem structure access methods
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

func (p *Val) GetInt() int64 {
	// Catch the bad.
	if p.valtype != valtype_int {
		return 0
	}

	return p.intval
}

func (p *Val) SetInt(i int64) bool {
	p.valtype = valtype_int
	p.intval = i
	return true
}

func (p *Val) GetFloat() float64 {
	// Catch the bad.
	if p.valtype != valtype_float {
		return 0.0
	}

	return p.floatval
}

func (p *Val) SetFloat(f float64) bool {
	p.valtype = valtype_float
	p.floatval = f
	return true
}

func (p *Val) GetString() *string {
	// Catch the bad.
	if p.valtype != valtype_string {
		return nil
	}

	return p.stringval
}

func (p *Val) SetString(s *string) bool {
	p.valtype = valtype_string
	p.stringval = s
	return true
}

// EOF
