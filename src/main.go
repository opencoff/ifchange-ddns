// nc-ddns.go - Simple DDNS notifier that watches for interface addr change
//
// NB: Namecheap DDNS API only supports v4
//
//
// Author: Sudhi Herle (sw@herle.net)
// License: GPLv2

package main

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	L "github.com/opencoff/go-logger"
	flag "github.com/opencoff/pflag"
)

var Dryrun bool

// All updaters take the same set of inputs
type Updater interface {
	Update(ip net.IP) error
}

// Usage: $0 interface FQDN keyfile
func main() {
	var sleep time.Duration
	var once, debug bool

	os.Args[0] = filepath.Base(os.Args[0])
	flag.DurationVarP(&sleep, "sleep", "s", 60*time.Second, "Sleep for `N` units (secs, mins) between checks")
	flag.BoolVarP(&once, "oneshot", "", false, "Do the update once and don't sleep/poll")
	flag.BoolVarP(&Dryrun, "dry-run", "n", false, "Dryrun mode (don't post http-update)")
	flag.BoolVarP(&debug, "debug", "d", false, "Run in debug mode")

	usage := fmt.Sprintf("%s [options] INTERFACE FQDN KEYFILE", os.Args[0])

	flag.Usage = func() {
		fmt.Printf("%s - Monitor a given interface and update DNS\nUsage: %s\n", os.Args[0], usage)
		flag.PrintDefaults()
	}

	flag.Parse()
	args := flag.Args()
	if len(args) < 3 {
		die("Insufficient arguments. Try '%s --help'", os.Args[0])
	}

	iface := args[0]
	fqdn := args[1]
	keyfile, err := filepath.Abs(args[2])
	if err != nil {
		die("can't get abs path for %s: %s", args[2], err)
	}

	fmt.Printf("iface=%s, fqdn=%s, keyfile=%s\n", iface, fqdn, keyfile)
	ip, err := getIP(iface)
	if err != nil {
		die("%s", err)
	}

	key, ok := ReadKeyFile(fqdn, "namecheap", keyfile)
	if !ok {
		die("can't find %s for namecheap in %s", fqdn, keyfile)
	}

	logdest := "SYSLOG"
	prio, ok := L.ToPriority("INFO")
	if !ok {
		die("Invalid log-level INFO?!")
	}

	if debug {
		prio = L.LOG_DEBUG
		logdest = "STDOUT"
	}

	// We want microsecond timestamps and debug logs to have short
	// filenames
	const logflags int = L.Ldate | L.Ltime | L.Lshortfile | L.Lmicroseconds

	log, err := L.NewLogger(logdest, prio, "", logflags)
	if err != nil {
		die("can't create logger: %s", err)
	}

	// always update  the first time
	nc, err := NewNamecheapUpdater(fqdn, key, log)
	if err != nil {
		die("%s", err)
	}

	err = nc.Update(ip)
	if err != nil {
		die("%s", err)
	}

	if !once {
		startPoll(log, iface, sleep, nc, ip)
	}

	log.Close()
	os.Exit(0)
}

func startPoll(log *L.Logger, iface string, sleep time.Duration, u Updater, old net.IP) {

	// Setup signal handlers
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan,
		syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	signal.Ignore(syscall.SIGPIPE, syscall.SIGFPE)

	tm := time.NewTicker(sleep)
	timechan := tm.C

	defer tm.Stop()
	for {
		select {
		case _ = <-timechan:
			ip, err := getIP(iface)
			if err != nil {
				log.Warn("%s", err)
			} else if !bytes.Equal(old, ip) {
				u.Update(ip)
				old = ip
			}

		case num := <-sigchan:
			sig := num.(syscall.Signal)
			log.Info("Caught signal %d; Terminating ..\n", int(sig))
			return
		}
	}
}

// Get all IPs of a given interface
func getIP(nm string) (net.IP, error) {
	ii, err := net.InterfaceByName(nm)
	if err != nil {
		return nil, err
	}

	addr, err := ii.Addrs()
	if err != nil {
		return nil, err
	}

	// filter out v6 addr and return first usable v4 addr
	for i := range addr {
		a := addr[i]
		ip, _, err := net.ParseCIDR(a.String())
		if err != nil {
			return nil, fmt.Errorf("%s: %s", nm, err)
		}

		if v4 := ip.To4(); v4 != nil && acceptable(v4) {
			return v4, nil
		}
	}

	return nil, fmt.Errorf("%s: can't find usable IP", nm)
}

// Return true if this IP is acceptable as a DDNS update
// i.e., filter out martians
func acceptable(ip net.IP) bool {
	for i := range Martians {
		nn := &Martians[i]
		if nn.Contains(ip) {
			return false
		}
	}
	return true
}

func getDefaultTransport() http.RoundTripper {
	t := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 60 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   6 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return t
}

var Martians = []net.IPNet{
	net.IPNet{ // loopback addr
		IP:   []byte{127, 0, 0, 0},
		Mask: net.CIDRMask(8, 32),
	},
	net.IPNet{ // 192.168.0.0/16
		IP:   []byte{192, 168, 0, 0},
		Mask: net.CIDRMask(16, 32),
	},
	net.IPNet{ // 172.16.0.0/12
		IP:   []byte{172, 16, 0, 0},
		Mask: net.CIDRMask(12, 32),
	},
	net.IPNet{ // 10.0.0.0/8
		IP:   []byte{10, 0, 0, 0},
		Mask: net.CIDRMask(8, 32),
	},
	net.IPNet{ // CGNAT 100.64.0.0/10
		IP:   []byte{100, 64, 0, 0},
		Mask: net.CIDRMask(10, 32),
	},
}
