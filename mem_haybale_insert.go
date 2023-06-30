// OpenActa/Haystack - bunch/stalk insert handling
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

/*
	We use the same layout for insert as a mem-stored haybale record would
	have, however we don't keep it sorted. This is for optimum insert speed.
	And that approach is fine, since we don't search the latest haybale.
	Before we do search it, we sort it once (using a helper array). Easy.
*/

package haystack

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"time"
)

// Helper function for InsertBunch() below
// Inserts a new stalk and returns its own offset (0 for error -> ignore)
func (p *Haybale) insertStalk(d *Dictionary, k string, v string) uint32 {
	var newstalk Haystalk

	dkey, res := d.FindOrAddKeyhash(k)
	if !res {
		return haystalk_ofs_nil
	}
	newstalk.dkey = dkey

	// First figure out what type our value is (int, float or string)
	// We played with regexes first, but now we just rely on Go's own value format opinions
	if i, err := strconv.Atoi(v); err == nil {
		newstalk.val.SetInt(int64(i))
	} else if f, err := strconv.ParseFloat(v, 64); err == nil {
		newstalk.val.SetFloat(float64(f))
	} else {
		// Not an int or float format, we'll make it a string then.

		// First we check whether our json flatten function added blank arrays etc
		if v == "[]" || v == "map[]" {
			v = "" // TODO: this may not be ideal, but it's ok for now
		}

		// We use the pointer to the string, so we don't have to (re-)allocate it.
		newstalk.val.SetString(&v)
	}

	if p.num_haystalks > 0 {
		// Make space at the designated position (just a slice of pointers, fast)
		p.haystalk = append(p.haystalk, &Haystalk{})
	} else {
		// Set up a fresh haystalk slice with one entry, ready to be filled (below)
		p.haystalk = make([]*Haystalk, 1, cap_initial)
	}

	// Update memsize on the fly, otherwise it'd be too slow
	p.Memsize += 25 // Haystalk struct
	if newstalk.val.valtype == valtype_string {
		p.Memsize += uint32(2 + len(*newstalk.val.stringval))
	}

	// These two get filled later by the caller, but we don't leave them at 0
	// because that is a valid offset.
	newstalk.first_ofs = haystalk_ofs_nil
	newstalk.next_ofs = haystalk_ofs_nil

	// Finally, insert at the correct position
	pos := p.num_haystalks
	p.haystalk[pos] = &newstalk
	p.num_haystalks++
	p.is_sorted_immutable = false // This append makes the Haybale not sorted

	newstalk.self_ofs = pos // This is used during sorting

	return pos
}

// Insert a bunch (aka a "row") of KV entries
func (p *Haybale) InsertBunch(d *Dictionary, flatmap map[string]interface{}) {
	var first, prev uint32

	if p.is_sorted_immutable {
		// We can't break this haybale from being immutable
		fmt.Fprintf(os.Stderr, "Cannot insert to immutable Haybale\n")
		// TODO: return some error condition
		return
	}

	if _, ok := flatmap[Timestamp_key]; !ok {
		return // Just ignore this bunch if there's no _timestamp field
	} else {
		// add the first tuple (_timestamp)
		vs := fmt.Sprintf("%v", flatmap[Timestamp_key]) // TODO improve this construct
		first = p.insertStalk(d, Timestamp_key, vs)
		// We need to do this here as _timestamp is skipped in the loop below
		p.haystalk[first].first_ofs = first // first field (_timestamp) points to self

		/*
			Update time_first and time_last (in nsecs) in our record.
			This is somewhat tricky as we'll need to parse the time string.
			What format will it have? We should support multiple formats.
		*/
		if t, err := time.Parse(time.RFC3339Nano, vs); err == nil { // Try to parse
			ts := t.UnixNano() // Convert to Unix nanosecond timestamp

			if p.time_first == 0 || ts < p.time_first {
				p.time_first = ts // Update lowest if lower
			}
			if ts > p.time_last {
				p.time_last = ts // Update highest if higher
			}
		}
	}

	prev = haystalk_ofs_nil

	for k, v := range flatmap {
		if k != Timestamp_key {
			if len(k) == 0 {
				continue // ignore
			} else if len(k) > max_keylen {
				// TODO: handle this nicer
				panic(fmt.Sprintf("Key '%s' longer than %d chars", k, max_keylen))
			}

			// insert each tuple
			vs := fmt.Sprintf("%v", v) // TODO improve this construct
			pos := p.insertStalk(d, k, vs)
			if pos != haystalk_ofs_nil {
				p.haystalk[pos].first_ofs = first // Point to first (_timestamp) field
				p.haystalk[pos].next_ofs = prev   // Make a backwards chain of fields
				prev = pos                        // On to next
			}
		}
	}

	p.haystalk[first].next_ofs = prev // Put _timestamp field in front of the rest
}

