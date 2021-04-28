// keyfile.go - parse a simplistic keyfile

// keyfile is a text file with 3 fields:
//
//   FQDN PROVIDER password
//
// Where PROVIDER is one of the supported providers. For now, this is just namecheap
//

package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
	"syscall"
)

// Read keyfile and return password for the given fqdn
func ReadKeyFile(fqdn, provider, keyfile string) (string, bool) {
	fd, err := os.Open(keyfile)
	if err != nil {
		die("can't open %s: %s", keyfile, err)
	}

	defer fd.Close()

	fi, err := fd.Stat()
	if err != nil {
		die("can't stat %s: %s", keyfile, err)
	}

	st := fi.Sys().(*syscall.Stat_t)
	me := uint32(os.Getuid())

	// If we are running as non-root, do additional checks
	if me != 0 {
		// no-one but the user running the file should have access to it.
		if (st.Mode & 0066) != 0 {
			die("keyfile %s: insecure permissions (group/world read-write)", keyfile)
		}
		if st.Uid != me {
			die("keyfile %s: user %d is not the owner (%d)", keyfile, me, st.Uid)
		}

		if err := checkStat(keyfile); err != nil {
			die("%s", err)
		}
	}

	r := bufio.NewScanner(fd)
	n := 0
	for r.Scan() {
		n += 1
		s := strings.TrimSpace(r.Text())
		if len(s) == 0 || s[0] == '#' {
			continue
		}

		v := strings.Fields(s)
		if len(v) != 3 {
			warn("%s:%d: not enough fields (need 3)", keyfile, n)
			continue
		}

		if fqdn == v[0] && provider == v[1] {
			return v[2], true
		}
	}

	return "", false
}

// check this stat result, validate it and its parent.
// We walk all the way up to the root
func checkStat(nm string) error {
	// walk every parent of the given name 'nm' and make sure perms are good all the way
	// through
	for dir := path.Dir(nm); nm != dir; nm = dir {
		fi, err := os.Stat(dir)
		if err != nil {
			return err
		}
		m := fi.Mode()
		if (m & 0022) != 0 {
			return fmt.Errorf("insecure perms on %s (group/world write)", dir)
		}
	}
	return nil
}
