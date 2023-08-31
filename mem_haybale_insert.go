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
	"log"
	"sort"
	"strconv"
	"time"
)

// Helper function for InsertBunch() below
// Inserts a new stalk (KV entry) and returns its own offset
// (haystalk_ofs_nil for error -> ignore)
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
	size := uint32(37) // Haystalk struct, approx
	if newstalk.val.valtype == valtype_string {
		size += uint32(2 + len(*newstalk.val.stringval))
	}
	// Update size of current Haybale.
	p.Memsize += size
	// Also update Haystack size. TODO: this needs a better approach
	d.HaystackPtr.memsize += size

	// These two get filled later by the caller, but we don't leave them at 0
	// because that is a valid offset.
	newstalk.first_ofs = haystalk_ofs_nil
	newstalk.next_ofs = haystalk_ofs_nil

	// Finally, insert at the correct position
	pos := p.num_haystalks
	newstalk.self_ofs = pos // This is used during sorting
	p.haystalk[pos] = &newstalk
	p.num_haystalks++

	return pos
}

// Insert a bunch (aka a "record") of KV entries
func (p *HaystackRoutinesType) InsertBunch(flatmap map[string]interface{}) {
	var first, prev uint32

	if p.writer_cur_haybale.is_sorted_immutable {
		// We can't break this haybale from being immutable
		log.Printf("Cannot insert to immutable Haybale")
		// TODO: return some error condition
		return
	}

	// We need to mutex here, otherwise Newhaybale() can make us bomb out
	HaystackRoutines.newhaybale_mutex.Lock()
	// Note that we don't defer but explicitly unlock later, so don't return without unlocking.

	if _, ok := flatmap[Timestamp_key]; !ok {
		return // Just ignore this bunch if there's no _timestamp field
		// TODO: generate a _timestamp and insert that way?
	} else {
		// add the first tuple (_timestamp)
		vs := fmt.Sprintf("%v", flatmap[Timestamp_key]) // TODO improve this construct
		first = p.writer_cur_haybale.insertStalk(&p.writer_cur_haystack.Dict, Timestamp_key, vs)
		// We need to do this here as _timestamp is skipped in the loop below
		p.writer_cur_haybale.haystalk[first].first_ofs = first // first field (_timestamp) points to self

		/*
			Update time_first and time_last (in nsecs) in our record.
			This is somewhat tricky as we'll need to parse the time string.
			What format will it have?
			TODO: we should support multiple formats.
		*/
		t, err := time.Parse(time.RFC3339Nano, vs)
		if err != nil { // Try to parse
			t, err = time.Parse("2006-01-02T15:04:05.999999999+0000", vs)
			if err != nil {
				log.Printf("Can't parse timestamp '%s': %v", vs, err)
				panic("Aawrgh!")
			}
		}

		ts := t.UnixNano() // Convert to Unix nanosecond timestamp

		if p.writer_cur_haybale.time_first == 0 || ts < p.writer_cur_haybale.time_first {
			p.writer_cur_haybale.time_first = ts // Update lowest if lower
		}
		if ts > p.writer_cur_haybale.time_last {
			p.writer_cur_haybale.time_last = ts // Update highest if higher
		}

	}

	// Now insert all the KV pairs as stalks

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
			pos := p.writer_cur_haybale.insertStalk(&p.writer_cur_haystack.Dict, k, vs)
			if pos != haystalk_ofs_nil {
				p.writer_cur_haybale.haystalk[pos].first_ofs = first // Point to first (_timestamp) field
				p.writer_cur_haybale.haystalk[pos].next_ofs = prev   // Make a backwards chain of fields
				prev = pos                                           // On to next
			}
		}
	}

	p.writer_cur_haybale.haystalk[first].next_ofs = prev // Put _timestamp field in front of the rest

	// Do this before checking our limits and possible messenging to diskwriter thread
	HaystackRoutines.newhaybale_mutex.Unlock()

	// Check whether we want to flush, based on configured thresholds
	if config.haystack_wait_maxsize > 0 &&
		HaystackRoutines.writer_cur_haystack.memsize >= config.haystack_wait_maxsize {
		p.FlushHaystack() // Send msg to diskwriter thread
	} else {
		if config.haybale_wait_minsize > 0 &&
			HaystackRoutines.writer_cur_haybale.Memsize >= config.haybale_wait_minsize {
			p.FlushHaybale() // Send msg to diskwriter thread
		}
	}
}

/*
// Sort all haybales
func (p *HaystackRoutinesType) SortAllBales() {
	//log.Printf("Sorting all (%d) haybale(s)...", len(p.writer_cur_haystack.Haybale)) // DEBUG
	// Start the clock
	//start := time.Now() // DEBUG

	for i := range p.writer_cur_haystack.Haybale {
		p.writer_cur_haystack.Haybale[i].SortBale()
	}

	//duration := time.Since(start)	// DEBUG
	//log.Printf("Haybale sort duration: %v", duration)	// DEBUG
}
*/

// Sort a Haybale, if needed. At this point we also de-dup strings
func (p *Haybale) SortBale() {
	if p.is_sorted_immutable {
		return // Nothing to do, is already sorted!
	}

	//log.Printf("Running the Go garbage collector")	// DEBUG
	//runtime.GC() // Force garbage collector to run all the way, to ensure we measure de-dup cleanly

	// log.Printf("Haybale sort & de-dup") // DEBUG
	//var m runtime.MemStats
	//runtime.ReadMemStats(&m)
	//oldalloc := m.HeapAlloc / (1024 * 1024)

	// Sort: using standard Go lib sorting function for now, with a closure.
	// If we used our own sorting function, we could take "self_ofs" out and save memory...
	// (now it needs to be part of the same struct. Other optimisations may also be possible.)
	sort.Slice(p.haystalk, func(p1, p2 int) bool {
		return p.haystalk[p1].Compare(*p.haystalk[p2]) < 0
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
					We re-assign to the shared string pointer, removing the
					last reference to the original string.
					The Go garbage collector should catch it from there.
				*/
				p.haystalk[i].val.stringval = prev_string
				p.Memsize -= uint32(len(*prev_string))
				//log.Printf("Dedup %s, saved %d bytes", *prev_string, len(*prev_string))	// DEBUG
			} else {
				prev_string = p.haystalk[i].val.stringval
			}
		}

	}

	p.is_sorted_immutable = true // Says that this haybale is sorted

	//runtime.GC() // Force garbage collector to run all the way, to measure what the de-dup accomplishes
	//runtime.ReadMemStats(&m)
	//newalloc := m.HeapAlloc / (1024 * 1024)
	//totalalloc := m.TotalAlloc / (1024 * 1024)
	//log.Printf("num_haystalks = %v, Heap was=%dM, now=%dM (%.02f%% gain), TotalAlloc=%dM",	// DEBUG
	//	p.num_haystalks, oldalloc, newalloc, 100.0-((float32(newalloc)/float32(oldalloc))*100.0), totalalloc)
}

// EOF