// Sort all haybales
func (p *Haystack) SortAllBales() {
	fmt.Fprintf(os.Stderr, "Sorting all (%d) haybale(s)...\n", len(p.Haybale))
	// Start the clock
	start := time.Now()

	for i := range p.Haybale {
		p.Haybale[i].SortBale()
	}

	duration := time.Since(start)
	fmt.Fprintf(os.Stderr, "Haybale sort duration: %v\n", duration)
}

// Sort a Haybale, if needed. At this point we also de-dup strings
func (p *Haybale) SortBale() {
	if p.is_sorted_immutable {
		return // Nothing to do, is already sorted!
	}

	fmt.Fprintf(os.Stderr, "Running the Go garbage collector\n")
	runtime.GC() // Force garbage collector to run all the way, to ensure we measure de-dup cleanly

	fmt.Fprintf(os.Stderr, "Haybale sort & de-dup\n")
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	oldalloc := m.HeapAlloc / (1024 * 1024)

	// Sort: using build-in Go lib sorting function for now, with a closure.
	// If we used our own sorting function, we could take "self_ofs" out and save memory...
	// (now it needs to be part of the same struct. Other optimisations may also be possible.
	sort.Slice(p.haystalk, func(p1, p2 int) bool {
		// TODO: use the HayStalk.Compare* functions here

		// First, sort our slice by dkey
		if p.haystalk[p1].dkey < p.haystalk[p2].dkey {
			return true
		} else if p.haystalk[p1].dkey > p.haystalk[p2].dkey {
			return false
		}

		// dkey equal, so now sort by valtype
		if p.haystalk[p1].val.valtype < p.haystalk[p2].val.valtype {
			return true
		} else if p.haystalk[p1].val.valtype > p.haystalk[p2].val.valtype {
			return false
		}

		// valtype also equal, so now sort by value
		switch p.haystalk[p1].val.valtype {
		case valtype_int:
			return p.haystalk[p1].val.GetInt() < p.haystalk[p2].val.GetInt()
		case valtype_float:
			return p.haystalk[p1].val.GetFloat() < p.haystalk[p2].val.GetFloat()
		case valtype_string:
			return *p.haystalk[p1].val.GetString() < *p.haystalk[p2].val.GetString()
		default:
			return false // This shouldn't happen
		}
	})

	// Now we create a map where newold_map[i] points to its old self
	newold_map := make([]uint32, p.num_haystalks)
	for i := uint32(0); i < p.num_haystalks; i++ {
		if p.haystalk[i].self_ofs != haystalk_ofs_nil {
			newold_map[p.haystalk[i].self_ofs] = i
		}
	}

	// And lastly, we fix up all the pointers, and de-dup adjacent strings
	// So we only grab adjacent values for stalks in mem, like src_ip="xxx".
	// We score substantial gains out of this (approx 10%), without complexity.
	// For the disk representation, we can do something else.
	var prev_string *string
	for i := uint32(0); i < p.num_haystalks; i++ {
		if p.haystalk[i].first_ofs != haystalk_ofs_nil {
			p.haystalk[i].first_ofs = newold_map[p.haystalk[i].first_ofs]
		}
		if p.haystalk[i].next_ofs != haystalk_ofs_nil {
			p.haystalk[i].next_ofs = newold_map[p.haystalk[i].next_ofs]
		}

		// De-dup logic
		if p.haystalk[i].val.valtype == valtype_string {
			if prev_string == nil {
				prev_string = p.haystalk[i].val.stringval
			} else if *p.haystalk[i].val.stringval == *prev_string {
				/*
					We re-assign to the shared string pointer, getting rid
					of the original string pointer.
					The Go garbage collector should catch it from there.
				*/
				p.haystalk[i].val.stringval = prev_string
				p.Memsize -= uint32(len(*prev_string))
				//fmt.Fprintf(os.Stderr, "Dedup %s, saved %d bytes\n", *prev_string, len(*prev_string))	// DEBUG
			} else {
				prev_string = p.haystalk[i].val.stringval
			}
		}

	}

	p.is_sorted_immutable = true // Says that this haybale is sorted

	runtime.GC() // Force garbage collector to run all the way, to measure what the de-dup accomplishes
	runtime.ReadMemStats(&m)
	newalloc := m.HeapAlloc / (1024 * 1024)
	totalalloc := m.TotalAlloc / (1024 * 1024)
	fmt.Fprintf(os.Stderr, "num_haystalks = %v, Heap was=%dM, now=%dM (%.02f%% gain), TotalAlloc=%dM\n",
		p.num_haystalks, oldalloc, newalloc, 100.0-((float32(newalloc)/float32(oldalloc))*100.0), totalalloc)
}
