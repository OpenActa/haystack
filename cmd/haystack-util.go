// OpenActa/Haystack - Haystack management utility
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

package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
	"openacta.dev/haystack"
)

func main() {
	fmt.Fprintln(os.Stderr, "Haystack - log management system - utility")
	fmt.Fprintln(os.Stderr, "Copyright (C) 2023 Arjen Lentz & Lentz Pty Ltd; All Rights Reserved")
	fmt.Fprintln(os.Stderr, "Licenced under the Affero General Public Licence (AGPL) v3(+)")
	fmt.Fprintln(os.Stderr)

	uuid := uuid.New()
	fmt.Printf("UUID: %s\n", uuid.String())

	// Generate a random 256-bit AES key
	key := make([]byte, haystack.AES_key_byte_len)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		panic(err)
	}

	// Convert binary key to printable string
	key_str := base64.StdEncoding.EncodeToString(key)
	fmt.Printf("Key:  %s\n", key_str)
}

// EOF
