// OpenActa/Haystack - unmarshall Haystack disk->mem format
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
   Read a Haystack back into memory.
   Unsurprisingly reading is slightly more finicky than writing the data.
   We need to check CRCs, and other format aspects, to ensure we're not
   reading in or processing garbage. We can't just blow up.
   TODO: The code itself needs to be checked/verified (_test coverage)

   See doc/haystack.txt and mem2disk.go for reference
*/

package haystack

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"

	"github.com/dsnet/compress/bzip2"
	"github.com/google/uuid"
)

// Read a byte
func getByteFromData(reader *bytes.Reader) byte {
	b, _ := reader.ReadByte() // This shouldn't error since we're checking stuff elsewhere

	return b
}

// Read a uint of len n bytes (LSB first)
func getUintFromData(reader *bytes.Reader, n int) uint64 {
	var u uint64

	for shift := 0; n > 0; n-- {
		u |= uint64(getByteFromData(reader)) << shift
		shift += 8
	}

	return u
}

// Read a float of len n bytes (generally 64bit)
func getFloatFromData(reader *bytes.Reader, n int) float64 {
	return (math.Float64frombits(getUintFromData(reader, n)))
}

// Read a string of length n
func getStringFromData(reader *bytes.Reader, n int) *string {
	// Since strings are immutable, we can't just append bytes to a string.
	// Therefore we start with a byte slice, append all we need, and convert.
	bs := make([]byte, 0, n) // pre-allocate slice of appropriate length

	for ; n > 0; n-- {
		bs = append(bs, getByteFromData(reader))
	}

	s := string(bs)
	return &s
}

// Our hash keys are different enough (3 byte length etc) so do all in this function
func getKeyFromData(reader *bytes.Reader) (uint32, *string) {
	dkey := uint32(getUintFromData(reader, 3))
	len := int(getUintFromData(reader, 1))
	s := getStringFromData(reader, len)

	return dkey, s
}

// Check a section (CRC and other sanity), return (error), section type, length and content
func (p *Haystack) getDisk2MemSections(data []byte) error {
	var read_com_len, read_unc_len int
	var prev_section int
	var err error

	file_reader := bytes.NewReader(data)

	// Loop through each section in the Haystack Haystack
trailer:
	for {
		// read in next section header
		header := make([]byte, min_DiskHeaderBaselen)
		if n, err := file_reader.Read(header); err != nil || n < min_DiskHeaderBaselen {
			return fmt.Errorf("unexpected end of file while reading Haystack")
		}
		hdr_reader := bytes.NewReader(header)

		// Get signature
		read_signature := getUintFromData(hdr_reader, 3)
		if read_signature != signature {
			return fmt.Errorf("incorrect signature (0x%06x instead of 0x%06x), not a Haystack or dataset corrupt?",
				read_signature, signature)
		}

		read_section := getByteFromData(hdr_reader) // Get section identifier

		fmt.Fprintf(os.Stderr, "getDisk2MemSections loop (section id: %d)\n", read_section) // DEBUG

		if prev_section == 0 && read_section != section_header {
			return fmt.Errorf("first section not header, not a Haystack or dataset corrupt?")
		}

		// Get lengths (uncompressed and compressed)
		read_unc_len = int(getUintFromData(hdr_reader, 4)) // uncompressed len of content
		read_com_len = int(getUintFromData(hdr_reader, 4)) // compressed len of content
		if read_unc_len < 1 || read_unc_len > max_filesize ||
			read_com_len < 1 || read_com_len > max_filesize ||
			read_com_len > read_unc_len {
			return fmt.Errorf("stored lengths %d (com), %d (unc) invalid, corrupted Haystack?", read_com_len, read_unc_len)
		}

		// CRC is over content (read_unc_len)
		read_crc := uint32(getUintFromData(hdr_reader, 4)) // Read stored CRC

		var len int
		if read_section == 1 {
			len = read_com_len
		} else {
			len = read_com_len + aesgcm_block_additional
		}
		content := make([]byte, len)

		if n, err := file_reader.Read(content); err != nil || n < len {
			return fmt.Errorf("unexpected end of file: %s", err)
		}

		if read_section != 1 {
			// Decryption
			content, err = getDisk2MemAES256GCMblock(content, header)
			if err != nil {
				return err
			}
			// Note that AES GCM also removes its 12 + 16 bytes of overhead
		}

		// Decompressing, if compressed
		if read_com_len < read_unc_len {
			content, err = getDisk2MemBzip2block(content)
			if err != nil {
				return err
			}
		}

		// Calculate our own CRC, to compare against the stored one
		header_crc := crc32.ChecksumIEEE(content)
		if read_crc != header_crc {
			return fmt.Errorf("section CRC mismatch (read 0x%08x, calculated 0x%08x), Haystack corrupted?",
				read_crc, header_crc)
		}

		switch read_section {
		case section_header:
			if err := p.getDisk2MemHeader(content); err != nil {
				return err
			}

		case section_dictionary:
			if prev_section != section_header && prev_section != section_haybale {
				return fmt.Errorf("Dictionary section can only follow a Header or Haybale")
			}
			if err := p.getDisk2MemDictionary(content); err != nil {
				return err
			}

		case section_haybale:
			if prev_section != section_dictionary {
				return fmt.Errorf("Haybale section can only follow a Dictionary")
			}
			if err := p.getDisk2MemHaybale(content); err != nil {
				return err
			}

		case section_trailer:
			break trailer // Trailer section, break out of our loop. So ignore any garbage after that.

		default:
			return fmt.Errorf("unknown section type %d, not a Haystack or dataset corrupt?", read_section)
		}

		prev_section = int(read_section)
	}

	return nil
}

