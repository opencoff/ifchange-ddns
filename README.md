[![GoDoc](https://godoc.org/github.com/opencoff/ifchange-ddns?status.svg)](https://godoc.org/github.com/opencoff/ifchange-ddns)
[![Go Report Card](https://goreportcard.com/badge/github.com/opencoff/ifchange-ddns)](https://goreportcard.com/report/github.com/opencoff/ifchange-ddns)
# ifchange-ddns - Monitor network interface and update DDNS

## What is it?
This is a small program to monitor a given network interface and 
update DDNS when the IPv4 address changes. 

I wrote this because OpenBSD dhclient removed the `dhclient-script`
functionality and I needed a way to update DDNS when my ISP assigns 
a new IPv4 address.

The current implementation only has support for namecheap DNS. Other
providers can be easily added.

## How do I use it?
Assuming you also use namecheap:

1. Build the client for your platform:

    ```sh
        $ make
        $ # or make for openbsd
        $ make openbsd
    ```

2. If you want to run this as a daemon on OpenBSD:
   Assuming the interface you want to monitor is `em0` and the
   entry to update is `host.mydomain.com`, as `root`, do:

    ```sh
       # cp bin/openbsd-amd64/ifchange-ddns /usr/local/bin/
       # mkdir /etc/ifchanged
       # cp dist/keyfile.conf /etc/ifchanged/ddns.conf
       # cp dist/openbsd/ifchanged /etc/rc.d
       # cat <<EOF >> /etc/rc.conf.local
       # ifchanged_flags="em0 host.mydomain.com /etc/ifchanged/ddns.conf"
       # ifchanged_user=nobody
       # EOF
       # chmod -R og-rw /etc/ifchanged
       # chown nobody /etc/ifchanged
    ```

    Next, edit `/etc/ifchanged/ddns.conf` with the correct DDNS
    update key from the control panel. The example in
    `dist/keyfile.conf` is for *reference* only.

    Finally, start the daemon:

    ```sh
       # /etc/rc.d/ifchanged start
    ```

    If you want the daemon to start at every boot, add "ifchanged"
    to `pkg_scripts` in `/etc/rc.conf.local`



## Where do I see logs?
By default, the daemon sends logs to syslog

## Can I use this on a different platform?
Yes, on platforms that have a hook for `dhclient-script`, you can
use this as a "oneshot" update program:

```sh
    $ ifchange-ddns --oneshot em0 host.mydomain.com /etc/ifchanged/ddns.conf
```

You may have to write a small shell script to parse the input
provided by `dhclient-script` and invoke `ifchange-ddns`.

