// OpenActa/Haystack - structures and constants (mem storage)
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

// Ref doc/haystack.txt

const (
	max_keylen       = 255               // Max text len of a key
	Max_memsize      = 512 * 1024 * 1024 // 512MB (half a gig) in RAM
	hashtable_size   = 16 * 1024 * 1024  // Exact size of key hashtable (16M)
	Timestamp_key    = "_timestamp"      // Timestamp key string
	haystalk_ofs_nil = 0xffffffff        // used for nil, last
	cap_initial      = 100000            // Size of initial haystalk slice allocation
)

type Haystack struct {
	Dict Dictionary

	Haybale []*Haybale // Array of pointers to Haybale record (time slices)

	// needed to keep track of our in-mem and on-disk size
	memsize uint32
}

type Dictionary struct {
	num_dkeys uint32                  // How many keys do we use (used in mem2disk)
	dkey      [hashtable_size]*string // 24-bit hash table (16MB)
	dirty     [hashtable_size]bool    // Save to disk with next Haybale (record)
}

type Haybale struct {
	num_haystalks uint32 // number of entries (Haystalks) in this haybale
	/*
		True if this haybale is sorted
		This flag also makes the haybale read-only (immutable)
		TODO: make Go routine safe
	*/
	is_sorted_immutable bool

	haystalk []*Haystalk // slice of pointers to KV entries

	time_first int64
	time_last  int64

	// needed to keep track of our in-mem and on-disk size
	Memsize uint32
}

type Haystalk struct {
	dkey uint32 // Key = Dictionary lookup #
	val  Val    // Value

	self_ofs  uint32 // Pointer to self. Used during sort.
	first_ofs uint32 // offset to first (_timestamp) in Haystalk (self for first)
	next_ofs  uint32 // offset to next in Haystalk (0xffffffff for last)
}

type ValType interface {
	int64 | float64 | *string
}

type Val struct {
	valtype uint8 // Value type (int, float, string)

	intval    int64
	floatval  float64
	stringval *string
}

// EOF
