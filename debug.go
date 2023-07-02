// OpenActa/Haystack test/debug functions
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
	"fmt"
	"os"
)

// Print Haybale (TEST/DEBUG purposes)
func (p *Haybale) PrintBale(d *Dictionary) {
	p.SortBale()

	for n := uint32(0); n < p.num_haystalks; n++ {
		if p.haystalk[n].first_ofs != n {
			continue
		}

		for r := p.haystalk[n].first_ofs; r != haystalk_ofs_nil; r = p.haystalk[r].next_ofs {
			if d.dkey[(*p.haystalk[r]).dkey] == nil { // DEBUG
				fmt.Fprintf(os.Stderr, "Assert: nil ptr from dkey %v\n", (*p.haystalk[r]).dkey)
				continue
			}
			fmt.Printf("%v=", *d.dkey[(*p.haystalk[r]).dkey])

			switch (*p.haystalk[r]).val.valtype {
			case valtype_int:
				fmt.Printf("%v", p.haystalk[r].val.GetInt())
			case valtype_float:
				fmt.Printf("%v", p.haystalk[r].val.GetFloat())
			case valtype_string:
				fmt.Printf("%v", *p.haystalk[r].val.GetString())
			}

			fmt.Printf("\n")
		}

		fmt.Printf("\n")
	}

	fmt.Printf("\n")
}

// EOF
