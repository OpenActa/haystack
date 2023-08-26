// OpenActa/Haystack - SHA512 Catalogue file handling
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
	"crypto/sha512"
	"fmt"
	"hash/crc32"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Generate and disk write SHA512 for cryptographic signature, over the entire
// compressed+encrypted dataset of a Haystack.
// We only need to provide the full Haystack path/filename
func CreateCatelogueFile(haystack_fname string) error {
	data, err := os.ReadFile(haystack_fname)
	if err != nil {
		log.Printf("Error reading Haystack file '%s': %v", haystack_fname, err)
		return err
	}

	// TODO: find time_first, time_last in cache, or Haystack file, instead of in filename
	fname := filepath.Base(haystack_fname)
	fname_noext := fname[:len(fname)-len(filepath.Ext(fname))]
	before, after, found := strings.Cut(fname_noext, "-")

	// TODO: should check found
	_ = found

	time_first, err := strconv.ParseInt(before, 10, 64) // int64
	if err != nil {
		log.Printf("Error decoding Haystack filename '%s' time_first component: %v", haystack_fname, err)
		return err
	}

	time_last, err := strconv.ParseInt(after, 10, 64) // int64
	if err != nil {
		log.Printf("Error decoding Haystack filename '%s' time_last component: %v", haystack_fname, err)
		return err
	}

	sha512data, err := mem2DiskSHA512block(data, time_first, time_last)
	if err != nil {
		log.Printf("Error calculating SHA-512 catalogue entry for Haystack '%s': %v", haystack_fname, err)
		return err
	}

	sha512hs_fname := fmt.Sprintf("%s/%v-%v.hsc", config.catalogue_dir, time_first, time_last)
	if err = os.WriteFile(sha512hs_fname, sha512data, NewFilePermissions); err != nil {
		log.Printf("Error writing Haystack file '%s': %v", sha512hs_fname, err)
		return err
	}

	return nil
}

// Calculate SHA512 over entire Haystack, return Catalogue data (file header + SHA512 section)
func mem2DiskSHA512block(dataset []byte, time_first int64, time_last int64) ([]byte, error) {
	var data = make([]byte, 0, 16384)
	var content = make([]byte, 0, 16384)

	// Give SHA512 file a proper file header so we have major/minor versioning
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
	encrypted_content, err := mem2DiskAES256GCMblock(&content, data, config.aes_keystore_current_uuid)
	if err != nil {
		return nil, err
	}

	data = append(data, *encrypted_content...) // we can glue it all together

	return append(hdr, data...), nil
}

// EOF
