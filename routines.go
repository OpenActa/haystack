// OpenActa/Haystack - Go routines for runtime management
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
	We use Go routines to manage the various activities of the Haystack subsystem.
	- primary
	- diskreader
*/

package haystack

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

type HaystackRoutinesType struct {
	diskreader_ch chan int

	diskwriter_ch chan int
	diskwriter_wg sync.WaitGroup

	diskwriter_iname string // initial fname we use for writing out Haystacks

	writer_cur_haystack *Haystack
	writer_cur_haybale  *Haybale
	writer_cur_fp       *os.File
	writer_prev_ofs     uint32
}

var HaystackRoutines HaystackRoutinesType

const (
	diskwriter_nop = iota
	diskwriter_flush_haybale
	diskwriter_flush_haystack
	diskwriter_close
)

// Call after getting config: ConfigureVariables() + ValidateConfiguration()
func StartUp() error {
	log.Printf("Haystack startup")

	newHaystack()

	// Assemble initial fname we use for writing out Haystacks
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Error retrieving own hostname: %v", err)
		return err
	}
	hostname = strings.ToLower(hostname) // we want hostname to be in all lowercase
	HaystackRoutines.diskwriter_iname = fmt.Sprintf("%s/%s.hs", config.datastore_dir, hostname)

	// Create our inter-routine comms channels
	HaystackRoutines.diskreader_ch = make(chan int)
	HaystackRoutines.diskwriter_ch = make(chan int)

	// Start up our subsystem Go routines
	go diskReader()
	go diskWriter()

	return nil
}

// Call before program exit
func ShutDown() {
	HaystackRoutines.FlushHaystack()

	// We need to actually wait (block) for this to finish
	HaystackRoutines.diskwriter_wg.Add(1)
	HaystackRoutines.diskwriter_ch <- diskwriter_close // Close everything
	HaystackRoutines.diskwriter_wg.Wait()
	// diskWriter Go routine will now have exited.
}

func (p *HaystackRoutinesType) FlushHaybale() {
	p.diskwriter_ch <- diskwriter_flush_haybale
}

func (p *HaystackRoutinesType) FlushHaystack() {
	p.diskwriter_ch <- diskwriter_flush_haystack
}

// diskReader go routine
func diskReader() {

}

// diskWriter go routine
func diskWriter() {
	for {
		select {
		// Check for commands from the diskwriter channel
		case cmd := <-HaystackRoutines.diskwriter_ch:
			switch cmd {
			case diskwriter_flush_haybale: // Flush Haybale
				// Do we actually have anything to flush?
				if HaystackRoutines.writer_cur_haystack.Haybale[len(HaystackRoutines.writer_cur_haystack.Haybale)-1].Memsize == 0 {
					break // Apparently not, so don't do anything here
				}

				log.Printf("Writing Haybale")

				writeHaystackHeader() // only writes something if needed

			case diskwriter_flush_haystack: // Flush Haystack
				//HaystackRoutines.FlushHaybale()

				// Do we actually have anything to flush?
				if HaystackRoutines.writer_cur_haystack.Haybale[len(HaystackRoutines.writer_cur_haystack.Haybale)-1].Memsize == 0 {
					break // Apparently not, so don't do anything here
				}

				log.Printf("Writing Haystack file")

				writeHaystackHeader() // only writes something if needed

				var time_first, time_last int64
				for i := range HaystackRoutines.writer_cur_haystack.Haybale {
					if HaystackRoutines.writer_cur_haystack.Haybale[i].Memsize > 0 && // Haybale has some content
						!HaystackRoutines.writer_cur_haystack.Haybale[i].is_sorted_immutable { // Haybale is not yet immutable
						if HaystackRoutines.writer_cur_haystack.Haybale[i] == HaystackRoutines.writer_cur_haybale {
							newHaybale() // Create a new Haybale for the main thread to write to
						}
						HaystackRoutines.writer_cur_haystack.Haybale[i].SortBale() // Make it immutable, too

						// Write out Dictionary+Haybale
						mem2DiskDictionaryAndHaybale(HaystackRoutines.writer_cur_haystack, i)
					}

					// Update our bounding timestamps as well (for the trailer, and SHA512 catalogue file)
					if time_first == 0 || HaystackRoutines.writer_cur_haystack.Haybale[i].time_first < time_first {
						time_first = HaystackRoutines.writer_cur_haystack.Haybale[i].time_first
					}
					if HaystackRoutines.writer_cur_haystack.Haybale[i].time_last > time_last {
						time_last = HaystackRoutines.writer_cur_haystack.Haybale[i].time_last
					}
				}

				/*

					// Write Haybales (or whole file, for now)
					data, _ = HaystackRoutines.writer_cur_haystack.Mem2Disk() // also returns error
					_, err = HaystackRoutines.writer_cur_fp.Write(data)
					if err != nil {
						log.Printf("Error writing %d bytes to file '%s': %v", len(data), HaystackRoutines.diskwriter_iname, err)
						break
					}

				*/

				writeHaystackTrailer(time_first, time_last)

			case diskwriter_close: // Close everything
				// only requested by ShutDown(), uses a wait group
				log.Printf("Haystack close/shutdown")
				// ...

				HaystackRoutines.diskwriter_wg.Done()
				return // exit Go routine

			}

			// Check for timeout of haybale_wait_mintime
			//case <-time.After(time.Duration(config.haybale_wait_mintime) * time.Second):
			// timed out - check if we need to do stuff

			// Check for timeout of haybale_wait_maxtime
			//case <-time.After(time.Duration(config.haybale_wait_maxtime) * time.Second):
			// timed out - check if we need to do stuff
		}
	}
}

