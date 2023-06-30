// OpenActa/Haystack - ingesting JSON (in a flat way)
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
	Why are we here?
	We parse JSON, but we want everything to a single layer for our KV store.
	The solution is to "flatten" arrays etc. Oh and we allow dups.

	From:
	"a": "b",
    "c": {
        "d": "e",
        "f": "g",
    },
    "z": [2, 1.4567],

	To:
	"a": "b",
	"c.d": "e",
	"c.f": "g",
	"z.0": 2,
	"z.1": 1.4567,
*/

package haystack

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/nqd/flat" // Third party library
)

func JSONToKVmap(b []byte) (map[string]interface{}, error) {
	var result map[string]interface{}

	// Unmarshal checks for validity too.
	// Realistically there's not much we can do with invalid lines. Ignore.
	err := json.Unmarshal(b, &result)
	if err != nil {
		return nil, err
	}

	// Note: using third party library
	// Uses reflection.
	flatmap, err := flat.Flatten(result, &flat.Options{
		Delimiter: ".",   // Use the . delimiter when flattening
		MaxDepth:  1000,  //	Maximum depth of arrays/structures
		Safe:      false, //	Flatten arrays as well as structures
	})

	if err != nil {
		return nil, err
	}

	// Make the timestamp field special
	if _, ok := flatmap["timestamp"]; ok {
		// timestamp to _timestamp
		flatmap[Timestamp_key] = flatmap["timestamp"]
		delete(flatmap, "timestamp")
	} else if _, ok := flatmap[Timestamp_key]; !ok {
		/*
			If there's no _timestamp field, we add one. We need one!
			Nanosecs, not because of accuracy (we dunno when the log entry was
			created), but because the log entries must remain in order.
			The below works out to "2022-01-01T00:00:00.123456789Z"
		*/
		flatmap[Timestamp_key] = time.Now().UTC().Format(time.RFC3339Nano)
	}

	/*
			Unfortunately, the parsing scrambles a few things.
		    Suricata eve.json has flow_id:
		    "1184018670052842" which ends up as "1.184018670052842e+15"
		    and similar others. We can easily clean that up.
		    TODO: create configurable regex map (multiple regexes/replace)
	*/
	if e_regex, err := regexp.CompilePOSIX(`([0-9])\.([0-9]+)e\+[0-9]+`); err == nil {
		for k, v := range flatmap {
			s := fmt.Sprint(v)
			if e_regex.MatchString(s) {
				flatmap[k] = e_regex.ReplaceAllString(s, "$1$2")
			}
		}
	}

	return flatmap, nil
}

// EOF
