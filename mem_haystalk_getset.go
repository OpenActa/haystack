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

type Integer struct {
	value int64
}

func (p *Integer) GetInt() int64 {
	return int64(p.value)
}

func (p *Integer) SetInt(i int64) bool {
	p.value = int64(i)
	return true
}

type Float struct {
	value float64
}

func (p *Float) GetFloat() float64 {
	return float64(p.value)
}

func (p *Float) SetFloat(f float64) bool {
	p.value = f
	return true
}

type String struct {
	value *string
}

func (p *String) GetString() string {
	return *p.value
}

func (p *String) SetString(s *string) bool {
	p.value = s
	return true
}

// EOF
