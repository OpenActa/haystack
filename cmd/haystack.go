// OpenActa - Haystack (index) handling - ProtoBuffers definition
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
	"bufio"
	"fmt"
	"os"
	"time"

	"openacta.dev/haystack"
)

var hs haystack.Haystack // New Haystack

func main() {
	fmt.Fprintln(os.Stderr, "Haystack - Haystack log management system test & benchmark tool")
	fmt.Fprintln(os.Stderr, "Copyright (C) 2023 Arjen Lentz & Lentz Pty Ltd; All Rights Reserved")
	fmt.Fprintln(os.Stderr, "Licenced under the Affero General Public Licence (AGPL) v3(+)")
	fmt.Fprintln(os.Stderr, "")

	hs.Haybale = make([]*haystack.Haybale, 0)

	var action bool
	var curarg int

	for curarg = 1; curarg < len(os.Args); curarg++ {
		switch os.Args[curarg] {
		// ----------------------- ingest json file to mem
		case "-i":
			if curarg+1 < len(os.Args) {
				curarg++
				fname := os.Args[curarg]

				fmt.Fprintf(os.Stderr, "Ingesting file '%s'\n", fname)
				// Open the file for reading
				file, err := os.Open(fname)
				if err != nil {
					fmt.Println("Error opening file:", err)
					return
				}
				defer file.Close()

				// Create a new scanner to read the file line by line
				scanner := bufio.NewScanner(file)

				// Start the clock
				start := time.Now()

				cur_hb := new(haystack.Haybale)
				hs.Haybale = append(hs.Haybale, cur_hb)

				// Iterate over each line in the file
				var i int
				for scanner.Scan() {
					line := scanner.Text()
					i++

					if cur_hb.Memsize > haystack.Max_memsize {
						new_hb := new(haystack.Haybale)

						hs.Haybale = append(hs.Haybale, new_hb)
						cur_hb = new_hb
					}
					flat, res := haystack.JSONToKVmap([]byte(line))
					_ = res

					cur_hb.InsertBunch(&hs.Dict, flat)
					if (i % 1000) == 0 {
						fmt.Fprintf(os.Stderr, "%d000 lines\r", i/1000)
					}
				}

				duration := time.Since(start)
				fmt.Fprintf(os.Stderr, "Inserted %d JSON lines, duration: %v\n", i, duration)

				// Check for any errors that may have occurred during scanning
				if err := scanner.Err(); err != nil {
					fmt.Println("Error scanning file:", err)
					return
				}

				action = true
			} else {
				fmt.Fprintf(os.Stderr, "Missing option for -i (requires a filename)\n")
			}

		case "-p":
			for i := range hs.Haybale {
				(*hs.Haybale[i]).PrintBale(&hs.Dict)
			}

			action = true

		case "-kv":
			if curarg+2 < len(os.Args) {
				curarg++
				ks := os.Args[curarg]
				curarg++
				vs := os.Args[curarg]
				hs.SortAllBales()
				hs.SearchKeyVal(ks, vs)

				action = true
			} else {
				fmt.Fprintf(os.Stderr, "Missing options for -kv (requires a key and a value)\n")
			}

			action = true

		case "-w":
			if curarg+1 < len(os.Args) {
				curarg++
				fname := os.Args[curarg]
				fmt.Fprintf(os.Stderr, "Writing Haystack file '%s'\n", fname)

				data, _ := hs.Mem2Disk() // also returns error
				os.WriteFile(fname, data, 0600)

				action = true
			} else {
				fmt.Fprintf(os.Stderr, "Missing option for -w (requires a filename)\n")
			}

			action = true

		case "-r":
			if curarg+1 < len(os.Args) {
				curarg++
				fname := os.Args[curarg]
				fmt.Fprintf(os.Stderr, "Reading Haystack file '%s'\n", fname)

				if data, err := os.ReadFile(fname); err != nil {
					fmt.Fprintf(os.Stderr, "Error reading Haystack file %s: %v\n", fname, err)
				} else {
					if err := hs.Disk2Mem(data); err != nil {
						fmt.Fprintf(os.Stderr, "Reading Haystack file %s: %v\n", fname, err)
					}

					action = true
				}
			} else {
				fmt.Fprintf(os.Stderr, "Missing option for -r (requires a filename)\n")
			}
		}
	}

	if !action {
		fmt.Fprintf(os.Stderr, "Usage: %s ...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, " -i <file>        Ingest JSON from <file> to mem\n")
		fmt.Fprintf(os.Stderr, " -w <file>        Write mem to Haybale <file>\n")
		fmt.Fprintf(os.Stderr, " -r <file>        Read Haybale <file> into mem\n")
		fmt.Fprintf(os.Stderr, " -p               Print mem to stdout\n")
		fmt.Fprintf(os.Stderr, " -kv <key> <val>  Search for <key> <value> pair in mem\n")
	}
}

// EOF
