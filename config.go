// OpenActa/Haystack - Configuration
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
	"encoding/base64"
	"encoding/csv"
	"log"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/viper"
)

/*
	Configurable options for the Haystack component go here.
	Everything else is set, or automatic/dynamic.

	From [haystack] section in /etc/openacta/openacta.conf
*/

type Haystack_Config struct {
	user                      string
	uid                       uint32
	group                     string
	gid                       uint32
	datastore_dir             string
	catalogue_dir             string
	aes_keystore_list         string
	aes_keystore_array        map[string][]byte // read from keystore_list
	aes_keystore_current_uuid string            // last uuid from keystore_list
	haystack_wait_maxsize     uint32
	haybale_wait_minsize      uint32
	haybale_wait_mintime      uint32
	haybale_wait_maxtime      uint32
	compression_level         uint32
}

var config Haystack_Config

/*
func init() {
	//config_set_defaults()
}

func config_set_defaults() {
	config.user = "openacta"
	config.group = "openacta"

	config.datastore_dir = "/tmp/openacta/data"
	config.catalogue_dir = "/tmp/openacta/catalogue"
	config.aes_keystore_list = "./testdata/keystore.list"

	config.haystack_wait_maxsize = 128 * 1024 * 1024 // 128M
	config.haybale_wait_minsize = 16 * 1024 * 1024   // 16M
	config.haybale_wait_mintime = 300                // 5 minutes
	config.haybale_wait_maxtime = 300                // 5 minutes

	config.compression_level = 9 // highest (slower)
}
*/

func ConfigureVariables() int {
	var errors int

	errors += config_parse_string(&config.user, "haystack.user")
	errors += config_parse_string(&config.group, "haystack.group")

	errors += config_parse_dirname(&config.datastore_dir, "haystack.datastore_dir")
	errors += config_parse_dirname(&config.catalogue_dir, "haystack.catalogue_dir")
	errors += config_parse_filename(&config.aes_keystore_list, "haystack.aes_keystore_list")

	errors += config_parse_size(&config.haystack_wait_maxsize, "haystack.haystack_wait_maxsize", haystack_wait_maxsize_lower, haystack_wait_maxsize_upper)
	errors += config_parse_size(&config.haybale_wait_minsize, "haystack.haybale_wait_minsize", haybale_wait_minsize_lower, haybale_wait_minsize_upper)
	errors += config_parse_time(&config.haybale_wait_mintime, "haystack.haybale_wait_mintime", haybale_wait_mintime_lower, haybale_wait_mintime_upper)
	errors += config_parse_time(&config.haybale_wait_maxtime, "haystack.haybale_wait_maxtime", haybale_wait_maxtime_lower, haybale_wait_maxtime_upper)

	errors += config_parse_int(&config.compression_level, "haystack.compression_level", compression_level_lower, compression_level_upper)

	return errors
}

func ValidateConfiguration() int {
	var errors int

	errors += checkSystemUserGroup()

	errors += checkFileUserGroupAttributes(config.datastore_dir)
	errors += checkFileUserGroupAttributes(config.catalogue_dir)
	errors += checkFileUserGroupAttributes(config.aes_keystore_list)

	errors += ConfigureAESKeyStore()

	return errors
}

func checkSystemUserGroup() int {
	var errors int

	// Check user and group configuration relative to what's on the system

	// Look up configured user or uid
	config_user, err := user.Lookup(config.user)
	if err != nil {
		// Can't find username - we check it this way, because a username could be all digits :)
		config_user, err = user.LookupId(config.user)
		if err != nil {
			// Not found as numeric either
			log.Printf("Configured user (%s) does not exist on system", config.user)
			errors++
		}
	}

	// Look up configured group or gid
	config_group, err := user.LookupGroup(config.group)
	if err != nil {
		// Can't find groupname - we check it this way, because a groupname could be all digits :)
		config_group, err = user.LookupGroupId(config.group)
		if err != nil {
			// Not found as numeric either
			log.Printf("Configured group (%s) does not exist on system", config.group)
			errors++
		}
	}

	if errors > 0 {
		return errors // return early
	}

	config.user = config_user.Username
	i, _ := strconv.Atoi(config_user.Uid)
	config.uid = uint32(i)

	config.group = config_group.Name
	i, _ = strconv.Atoi(config_group.Gid)
	config.gid = uint32(i)

	// Now check current user is same as configured user
	current_user, _ := user.Current()
	if current_user.Username != config.user {
		log.Printf("Current user (%s) not same as configured user (%s)",
			current_user.Uid, config.user)
		errors++
	}

	// Check that current group is same as configured group as well
	i, _ = strconv.Atoi(current_user.Gid)
	gid := uint32(i)
	if gid != config.gid {
		log.Printf("Current primary group ID (%d) not same as configured group ID (%d)",
			gid, config.gid)
		errors++
	}

	return errors
}

