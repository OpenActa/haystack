// OpenActa/Haystack Dictionary
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
	https://pkg.go.dev/hash/fnv
	https://en.wikipedia.org/wiki/Fowler-Noll-Vo_hash_function
	Mind that the Go library doesn't have a 24-bit implementation, but we can derive.

	The distribution is ok-ish (no/few collisions), but lots of empties near the end?
	(Based on a test with /usr/share/dict/words)
*/

package haystack

import (
	"hash/fnv"
	"strings"
)

const (
	hash_skip       = 101        // May be a prime with reasonable dispersal properties?
	hashkey_mask    = 0x00ffffff // 24-bit
	hashkey_invalid = 0xffffffff
)

// This function will check whether a key exists in our hash table:
// returns #,true if found, or insertslot,false if not found.
// panic or -1,false if we skip all around and find no spot
// We store dictionary keys as they were, but compare case-insensitive
func (p *Dictionary) KeyExists(s string) (uint32, bool) {
	s = strings.ToLower(s)

	h := p.findKeyhash(s)

	// Now try to find our match
	if p.dkey[h] == nil { // Empty slot
		return h, false
	} else if strings.ToLower(*p.dkey[h]) == s { // Match
		return h, true // Yay, found the key straight off
	}

	// No immediate hit, so we have to skip around
	for i := 0; i < hashtable_size; i++ {
		h = (h + hash_skip) & hashkey_mask
		if p.dkey[h] == nil { // Empty slot
			return h, false
		} else if strings.ToLower(*p.dkey[h]) == s { // Found our key now
			return h, true
		}
	}

	// Just in case our skipping doesn't get results
	// We may still have to adjust the algorithm to get a nicer distribution?
	// Just walking the table is too slow, so we panic.
	// TODO - handle this without havoc, we already have hashkey_invalid
	panic("dictionary.go: Dictionary key hash fail!?")

	// return hashkey_invalid, false
}

// Note that this always return successfully, since we're just hashing, no look-up.
// And remember we're using a 24-bits hashtable, not 32!
func (p *Dictionary) findKeyhash(s string) uint32 {
	fnvh := fnv.New32a()                 // Initialise new hash
	fnvh.Write([]byte(s))                // Hash our key string
	return (fnvh.Sum32() & hashkey_mask) // Get hash and bound within 24-bits
}

func (p *Dictionary) FindOrAddKeyhash(s string) (uint32, bool) {
	if h, res := p.KeyExists(s); res { // Found existing key
		return h, true
	} else {
		p.dkey[h] = &s    // This key is new, put it into the empty slot
		p.dirty[h] = true // Mark for writing to disk
		p.num_dkeys++     // Increase tally

		return h, true // Success
	}
}

// EOF