func newHaystack() {
	// Create a new Haystack to which we can write
	var new_hs Haystack

	HaystackRoutines.writer_cur_haystack = &new_hs

	// Set this Haystack's AES uuid to current configured one.
	HaystackRoutines.writer_cur_haystack.aes_key_uuid = config.aes_keystore_current_uuid

	// Set up an empty Haybale array
	HaystackRoutines.writer_cur_haystack.Haybale = make([]*Haybale, 0)

	newHaybale()
}

func newHaybale() {
	// Create a new Haybale to which we can write
	var new_hb Haybale

	HaystackRoutines.writer_cur_haybale = &new_hb

	// Put the back-pointer from new writer Haybale to the new Haystack
	HaystackRoutines.writer_cur_haybale.HaystackPtr = HaystackRoutines.writer_cur_haystack

	// Add the new writer Haybale to the array of Haybales in the Haystack
	HaystackRoutines.writer_cur_haystack.Haybale = append(HaystackRoutines.writer_cur_haystack.Haybale, HaystackRoutines.writer_cur_haybale)
}

func writeHaystackHeader() error {
	var err error

	if HaystackRoutines.writer_cur_fp == nil {
		HaystackRoutines.writer_cur_fp, err = os.OpenFile(HaystackRoutines.diskwriter_iname, os.O_WRONLY|os.O_CREATE, NewFilePermissions)
		if err != nil {
			log.Printf("Error creating file '%s' for write: %v", HaystackRoutines.diskwriter_iname, err)
			return err
		}

		// Write Haystack file header
		err := mem2DiskFileHeader(HaystackRoutines.writer_cur_fp)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeHaystackTrailer(time_first int64, time_last int64) error {
	// Write Haystack file trailer
	err := HaystackRoutines.writer_cur_haystack.mem2DiskFileTrailer(HaystackRoutines.writer_cur_haystack.last_dict_ofs, time_first, time_last)
	if err != nil {
		log.Printf("Error writing Haystack '%s' file trailer: %v", HaystackRoutines.diskwriter_iname, err)
		return err
	}

	HaystackRoutines.writer_cur_fp.Close() // Close output file
	HaystackRoutines.writer_cur_fp = nil   // Set file handle to nil, so we remember
	HaystackRoutines.writer_prev_ofs = 0

	fname := fmt.Sprintf("%s/%d-%d.hs", config.datastore_dir, time_first, time_last)
	if err := os.Rename(HaystackRoutines.diskwriter_iname, fname); err != nil {
		log.Printf("Error renaming file '%s' to '%s': %v", HaystackRoutines.diskwriter_iname, fname, err)
		return err
	}

	// Also create SHA512 file
	CreateCatelogueFile(fname)

	newHaystack()

	return nil
}

// EOF