// Process Header content
func (p *Haystack) getDisk2MemHeader(content []byte) error {
	fmt.Fprintf(os.Stderr, "getDisk2MemHeader\n") // DEBUG

	reader := bytes.NewReader(content)

	read_version_major := getByteFromData(reader)
	read_version_minor := getByteFromData(reader)

	// If/Once there are multiple versions or formats, we can implement appropriate handling
	// rather than just refusing. We want to be at least backwards compatible.
	if read_version_major != version_major || read_version_minor != version_minor {
		return fmt.Errorf("stored version of Haystack (%d.%d) incompatible with this server (%d.%d)",
			read_version_major, read_version_minor, version_major, version_minor)
	}

	// Read back UUID (in binary form) of AES key
  uuid_bytes := make([]byte, 16) // 16 bytes
	for i := 0; i < len(uuid_bytes); i++ {
		uuid_bytes[i] = getByteFromData(reader)
	}
	uuid_raw, err := uuid.FromBytes(uuid_bytes)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stderr, "File AES used key uuid %s\n", uuid_raw.String()) // DEBUG
	if uuid_raw.String() != aes_test_uuid {
		return fmt.Errorf("file was encrypted with different (unknown) AES key")
	}

	return nil
}

// Process Dictionary content
func (p *Haystack) getDisk2MemDictionary(content []byte) error {
	fmt.Fprintf(os.Stderr, "getDisk2MemDictionary\n") // DEBUG

	reader := bytes.NewReader(content)

	if reader.Len() < min_DiskDictHeaderLen {
		return fmt.Errorf("dictionary section too short, missing fields")
	}

	read_prev_ofs := getUintFromData(reader, 4)
	read_num_dkeys := int(getUintFromData(reader, 4)) // reading 4 rather than 3 bytes, just for alignment
	// No further fields in the dictionary header at this point

	//fmt.Fprintf(os.Stderr, "read_num_dkeys=%d\n", read_num_dkeys) // DEBUG

	// This one can't be checked, either, because we're not passing the prev_ofs around between calls.
	_ = read_prev_ofs // not used here (just for recovery purposes)

	if read_num_dkeys > max_dkeys {
		return fmt.Errorf("read num dkeys %d > %d possible", read_num_dkeys, max_dkeys)
	}

	for i := 0; i < read_num_dkeys; i++ {
		dkey, key := getKeyFromData(reader)

		//fmt.Fprintf(os.Stderr, "dkey[%d]=%-10s\r", dkey, *key) // DEBUG

		// Put key in our own hash table. Same location as original.
		// Exact same 24-bit (min_DiskDictHeaderLen) range. Also, we use ptr to string
		p.Dict.dkey[dkey] = key
	}

	return nil
}