func checkFileUserGroupAttributes(path string) int {
	var errors int

	st, _ := os.Stat(path)

	if config.uid != st.Sys().(*syscall.Stat_t).Uid {
		log.Printf("'%s' is not owned by current user (%s)", path, config.user)
		errors++
	}

	if config.gid != st.Sys().(*syscall.Stat_t).Gid {
		log.Printf("'%s' is not owned by primary group (%s)", path, config.group)
		errors++
	}

	var perm_allowed uint32

	if st.IsDir() {
		perm_allowed = 0770
	} else {
		perm_allowed = 0660
	}

	file_perm := uint32(st.Mode().Perm())
	if (file_perm & 0007) != 0 { // If any "others" are allowed, we object.
		log.Printf("Permissions for '%s' are %04o (allowed: %04o)", path, file_perm, perm_allowed)
		errors++
	}

	return errors
}

func config_parse_string(s *string, key string) int {
	if str := viper.GetString(key); str != "" {
		*s = str
	} else {
		log.Printf("Configuration entry for '%s' missing or empty", key)
		return 1
	}

	return 0 // 0 = success
}

func config_parse_dirname(v *string, key string) int {
	if dirpath := viper.GetString(key); dirpath != "" {
		if *v != "" {
			log.Printf("Cannot change path for '%s' from '%s' to '%s' while running", key, *v, dirpath)
			return 1
		}

		*v = dirpath
	} else {
		log.Printf("Configuration entry for '%s' missing or empty", key)
		return 1
	}

	st, err := os.Stat(*v)
	if err != nil {
		log.Printf("%s path: %s", key, err)
		return 1
	} else if !st.IsDir() {
		log.Printf("%s path '%s' is not a directory", key, *v)
		return 1
	}

	return 0 // 0 = success
}

func config_parse_filename(v *string, key string) int {
	if fname := viper.GetString(key); fname != "" {
		*v = fname
	} else {
		log.Printf("Configuration entry for '%s' missing or empty", key)
		return 1
	}

	st, err := os.Stat(*v)
	if err != nil {
		log.Printf("%s file: %s", key, err)
		return 1
	} else if st.IsDir() {
		log.Printf("%s path '%s' is not a file", key, *v)
		return 1
	}

	return 0 // 0 = success
}

func config_parse_int(i *uint32, key string, lower uint32, upper uint32) int {
	*i = viper.GetUint32(key)

	if *i < lower || *i > upper {
		log.Printf("Variable %s out of bounds (%d), must be between %d and %d",
			key, *i, lower, upper)
		return 1
	}

	return 0 // 0 = success
}

func config_parse_size(i *uint32, key string, lower uint32, upper uint32) int {
	s := viper.GetString(key)
	if s == "" {
		log.Printf("Configuration entry for '%s' missing or empty", key)
		return 1
	}
	multiplier := 1

	s = strings.ToUpper(s)
	if strings.HasSuffix(s, "M") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "G") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "G")
	}

	size, err := strconv.Atoi(s)
	if err != nil {
		log.Printf("Cannot parse variable %s: '%s'", key, s)
		return 1
	}

	*i = uint32(size) * uint32(multiplier)

	if *i < lower || *i > upper {
		log.Printf("Variable %s out of bounds (%d), must be between %d and %d",
			key, *i, lower, upper)
		return 1
	}

	return 0 // 0 = success
}

func config_parse_time(i *uint32, key string, lower uint32, upper uint32) int {
	s := viper.GetString(key)
	if s == "" {
		log.Printf("Configuration entry for '%s' missing or empty", key)
		return 1
	}
	multiplier := 1

	s = strings.ToLower(s)
	if strings.HasSuffix(s, "s") {
		multiplier = 1
		s = strings.TrimSuffix(s, "s")
	} else if strings.HasSuffix(s, "m") {
		multiplier = 60
		s = strings.TrimSuffix(s, "m")
	}

	size, err := strconv.Atoi(s)
	if err != nil {
		log.Printf("Cannot parse variable %s: '%s'", key, s)
		return 1
	}

	*i = uint32(size) * uint32(multiplier)

	if *i < lower || *i > upper {
		log.Printf("Variable %s out of bounds (%d), must be between %d and %d",
			key, *i, lower, upper)
		return 1
	}

	return 0 // 0 = success
}

func ConfigureAESKeyStore() int {
	file, err := os.Open(config.aes_keystore_list)
	if err != nil {
		log.Printf("Error opening AES keystore file: %s", err)
		return 1
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)
	reader.Comment = '#' // Specify # as comment character
	reader.FieldsPerRecord = 3

	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("Error reading AES keystore file: %s", err)
		return 1
	}

	new_array := make(map[string][]byte)
	for _, fields := range records {
		// Convert printable base64 AES key string back to binary sequence we can use
		key, err := base64.StdEncoding.DecodeString(fields[1])
		if err != nil {
			log.Printf("Error decoding base64 AES key (uuid %s): %s", fields[0], err)
			return 1
		}

		// uuid is key, AES key (decoded from base64) is value
		new_array[fields[0]] = key

		// most recent one is active key
		config.aes_keystore_current_uuid = fields[0]
	}
	// We do it this way because another Go routine may be accessing
	config.aes_keystore_array = new_array

	return 0 // 0 = success
}

// EOF
