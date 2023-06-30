// OpenActa/Haystack Dictionary - tests
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

import "testing"

func TestFindOrAddKeyhash(t *testing.T) {
	var haystack Haystack

	// Colliding words from /usr/share/dict/words (Linux)
	// TODO we still need some multi-colliders for full code coverage.

	var dkeys []string = []string{"foo", "bar", "snarf", "Foo", "oink", // Foo is dup
		"envEloPES", "VerandahS", "dIMPLES", "WAITS", "CONFERATE", "vizualising", // 1x Colliding
	}
	var dhash []uint32 = []uint32{15957719, 12025114, 14976195, 15957719, 14592958,
		3612882, 5259835, 14872617, 14872718, 1576052, 1054892}

	for i := 0; i < len(dkeys); i++ {
		h, res := haystack.Dict.FindOrAddKeyhash(dkeys[i])
		if res != true || h != dhash[i] {
			t.Errorf("Dictionary add %v = %v, wanted %v (res=%v)", dkeys[i], h, dhash[i], res)
		}
	}
}

// EOF
