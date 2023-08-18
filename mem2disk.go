// OpenActa/Haystack - marshall Haystack mem->disk format
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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"strings"
	"github.com/dsnet/compress/bzip2"
	"github.com/google/uuid"
)

var aesgcm_nonce = make([]byte, aesgcm_nonce_byte_len)

func init() {
	// Create a unique starting nonce (feeding off the system random # generator)
	// We do it here so it's only done once during app's lifetime.
	// TODO: ideally we'd save the nonce (IV=Initialisation Vector) on HD or in Redis
	if _, err := io.ReadFull(rand.Reader, aesgcm_nonce); err != nil {
		panic(err)
	}
}

// We must not re-use an IV (initialisation vector, nonce) so we increment it.
func aes_inc_nonce() {
	// We need to do the inc "by hand" as it's 96 bits, larger than any of our variable types
	for i := 0; i < aesgcm_nonce_byte_len; i++ {
		aesgcm_nonce[i]++         // increment
		if aesgcm_nonce[i] != 0 { // overflow=carry
			break // no carry = done
		}
	}
}

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

// Assemble the disk structure for an entire Haystack
// Return compressed/encrypted dataset, sha512 block, error
func (p *Haystack) Mem2Disk() ([]byte, []byte, error) {
	data := make([]byte, 0, 16384) // Set up our byte array, with some initial room to spare

	header, err := mem2DiskFileHeader()
	if err != nil {
		return nil, nil, err
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
			return nil, nil, err
		} else {
			data = append(data, dc...)
		}

		// After a Dictionary comes a Haybale structure
		if hb, err := p.Haybale[i].Mem2Disk(&p.Dict); err != nil {
			return nil, nil, err
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

	if trailer, err := mem2DiskFileTrailer(prev_ofs, time_first, time_last); err != nil {
		return nil, nil, err
	} else {
		data = append(data, trailer...)
	}

	// Generate SHA512 for cryptographic signature, over the entire
	// compressed+encrypted dataset
	sha512section, err := mem2DiskSHA512block(data, time_first, time_last)
	if err != nil {
		return nil, nil, err
	}

	return data, sha512section, nil
}

func mem2DiskSHA512block(dataset []byte, time_first int64, time_last int64) ([]byte, error) {
	var data = make([]byte, 0, 16384)
	var content = make([]byte, 0, 16384)

	// Give SHA512 file has a proper header so we have major/minor versioning
	hdr, err := mem2DiskFileHeader()
	if err != nil {
		return nil, err
	}

	// Now for the SHA512 itself
	sha512 := sha512.Sum512(dataset)

	// section header
	addMultibyteToData(&data, uint64(signature), 3)
	addByteToData(&data, section_sha512)

	// section content
	addMultibyteToData(&content, uint64(time_first), 8)
	addMultibyteToData(&content, uint64(time_last), 8)

	for i := 0; i < sha512_byte_len; i++ {
		addByteToData(&content, sha512[i]) // 32 bytes (512 bits) SHA512
	}

	// now we know the content length. Don't bother with compression.
	addMultibyteToData(&data, uint64(len(content)), 4)
	addMultibyteToData(&data, uint64(len(content)), 4)

	crc := crc32.ChecksumIEEE(content)        // CRC over the content
	addMultibyteToData(&data, uint64(crc), 4) // append CRC

	// Encryption
	encrypted_content, err := mem2DiskAES256GCMblock(&content, data)
	if err != nil {
		return nil, err
	}

	data = append(data, hdr...)
	data = append(data, *encrypted_content...) // we can glue it all together

	return data, nil
}

// Assemble disk structure for the Haystack header
func mem2DiskFileHeader() ([]byte, error) {
	content := make([]byte, 0, min_filesize)
	data := make([]byte, 0, min_filesize)

	addByteToData(&content, version_major)
	addByteToData(&content, version_minor)

	uuid, _ := uuid.Parse(aes_test_uuid)    // grab AES uuid
	uuid_binary, _ := uuid.MarshalBinary()  // get it out in binary
	for i := 0; i < len(uuid_binary); i++ { // 16 bytes
		addByteToData(&content, uuid_binary[i]) // put it in our structure
	}

	// Haystack (file) header
	addMultibyteToData(&data, signature, 3)
	addByteToData(&data, section_header)

	addMultibyteToData(&data, uint64(len(content)), 4) // Len should be 18 for this version
	addMultibyteToData(&data, uint64(len(content)), 4) // No compression

	crc := crc32.ChecksumIEEE(content)        // CRC over all of header content
	addMultibyteToData(&data, uint64(crc), 4) // append CRC

	// No encryption of file header, otherwise we can't convey uuid (chicken&egg)

	data = append(data, content...) // we can glue it all together

	return data, nil
}

// Assemble disk structure for the Haystack trailer
func mem2DiskFileTrailer(last_dict_ofs uint32, time_first int64, time_last int64) ([]byte, error) {
	content := make([]byte, 0, min_filesize)
	data := make([]byte, 0, min_filesize)

	addMultibyteToData(&content, uint64(last_dict_ofs), 4)
	addMultibyteToData(&content, uint64(time_first), 8)
	addMultibyteToData(&content, uint64(time_last), 8)

	// Haystack (file) header
	addMultibyteToData(&data, signature, 3)
	addByteToData(&data, section_trailer)

	addMultibyteToData(&data, uint64(len(content)), 4) // Len should be 20 for this version
	addMultibyteToData(&data, uint64(len(content)), 4) // No compression

	crc := crc32.ChecksumIEEE(content)        // CRC over all of the trailer content
	addMultibyteToData(&data, uint64(crc), 4) // append CRC

	// Encryption
	encrypted_content, err := mem2DiskAES256GCMblock(&content, data)
	if err != nil {
		return nil, err
	}

	data = append(data, *encrypted_content...) // we can glue it all together

	return data, nil
}

// Assemble disk structure for bzip2 -9 compression
// https://github.com/dsnet/compress
// (Go's standard library implementation only does decompression)
// Ref. https://github.com/dsnet/compress/blob/master/doc/bzip2-format.pdf
func mem2DiskBzip2block(content []byte) ([]byte, error) {
	//fmt.Fprintf(os.Stderr, "bzip2 -9\n")	// DEBUG

	var bzip2_config bzip2.WriterConfig
	var buf bytes.Buffer

	bzip2_config.Level = bzip2.BestCompression // Choose best compression (-9 equiv)

	writer, err := bzip2.NewWriter(&buf, &bzip2_config)
	if err != nil {
		return nil, fmt.Errorf("error bzip2 compressing: %v", err)
	}

	// Compress, bzip2 -9 style.
	if _, err := writer.Write(content); err != nil {
		return nil, fmt.Errorf("error bzip2 compressing: %v", err)
	}
	writer.Close()

	// Check if our output is indeed shorter (it will almost always be)
	if writer.OutputOffset > 0 && writer.OutputOffset < writer.InputOffset {
		compressed_data := buf.Bytes()
		return compressed_data, nil
	}

	// return original data, since compressed wasn't any shorter
	return content, nil
}

// Assemble disk structure for an AES encrypted block
// We use 256 bit AES block cipher in GCM mode, with AEAD
// Ref. https://csrc.nist.gov/pubs/sp/800/38/d/final
func mem2DiskAES256GCMblock(plaintext *[]byte, extra []byte) (*[]byte, error) {
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

	// AES GCM mode adds some (16) bytes, so the encrypted dataset is longer!
	encrypted_data := make([]byte, 0, len(*plaintext)+aesgcm.Overhead())

	// Put in our section header in as additional authenticated data (AEAD).
	// This allows us to authenticate (and validate) the stored sections in full.
	encrypted_content := append(encrypted_data, aesgcm.Seal(nil, aesgcm_nonce, *plaintext, extra)...)

	// Put it all together
	data := make([]byte, 0, aesgcm.NonceSize()+len(*plaintext)+aesgcm.Overhead())
	data = append(data, aesgcm_nonce...)
	data = append(data, encrypted_content...)

	aes_inc_nonce() // increment nonce so it doesn't get re-used

	return &data, nil
}

// Assemble the disk structure for one Dictionary
func (p *Dictionary) Mem2Disk(prev_ofs uint32) ([]byte, error) {
	var data = make([]byte, 0, 16384)
	var content = make([]byte, 0, 16384)

	// section header
	addMultibyteToData(&data, uint64(signature), 3)
	addByteToData(&data, section_dictionary)

	addMultibyteToData(&content, uint64(prev_ofs), 4)    // File pointer to previous Dictionary&Haybale
	addMultibyteToData(&content, uint64(p.num_dkeys), 4) // Number of (new) dkeys, max. 16M
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

	unc_len := len(content)

	crc := crc32.ChecksumIEEE(content) // CRC over all of the Dictionary content

	// Compression
	content, err := mem2DiskBzip2block(content)
	if err != nil {
		return nil, err
	}
	com_len := len(content)

	//fmt.Fprintf(os.Stderr, "Dict mem2disk() unc_len=%d, com_len=%d\n", unc_len, com_len) // DEBUG

	addMultibyteToData(&data, uint64(unc_len), 4) // add uncompressed len into the section start
	addMultibyteToData(&data, uint64(com_len), 4) // add compressed len into the section start

	addMultibyteToData(&data, uint64(crc), 4) // append CRC

	// Encryption
	encrypted_content, err := mem2DiskAES256GCMblock(&content, data)
	if err != nil {
		return nil, err
	}

	data = append(data, *encrypted_content...) // we can glue it all together

	return data, nil
}

// Assemble the disk structure for one Haybale
func (p *Haybale) Mem2Disk(d *Dictionary) ([]byte, error) {
	var data = make([]byte, 0, 16384)
	var content = make([]byte, 0, 16384)

	p.SortBale() // First of all, make sure this bale is sorted.

	// section header
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

	addMultibyteToData(&data, uint64(len(content)), 4) // add uncompressed len into the section start

	crc := crc32.ChecksumIEEE(content) // CRC over all of the Haybale content

	// Compression
	content, err := mem2DiskBzip2block(content)
	if err != nil {
		return nil, err
	}
	addMultibyteToData(&data, uint64(len(content)), 4) // add compressed len into the section start

	addMultibyteToData(&data, uint64(crc), 4) // append CRC

	// Encryption
	encrypted_content, err := mem2DiskAES256GCMblock(&content, data)
	if err != nil {
		return nil, err
	}

	data = append(data, *encrypted_content...) // we can glue it all together

	return data, nil
}

// EOF
