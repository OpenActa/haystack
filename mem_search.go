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

func (p *Haystack) SearchKeyValArray(kv_array map[string]string) {
	var matches uint

	// Start the clock
	start := time.Now()

	hv := make([]Haystalk, 0, len(kv_array))
	for ks, v := range kv_array {
		var new_hv Haystalk
		var found bool

		new_hv.dkey, found = p.Dict.KeyExists(ks)

		// doesn't exist, and it's an AND construct so we can just bail out
		if !found {
			fmt.Fprintf(os.Stderr, "Key '%s' not present in dataset\n", ks)
			return
		}

		// Figure out what type our first value is (int, float or string)
		if i, err := strconv.Atoi(v); err == nil {
			new_hv.val.SetInt(int64(i))
		} else if f, err := strconv.ParseFloat(v, 64); err == nil {
			new_hv.val.SetFloat(f)
		} else {
			// Not an int or float format, we'll make it a string then.
			vs := v // So the compiler allocates a new string
			new_hv.val.SetString(&vs)
			//fmt.Fprintf(os.Stderr, "New string = %s\n", *new_hv.val.GetString())	// DEBUG
		}

		hv = append(hv, new_hv)
	}

	fmt.Fprintf(os.Stderr, "Search conditions: hv = %v\n", hv) // DEBUG
	for i := 0; i < len(hv); i++ {
		fmt.Fprintf(os.Stderr, "[%d] %s=%s\n", i, *p.Dict.dkey[hv[i].dkey], *hv[i].val.GetString())
	}

	// Run through all Haybales
	for i := range p.Haybale {
		cur_hb := p.Haybale[i]

		// Make sure the bale is sorted
		//cur_hb.SortBale()					// DEBUG - not any more for normal ops
		if !cur_hb.is_sorted_immutable { // So obviously this should never happen.
			fmt.Fprintf(os.Stderr, "Haybale %d is not sorted, we can't search that!\n", i) // DEBUG
		}

		// Check in each Haybale
		stalks := int(cur_hb.num_haystalks)

		fmt.Fprintf(os.Stderr, "Looking in Haybale %d (%d stalks)\n", i, stalks)

		/*
			We do a binary search within the Haybale.
			The sort.Search (https://pkg.go.dev/sort#Search) function returns
			the position the key would be (if it exists), or the length of the
			array if there's no match.
			We wrap all that in the for loop clause, with a closure.
			Consequently, for a match, we walk all the matches. Neat!
		*/
	haystalk_loop:
		for j := sort.Search(stalks, func(x int) bool {
			// Since our data is sorted in ascending order, we search with <=
			res := hv[0].Compare(*cur_hb.haystalk[x])
			fmt.Fprintf(os.Stderr, "res=%d\n", res) // DEBUG
			if res <= 0 {
				return true
			} else {
				return false
			}
		}); j < stalks && cur_hb.haystalk[j].Compare(hv[0]) == 0; j++ {
			// ----
			if len(hv) > 1 {
				fmt.Fprintf(os.Stderr, "Part match: checking additional conditions\n")

				// Here we check for additional conditions (AND clause style)
				for k := 1; k < len(hv); k++ { // from 1: 2nd key/val onward
					cur_hv := hv[k]

					found := false
					for andi := cur_hb.haystalk[j].first_ofs; !found && andi != haystalk_ofs_nil; andi = cur_hb.haystalk[andi].next_ofs {
						if cur_hb.haystalk[andi].Compare(cur_hv) == 0 {
							found = true
						}
					}
					if !found { // No match for this entry, so we can shortcut out
						break haystalk_loop
					}
				}
			} else {
				fmt.Fprintf(os.Stderr, "No additional conditions\n")
			}
			// ----

			// Got a match!
			matches++

			// Now it gets funky...
			// Go to first entry of this bunch, which is the _timestamp,
			// then walk the rest of the bunch.
			bunch := make(map[string]string)
			var vs string
			for k := cur_hb.haystalk[j].first_ofs; k != haystalk_ofs_nil; k = cur_hb.haystalk[k].next_ofs {
				switch cur_hb.haystalk[k].val.valtype {
				case valtype_int:
					vs = fmt.Sprintf("%d", cur_hb.haystalk[k].val.GetInt())
				case valtype_float:
					vs = fmt.Sprintf("%f", cur_hb.haystalk[k].val.GetFloat())
				case valtype_string:
					vs = *cur_hb.haystalk[k].val.GetString()
				}

				bunch[*p.Dict.dkey[cur_hb.haystalk[k].dkey]] = vs
			}

			bunch_json, _ := json.Marshal(bunch)
			fmt.Println(string(bunch_json))
		}
	}

	duration := time.Since(start)
	fmt.Fprintf(os.Stderr, "%d matches, duration: %v\n", matches, duration)
}

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
		//cur_hb.SortBale()		// DEBUG - not any more for normal ops
		if !cur_hb.is_sorted_immutable { // So obviously this should never happen.
			panic("Haybale is not sorted, we can't search that!")
		}

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

			// Check the key value again here, so we can drop out when there are no more matches
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
				break
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
