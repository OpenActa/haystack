// OpenActa/Haystack - basic search
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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"
)

func (p *Haystack) SearchKeyVal(ks string, v string) {
	var matches uint
	var val Val

	fmt.Fprintf(os.Stderr, "Searching for key %s = %s\n", ks, v)

	// Start the clock
	start := time.Now()

	dkey, found := p.Dict.KeyExists(ks)
	if !found {
		fmt.Fprintf(os.Stderr, "Key '%s' not present in dataset\n", ks)
		return
	}

	// Figure out what type our value is (int, float or string)
	// We played with regexes first, but now we just rely on Go's own value format opinions
	if i, err := strconv.Atoi(v); err == nil {
		val.SetInt(int64(i))
	} else if f, err := strconv.ParseFloat(v, 64); err == nil {
		val.SetFloat(f)
	} else {
		// Not an int or float format, we'll make it a string then.
		val.SetString(&v)
	}

	// Run through all Haybales
	for i := range p.Haybale {
		cur_hb := p.Haybale[i]

		// Make sure the bale is sorted
		cur_hb.SortBale()

		// Check in each Haybale
		stalks := int(cur_hb.num_haystalks)

		fmt.Fprintf(os.Stderr, "Looking in Haybale %d (%d stalks)\n", i, stalks)

		/*
			We do a binary search within the Haybale.
			The sort.Search function returns the position the key would be (if it exists)
			or the length of the array if there's no match.
			We wrap all that in the for loop clause, with a closure.
			Consequently, for a match, we walk all the matches. Neat!
		*/
		for j := sort.Search(stalks, func(x int) bool {
			// TODO: use the HayStalk.Compare* functions here

			hsx := cur_hb.haystalk[x]

			// First, find matching dkey
			if hsx.dkey > dkey {
				return true
			} else if hsx.dkey < dkey {
				return false
			}

			// dkey equal, so search on valtype
			if hsx.val.valtype > val.valtype {
				return true
			} else if hsx.val.valtype < val.valtype {
				return false
			}

			// valtype also equal, so now search on value
			switch hsx.val.valtype {
			case valtype_int:
				return hsx.val.GetInt() >= val.intval
			case valtype_float:
				return hsx.val.GetFloat() >= val.floatval
			case valtype_string:
				return *hsx.val.GetString() >= *val.stringval
			default:
				return true // This shouldn't happen
			}
		}); j < stalks && cur_hb.haystalk[j].dkey == dkey && cur_hb.haystalk[j].val.valtype == val.valtype; j++ {
			// TODO: Use the HayStalk.Compare* functions here?

			// Check the key value
			var s string
			switch cur_hb.haystalk[j].val.valtype {
			case valtype_int:
				s = fmt.Sprintf("%d", cur_hb.haystalk[j].val.GetInt())
			case valtype_float:
				s = fmt.Sprintf("%f", cur_hb.haystalk[j].val.GetFloat())
			case valtype_string:
				s = *cur_hb.haystalk[j].val.GetString()
			}
			if v != s { // Not a matching key value
				continue
			}

			// Got a match!
			matches++

			// Now it gets funky...
			// Go to first entry of this bunch, which is the _timestamp,
			// then walk the rest of the bunch.
			bunch := make(map[string]string)
			var spotted = false // Just a precaution against bugs
			for k := cur_hb.haystalk[j].first_ofs; k != haystalk_ofs_nil; k = cur_hb.haystalk[k].next_ofs {
				// Find our specific key
				if cur_hb.haystalk[k].dkey == dkey {
					spotted = true
				}

				switch cur_hb.haystalk[k].val.valtype {
				case valtype_int:
					s = fmt.Sprintf("%d", cur_hb.haystalk[k].val.GetInt())
				case valtype_float:
					s = fmt.Sprintf("%f", cur_hb.haystalk[k].val.GetFloat())
				case valtype_string:
					s = *cur_hb.haystalk[k].val.GetString()
				}

				bunch[*p.Dict.dkey[cur_hb.haystalk[k].dkey]] = s
			}

			if !spotted { // This shouldn't happen
				panic("Key not found in selected bunch!?")
			}

			bunch_json, _ := json.Marshal(bunch)
			fmt.Println(string(bunch_json))
		}
	}

	duration := time.Since(start)
	fmt.Fprintf(os.Stderr, "%d matches, duration: %v\n", matches, duration)
}

// EOF
