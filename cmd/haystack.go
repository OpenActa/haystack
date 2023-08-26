// OpenActa/Haystack test and benchmark CLI
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
	"log"
	"os"
	"time"

	"github.com/spf13/viper"
	"openacta.dev/haystack"
)

var hs haystack.Haystack // New Haystack

func main() {
	log.Println("Haystack - Haystack log management system test & benchmark tool")
	log.Println("Copyright (C) 2023 Arjen Lentz & Lentz Pty Ltd; All Rights Reserved")
	log.Println("Licenced under the Affero General Public Licence (AGPL) v3(+)")
	fmt.Println()

	hs.Haybale = make([]*haystack.Haybale, 0)

	var action bool
	var curarg int

	viper.SetConfigFile("./testdata/haystack.conf")
	viper.SetConfigType("ini")
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Error reading configuration")
		os.Exit(1)
	}

	errors := haystack.ConfigureVariables()
	if errors > 0 {
		log.Printf("%d errors reading Haystack configuration", errors)
		os.Exit(1)
	}

	errors = haystack.ValidateConfiguration()
	if errors > 0 {
		log.Printf("%d errors validating Haystack configuration", errors)
		os.Exit(1)
	}

	errors = haystack.ConfigureAESKeyStore()
	if errors > 0 {
		log.Printf("%d errors initialising Haystack subsystem", errors)
		os.Exit(1)
	}

	for curarg = 1; curarg < len(os.Args); curarg++ {
		switch os.Args[curarg] {
		// ----------------------- ingest json file to mem
		case "-i":
			if curarg+1 < len(os.Args) {
				curarg++
				fname := os.Args[curarg]

				log.Printf("Ingesting file '%s'", fname)
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
				cur_hb.HaystackPtr = &hs
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
						cur_hb.HaystackPtr = &hs
					}
					flat, res := haystack.JSONToKVmap([]byte(line))
					_ = res

					cur_hb.InsertBunch(&hs.Dict, flat)
					if (i % 1000) == 0 {
						fmt.Fprintf(os.Stderr, "%d000 lines\r", i/1000)
					}
				}

				duration := time.Since(start)
				log.Printf("Inserted %d JSON lines, duration: %v", i, duration)

				// Check for any errors that may have occurred during scanning
				if err := scanner.Err(); err != nil {
					fmt.Println("Error scanning file:", err)
					return
				}

				action = true
			} else {
				log.Printf("Missing option for -i (requires a filename)")
			}

		case "-p":
			for i := range hs.Haybale {
				(*hs.Haybale[i]).PrintBale(&hs.Dict)
			}

			action = true

		case "-kv":
			hs.SortAllBales()

			kv_array := make(map[string]string)
			if curarg+2 < len(os.Args) {
				for curarg+2 < len(os.Args) {
					kv_array[os.Args[curarg+1]] = os.Args[curarg+2]
					curarg += 2
				}
			} else {
				log.Printf("Missing options for -kv (requires a key and a value)")
				break
			}

			/*
				for k, v := range kv_array {
					hs.SearchKeyVal(k, v)
				}
			*/
			hs.SearchKeyValArray(kv_array)

			action = true
			curarg = len(os.Args) // Hack so we're always the last param(s)

		case "-w":
			if curarg+1 < len(os.Args) {
				curarg++
				fname := os.Args[curarg]
				log.Printf("Writing Haystack file '%s'", fname)

				// Start the clock
				start := time.Now()
				data, _ := hs.Mem2Disk() // also returns error
				duration := time.Since(start)
				log.Printf("Mem2Disk() duration: %v", duration)
				os.WriteFile(fname, data, haystack.NewFilePermissions)

				haystack.CreateCatelogueFile(fname)

				action = true
			} else {
				log.Printf("Missing option for -w (requires a filename)")
			}

			action = true

		case "-r":
			if curarg+1 < len(os.Args) {
				curarg++
				fname := os.Args[curarg]
				log.Printf("Reading Haystack file '%s'", fname)

				if data, err := os.ReadFile(fname); err != nil {
					log.Printf("Error reading Haystack file %s: %v", fname, err)
				} else {
					// Start the clock
					start := time.Now()
					if err := hs.Disk2Mem(data); err != nil {
						log.Printf("Reading Haystack file %s: %v", fname, err)
					}
					data = nil // de-reference as we don't need it anymore
					duration := time.Since(start)
					log.Printf("Disk2Mem() duration: %v", duration)
				}
				action = true
			} else {
				log.Printf("Missing option for -r (requires a filename)")
			}
		}
	}

	if !action {
		log.Printf("Usage: %s ...", os.Args[0])
		log.Printf(" -i <file>            Ingest JSON from <file> to mem")
		log.Printf(" -w <file>            Write mem to Haystack <file>")
		log.Printf(" -r <file>            Read Haystack <file> into mem")
		log.Printf(" -p                   Print mem to stdout")
		log.Printf(" -kv <key> <val> ...  Search for <key> <value> pair(s) in mem")
	}
}

// EOF
