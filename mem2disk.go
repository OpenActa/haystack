// OpenActa/Haystack - marshall Haybale mem->disk format
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
   Our in-memory format is different from how we store on disk.
   We want to marshall (encode) to an efficient disk format.

   See disk_structure.go, and /doc/haystack.txt
*/

package haystack

import (
	"fmt"
	"hash/crc32"
	"math"
	"strings"
)

// TODO: make all this nicer. All the Go way, but no copying of stuff when it can be avoided.

func addByteToData(buf *[]byte, b byte) {
	*buf = append(*buf, b)
}

func addMultibyteToData(buf *[]byte, v uint64, len int) {
	for i := 0; i < len; i++ {
		b := byte(v & 0xff)
		addByteToData(buf, b)
		v >>= 8
	}
}

// Store both the length (uint32, LSB 4 bytes) and the string (byte sequence, no terminator)
func addStringToData(buf *[]byte, s string) {
	r := strings.NewReader(s)
	len := int32(r.Size())

	addMultibyteToData(buf, uint64(len), 4)

	for i := int32(0); i < len; i++ {
		b, _ := r.ReadByte() // Could come up with EOF error, but really...
		addByteToData(buf, b)
	}
}

// Our hash keys are different enough (3 byte length etc) so do all in this function
func addKeyToData(buf *[]byte, dkey uint32, key *string) error {
	addMultibyteToData(buf, uint64(dkey), 3)

	r := strings.NewReader(*key)
	len := int32(r.Size())

	if len > max_keylen {
		// TODO: this shouldn't happen, we already have a check on insert
		return fmt.Errorf("key '%s' length %d > %d limit", *key, len, max_keylen)
	}

	addByteToData(buf, uint8(len))

	for i := int32(0); i < len; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read byte from key: %w", err)
		}
		addByteToData(buf, b)
	}

	return nil
}

// Assemble disk structure for the Haystack header
func (p *Haystack) Mem2DiskFileHeader() ([]byte, error) {
	content := make([]byte, 0, min_filesize)

	addByteToData(&content, version_major)
	addByteToData(&content, version_minor)

	// Haystack (file) header
	data := make([]byte, 0, min_filesize)

	addMultibyteToData(&data, signature, 3)
	addByteToData(&data, section_header)

	addMultibyteToData(&data, uint64(len(content)), 4) // Len should be 2 for this version
	data = append(data, content...)                    // After we write len, we can glue it all together

	crc := crc32.ChecksumIEEE(data)           // CRC over all of the header data
	addMultibyteToData(&data, uint64(crc), 4) // last of all, append CRC

	return data, nil
}

// Assemble the disk structure for an entire Haystack
func (p *Haystack) Mem2Disk() ([]byte, error) {
	data := make([]byte, 0, 16384) // Set up our byte array, with some initial room to spare

	if header, err := p.Mem2DiskFileHeader(); err != nil {
		return nil, err
	} else {
		data = append(data, header...)
	}

	// Now go through all the haybales
	var time_first, time_last int64
	var prev_ofs, cur_ofs uint32
	for i := range p.Haybale {
		cur_ofs = uint32(len(data)) // note current offset in our buffer

		// First we write out a Dictionary.
		// For the first Haybale, prev_ofs will be 0:
		// that will write out a full Dictionary and append it to our header.
		if dc, err := p.Dict.Mem2Disk(prev_ofs); err != nil {
			return nil, err
		} else {
			data = append(data, dc...)
		}

		// After a Dictionary comes a Haybale structure
		if hb, err := p.Haybale[i].Mem2Disk(&p.Dict); err != nil {
			return nil, err
		} else {
			data = append(data, hb...)
		}

		prev_ofs = cur_ofs

		// Update our bounding timestamps as well (for the trailer)
		if time_first == 0 || p.Haybale[i].time_first < time_first {
			time_first = p.Haybale[i].time_first
		}
		if p.Haybale[i].time_last > time_last {
			time_first = p.Haybale[i].time_last
		}
	}

	if trailer, err := p.Mem2DiskFileTrailer(prev_ofs, time_first, time_last); err != nil {
		return nil, err
	} else {
		data = append(data, trailer...)
	}

	return data, nil
}