// Process Haybale content
func (p *Haystack) getDisk2MemHaybale(content []byte) error {
	fmt.Fprintf(os.Stderr, "getDisk2MemHaybale\n") // DEBUG

	if len(content) == 0 { // do we need to bother?
		return nil
	}

	var new_hb Haybale // Create a new haybale

	reader := bytes.NewReader(content)

	if reader.Len() < min_DiskHaybaleHeaderLen {
		return fmt.Errorf("haybale section too short, missing fields")
	}

	read_num_haystalks := int(getUintFromData(reader, 4))

	new_hb.time_first = int64(getUintFromData(reader, 8))
	new_hb.time_last = int64(getUintFromData(reader, 8))

	var prev_string *string
	var read_len uint32
	for i := 0; i < read_num_haystalks; i++ {
		var newstalk Haystalk

		if i > 0 {
			new_hb.haystalk = append(new_hb.haystalk, &Haystalk{})
		} else { // allocate to the exact # we will have
			new_hb.haystalk = make([]*Haystalk, 1, read_num_haystalks)
		}

		newstalk.dkey = uint32(getUintFromData(reader, 3))
		if p.Dict.dkey[newstalk.dkey] == nil { // DEBUG
			panic(fmt.Sprintf("Read back nil referenced dkey %d from disk\n", newstalk.dkey))
		}

		read_valtype := uint8(getUintFromData(reader, 1))

		newstalk.first_ofs = uint32(getUintFromData(reader, 4))
		newstalk.next_ofs = uint32(getUintFromData(reader, 4))

		switch read_valtype {
		case valtype_int:
			newstalk.val.SetInt(int64(getUintFromData(reader, 8)))

		case valtype_float:
			newstalk.val.SetFloat(getFloatFromData(reader, 8))

		case valtype_string:
			read_len = uint32(getUintFromData(reader, 4))
			if read_len == len_dup {
				if prev_string == nil { // best to check these things
					return fmt.Errorf("de-dupped string indicated but not present")
				}

				newstalk.val.SetString(prev_string) // use the dup
			} else {
				s := getStringFromData(reader, int(read_len))
				newstalk.val.SetString(s)
				prev_string = s
			}
		}

		new_hb.Memsize += 37 // Haystalk struct, approx
		if newstalk.val.valtype == valtype_string && read_len != len_dup {
			new_hb.Memsize += uint32(2 + len(*newstalk.val.stringval))
		}

		new_hb.haystalk[i] = &newstalk // Append stalk into the haybale
		newstalk.self_ofs = uint32(i)  // ofs of self. Not really needed here since we're immutable

		new_hb.num_haystalks++
	}

	p.memsize += new_hb.Memsize       // Calculate in this new haybale
	new_hb.is_sorted_immutable = true // Set to immutable (obviously) and it's sorted.
	// TODO: with multiple go routines we probably need to have a semaphore around the following
	p.Haybale = append(p.Haybale, &new_hb) // Append to data available for search

	return nil
}

// bzip2's signatures are HSB (highest significant byte) first
func bzip2_check_sig(dataslice []byte, sigseq uint64) bool {
	var res uint64

	for i := 0; i < len(dataslice); i++ {
		res <<= 8
		res |= uint64(dataslice[i])
	}

	return res == sigseq
}

// Process bzip2 -9 content
func getDisk2MemBzip2block(data []byte) ([]byte, error) {
	fmt.Fprintf(os.Stderr, "getDisk2MemBzip2block\n") // DEBUG

	// check for bzip2 file and block signatures
	if !bzip2_check_sig(data[0:2], bzip2_hdrMagic) ||
		!bzip2_check_sig(data[4:10], bzip2_blkMagic) {
		// If no signatures, presume not compressed...
		// In the worst case, it'll fail CRC check. Good.
		return data, nil
	}

	// It's a bzip2 compressed block: decompress our data!
	var bzip2_config bzip2.ReaderConfig

	if reader, err := bzip2.NewReader(bytes.NewReader(data), &bzip2_config); err != nil {
		return nil, fmt.Errorf("error decompressing bzip2: %v", err)
	} else if buf, err := io.ReadAll(reader); err != nil {
		return nil, fmt.Errorf("error decompressing bzip2: %v", err)
	} else if reader.OutputOffset > max_filesize {
		return nil, fmt.Errorf("dataset too long, not a Haystack?")
	} else {
		reader.Close()

		// assign decompressed data so we can process it
		data = buf
	}

	return data, nil
}

// Process AES256-GCM content
func getDisk2MemAES256GCMblock(data []byte, extra []byte) ([]byte, error) {
	fmt.Fprintf(os.Stderr, "Process AES256+GCM (extra=%v)\n", extra) // DEBUG

	// Convert printable AES key string back to binary sequence we can use
	key, err := base64.StdEncoding.DecodeString(aes_test_key)
	if err != nil {
		return nil, fmt.Errorf("error decoding base64 encoded AES key: %s", err)
	}

	// Create a new AES cipher block using the raw key
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("error initialising AES cipher: %s", err)
	}

	// Create a new GCM cipher mode using the AES cipher block
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("error initialising GCM cipher mode: %s", err)
	}

	// Read the nonce back
	nonce := data[0:aesgcm.NonceSize()]
	data = data[aesgcm.NonceSize():]

	// cleartext is slightly shorter than ciphertext, so this is ok.
	// It's just about efficiency anyway, nothing bad is going to happen.
	var plaintext = make([]byte, 0, len(data))

	plaintext, err = aesgcm.Open(nil, nonce, data, extra)
	if err != nil {
		return nil, fmt.Errorf("error decrypting Haystack: %s", err)
	}

	return plaintext, nil
}

// Process byte slice into complete Haystack structure
// We check the wazoo out of this!
func (p *Haystack) Disk2Mem(data []byte) error {
	fmt.Fprintf(os.Stderr, "Disk2Mem\n") // DEBUG

	len := len(data)

	// First check some general file stuff
	if len < min_filesize {
		return fmt.Errorf("dataset too short, not a Haystack?")
	}

	if len > max_filesize {
		return fmt.Errorf("dataset too long, not a Haystack?")
	}

	// Now dive into the file's content
	if err := p.getDisk2MemSections(data); err != nil {
		return err
	}

	return nil // All good.
}

// EOF
