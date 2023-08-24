// OpenActa/Haystack - Logging
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
	"log"
	"log/syslog"
	"os"
	"path/filepath"
)

func init() {
	// Check whether stderr is open and either a console or a file
	if _, err := os.Stderr.Stat(); err != nil {
		// No console or file, so we need to set up for syslog
		// Use the base filename (without path) as our app name
		syslogWriter, err := syslog.New(syslog.LOG_INFO, filepath.Base(os.Args[0]))
		if err != nil {
			log.Fatal(err) // This can't print anywhere, but we exit with error.
		}

		log.SetFlags(0)             // We don't want timestamp etc inside msgs
		log.SetOutput(syslogWriter) // Now make all writes to log go to syslog
	}
}

// EOF