// Assemble disk structure for the Haystack trailer
func (p *Haystack) Mem2DiskFileTrailer(last_dict_ofs uint32, time_first int64, time_last int64) ([]byte, error) {
	content := make([]byte, 0, min_filesize)

	addMultibyteToData(&content, uint64(last_dict_ofs), 4)
	addMultibyteToData(&content, uint64(time_first), 8)
	addMultibyteToData(&content, uint64(time_last), 8)

	data := make([]byte, 0, min_filesize)

	// Haystack (file) header
	addMultibyteToData(&data, signature, 3)
	addByteToData(&data, section_trailer)

	addMultibyteToData(&data, uint64(len(content)), 4) // Len should be 16 for this version
	data = append(data, content...)                    // After we write len, we can glue it all together

	crc := crc32.ChecksumIEEE(data)           // CRC over all of the header
	addMultibyteToData(&data, uint64(crc), 4) // last of all, append CRC

	return data, nil
}

// Assemble the disk structure for one Dictionary
func (p *Dictionary) Mem2Disk(prev_ofs uint32) ([]byte, error) {
	var data = make([]byte, 0, 16384)

	addMultibyteToData(&data, uint64(signature), 3)
	addByteToData(&data, section_dictionary)

	var content = make([]byte, 0, p.num_dkeys*32) // Give it a good initial space

	addMultibyteToData(&content, uint64(prev_ofs), 4)    // File pointer to previous Dictionary&Haybale
	addMultibyteToData(&content, uint64(p.num_dkeys), 4) // 4 rather than 3 bytes, for alignment
	// fmt.Fprintf(os.Stderr, "Dict: prev_ofs=%d, num_dkeys=%d\n", prev_ofs, p.num_dkeys) // DEBUG

	for i := uint32(0); i < hashtable_size; i++ {
		if p.dkey[i] == nil {
			// Empty hash slot
			continue
		}

		if !p.dirty[i] && prev_ofs != 0 {
			// If we're not supposed to output the entire dictionary.
			// We do this for Haybales, they only have an incremental dictionary
			continue
		}

		if err := addKeyToData(&content, i, p.dkey[i]); err != nil {
			return nil, err
		}
		p.dirty[i] = false // key handled, doesn't need to be written any more
	}

	addMultibyteToData(&data, uint64(len(content)), 4) // add len into the section start
	data = append(data, content...)                    // After we write len, we can glue it all together

	crc := crc32.ChecksumIEEE(data)           // CRC over all of the Dictionary data
	addMultibyteToData(&data, uint64(crc), 4) // last of all, append CRC

	return data, nil
}

// Assemble the disk structure for one Haybale
func (p *Haybale) Mem2Disk(d *Dictionary) ([]byte, error) {
	var data = make([]byte, 0, 16384)
	var content = make([]byte, 0, 16384)

	p.SortBale() // First of all, make sure this bale is sorted.

	addMultibyteToData(&data, uint64(signature), 3)
	addByteToData(&data, section_haybale)

	// Write out # of haystalks
	addMultibyteToData(&content, uint64(p.num_haystalks), 4)

	addMultibyteToData(&content, uint64(p.time_first), 8)
	addMultibyteToData(&content, uint64(p.time_last), 8)

	// Walk the stalks
	var prev_string *string
	for i := uint32(0); i < p.num_haystalks; i++ {
		addMultibyteToData(&content, uint64(p.haystalk[i].dkey), 3)

		addByteToData(&content, p.haystalk[i].val.valtype)

		addMultibyteToData(&content, uint64(p.haystalk[i].first_ofs), 4)
		addMultibyteToData(&content, uint64(p.haystalk[i].next_ofs), 4)

		// Encode our values appropriately
		switch p.haystalk[i].val.valtype {
		case valtype_int:
			addMultibyteToData(&content, uint64(p.haystalk[i].val.intval), 8)

		case valtype_float:
			addMultibyteToData(&content, math.Float64bits(p.haystalk[i].val.floatval), 8)

		case valtype_string:
			// De-duplicate strings as well. Only adjacent ones - simple but effective.
			if prev_string != nil && *p.haystalk[i].val.stringval == *prev_string {
				// We mark the structure to indicate the value of the previous string,
				// then the disk2mem loader can make sense of it.
				addMultibyteToData(&content, uint64(len_dup), 4) // magic "len" indicator for dup
				// no value
			} else {
				prev_string = p.haystalk[i].val.stringval
				addStringToData(&content, *p.haystalk[i].val.stringval)
			}
		}
	}

	addMultibyteToData(&data, uint64(len(content)), 4) // add len into the section start
	data = append(data, content...)                    // After we write len, we can glue it all together

	crc := crc32.ChecksumIEEE(data) // CRC over this Haybale
	addMultibyteToData(&data, uint64(crc), 4)

	return data, nil
}

// EOF
