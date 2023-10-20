// namecheap.go -- update config for namecheap
//
// Author: Sudhi Herle (sw@herle.net)
// License: GPLv2

package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html/charset"

	L "github.com/opencoff/go-logger"
)

const (
	_NamecheapHost    string = "dynamicdns.park-your-domain.com"
	_NamecheapURLPath string = "/update"
)

type namecheapUpdater struct {
	fqdn string
	v    url.Values
	log  *L.Logger
}

var _ Updater = &namecheapUpdater{}

func NewNamecheapUpdater(fqdn, key string, log *L.Logger) (Updater, error) {
	i := strings.Index(fqdn, ".")
	if i < 0 {
		return nil, fmt.Errorf("namecheap: %s is not a fqdn", fqdn)
	}

	v := url.Values(make(map[string][]string))
	v.Set("host", fqdn[:i])
	v.Set("domain", fqdn[i+1:])
	v.Set("password", key)

	u := &namecheapUpdater{
		fqdn: fqdn,
		v:    v,
		log:  log,
	}

	log.Info("Using namecheap as DDNS for %s", fqdn)

	return u, nil
}

func (n *namecheapUpdater) safeurl(ip net.IP) string {
	a := url.Values(make(map[string][]string))
	for k, v := range n.v {
		if k == "password" {
			v = []string{"xxxxxxxxxxxxxx"}
		}
		a[k] = v
	}
	a.Set("ip", ip.String())

	u := url.URL{
		Scheme:   "https",
		Host:     _NamecheapHost,
		Path:     _NamecheapURLPath,
		RawQuery: a.Encode(),
	}
	return u.String()
}

type xmlResponse struct {
	IP    string   `xml:"IP"`
	Errs  int      `xml:"ErrCount"`
	Err1  string   `xml:"errors>Err1"`
	Err2  string   `xml:"errors>Err2"`
	Resps int      `xml:"ResponseCount"`
	Resp  []string `xml:"responses>response>ResponseString"`
	Done  bool     `xml:"Done"`
}

// A successful update looks like so:
//
//	<?xml version="1.0"?>
//	<interface-response>
//	    <Command>SETDNSHOST</Command>
//	    <Language>eng</Language>
//	    <IP>98.33.25.87</IP>
//	    <ErrCount>0</ErrCount>
//	    <ResponseCount>0</ResponseCount>
//	    <Done>true</Done>
//	    <debug><![CDATA[]]></debug>
//	</interface-response>
//
// Error response looks like so:
//	<?xml version="1.0"?>
//	<interface-response>
//	    <Command>SETDNSHOST</Command>
//	    <Language>eng</Language>
//	    <ErrCount>1</ErrCount>
//	    <errors>
//		<Err1>Passwords do not match</Err1>
//	    </errors>
//	    <ResponseCount>1</ResponseCount>
//	    <responses>
//		<response>
//		    <ResponseNumber>304156</ResponseNumber>
//		    <ResponseString>Validation error; invalid ; password</ResponseString>
//		</response>
//	    </responses>
//	</interface-response>

func (n *namecheapUpdater) Update(ip net.IP) error {
	log := n.log

	eret := func(s string, err error) error {
		n.log.Warn("namecheap: %s %s", s, err)
		return fmt.Errorf("namecheap: %s %w", s, err)
	}

	if len(ip) == 0 {
		log.Error("namecheap: No IP for %s?", n.fqdn)
		return fmt.Errorf("namecheap: no IP for %s", n.fqdn)
	}

	n.v.Set("ip", ip.String())
	u := url.URL{
		Scheme:   "https",
		Host:     _NamecheapHost,
		Path:     _NamecheapURLPath,
		RawQuery: n.v.Encode(),
	}

	log.Debug("namecheap: beginning DDNS update %s=%s", n.fqdn, ip.String())
	if Dryrun {
		log.Info("namecheap: Dryrun %q", n.safeurl(ip))
		return nil
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return eret("", err)
	}

	cl := &http.Client{
		Transport: getDefaultTransport(),
	}

	resp, err := cl.Do(req)
	if err != nil {
		return eret("", err)
	}

	defer resp.Body.Close()

	// Namecheap xml response is utf-16 encoded; we need extra steps to
	// decode it.
	var x xmlResponse

	rd, err := charset.NewReader(resp.Body, "utf-16")
	if err != nil {
		return eret("", err)
	}

	dec := xml.NewDecoder(rd)
	dec.CharsetReader = func(_ string, rd io.Reader) (io.Reader, error) {
		return rd, nil
	}

	if err = dec.Decode(&x); err != nil {
		return eret("xml", err)
	}

	if x.Errs > 0 {
		log.Warn("namecheap: xml decode: %s", x.Err1)
		return fmt.Errorf("namecheap: Errs %s", x.Err1)
	}

	log.Info("namecheap: %s %s DDNS complete", ip.String(), n.fqdn)
	return nil
}
