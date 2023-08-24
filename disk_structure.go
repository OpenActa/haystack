// OpenActa/Haystack - structures and constants (disk storage)
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

/*
	The structures in this file are commented out, because they cannot be used
	directly. Many aspects of the file format are variable length or otherwise
	dynamic in nature. The Mem2Disk and Disk2Mem code handles this directly.
	Note that this means we need to be very careful to not break something.

	Ref doc/haystack.txt
*/

const (
	min_filesize = 58                   // len(Header) + len(Trailer)
	max_filesize = (1024 * 1024 * 1024) // 1GB (outer limit)
	len_dup      = 0xfffffffe           // Len to indicate de-dupped string

	bzip2_hdrMagic = 0x425a         // Hex of "BZ"
	bzip2_blkMagic = 0x314159265359 // BCD of PI

	sha512_byte_len = 64 // SHA-512

	AES_key_byte_len        = (256 / 8)                    // AES256
	aesgcm_nonce_byte_len   = 12                           // AES nonce is 92 bits
	aesgcm_block_additional = (aesgcm_nonce_byte_len + 16) // + AES GCM overhead

	NewFilePermissions = 0660 // Permissions for new files
	NewDirPermissions  = 0770 // Permissions for new directories
)

/*
type DiskSection struct {
	sig 	[3]byte		// Section signature
	id		uint8		// File section identifier
	unc_len	uint32		// Uncompressed content length
	com_len	uint32		// Compressed content length
	crc 	uint32		// IEEE CRC-32
	<content>			// Section content (compressed and encrypted)
}
*/

const (
	signature = 0xebfeda // Our 3 byte file/segment signature

	min_DiskHeaderBaselen = 16 // # bytes in preamble of any section
)

const ( // Haystack file section identifiers
	section_header     = 1
	section_dictionary = 2
	section_haybale    = 3
	section_sha512     = 254
	section_trailer    = 255
)

/*
type DiskFileHeader struct {
	major     uint8     	// Major version
	minor     uint8     	// Minor version
	aes_uuid  [16]byte		// AES key uuid
}
*/

const (
	version_major = 1
	version_minor = 0
)

/*
type DiskDictHeader struct {
	prev_ofs  uint32 		// offset of previous Dictionary+Haybale (or 0 for none)
	num_dkeys uint32		// number of keys (max 16777216)
	<DiskDictEntry> ...		// Dictionary entries
}
*/

const (
	min_DiskDictHeaderLen = 8
	max_dkeys             = hashtable_size // 16M (24-bit hash table)
)

/*
type DiskDictEntry struct {
	dkey         [3]byte 	// Dictionary key #
	dkey_namelen uint8   	// Byte length of key name (max 255)
	dkey_name    []byte 	// Dictionary key name
}
*/

/*
type DiskHaybaleHeader struct {
	num_stalks uint32	// number of DiskHaybaleEntry (stalks) in this record
	time_first uint64	// _timestamp of first entry in this record
	time_last  uint64	// _timestamp of last entry in this record
}
*/

const (
	min_DiskHaybaleHeaderLen = 20
)

/*
type DiskHaytalkEntry struct {
	dkey    [3]byte		// Key = Dictionary lookup #
	valtype uint8
	first   uint32		// offset to first (_timestamp) in bunch (self for _timestamp)
	next    uint32		// offset to next in bunch (0 for last)
	// for strings only
	len		uint32		// length of string, or len_dup for a de-dupped string
	// Note: the following is left out for de-dupped strings.
	val		[]byte		// byte sequence of string value (not terminated)
}
*/

const (
	valtype_int    = 1
	valtype_float  = 2
	valtype_string = 3
)

/*
type DiskFileSHA512 struct {
	time_first uint64 	// _timestamp of first entry in this Haystack
	time_last  uint64 	// _timestamp of last entry in this Haystack
	sha512     [64]byte // SHA-512 over all of Haystack file
}
*/

/*
type DiskFileTrailer struct {
	last_dict_ofs		// Offset to last Dictionary (and accompanying Haystack)
	time_first uint64	// _timestamp of first entry in this Haystack
	time_last  uint64	// _timestamp of last entry in this Haystack
}
*/

// EOF
