// Copyright © 2015-2016 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/platinasystems/test"
	"github.com/platinasystems/test/netport"
)

func mpTest(t *testing.T) {
	test.SkipIfDryRun(t)
	assert := test.Assert{t}
	defer nsifDelNets(netport.FourNets).Test(t)
	for i := range netport.FourNets {
		nd := &netport.FourNets[i]
		ns := nd.Netns
		_, err := os.Stat(filepath.Join("/var/run/netns", ns))
		if err != nil {
			assert.Program("ip", "netns", "add", ns)
		}
		assert.Program("ip", "netns", "exec", ns, "sysctl", "-w", "net/ipv4/conf/all/rp_filter=0")
		ifname := netport.PortByNetPort[nd.NetPort]
		nd.Ifname = ifname
		assert.Program("ip", "link", "set", ifname, "up",
			"netns", ns)
		assert.Program("ip", "netns", "exec", ns,
			"ip", "address", "add", nd.Ifa,
			"dev", ifname)
		assert.Program("ip", "netns", "exec", ns, "sysctl", "-w", "net/ipv4/conf/"+ifname+"/rp_filter=0")
		for _, dIf := range nd.DummyIfs {
			assert.Program("ip", "netns", "exec", ns, "ip", "link", "add", dIf.Ifname, "type", "dummy")
			assert.Program("ip", "netns", "exec", ns, "ip", "link", "set", dIf.Ifname, "up")
			assert.Program("ip", "netns", "exec", ns, "ip", "addr", "add", dIf.Ifa, "dev", dIf.Ifname)
		}
	}
	test.Tests{
		staticRoute(netport.FourNets),
		pingRemotesP(netport.FourNets),
		removeLastRoute(netport.FourNets),
		pingRemotesP(netport.FourNets),
		pingGateways(netport.FourNets),
		removeRoutePingGW(netport.FourNets),
	}.Test(t)
}

func mpIp6Test(t *testing.T) {
	test.SkipIfDryRun(t)
	assert := test.Assert{t}
	defer nsifDelNets(netport.FourIp6Nets).Test(t)
	for i := range netport.FourIp6Nets {
		nd := &netport.FourIp6Nets[i]
		ns := nd.Netns
		_, err := os.Stat(filepath.Join("/var/run/netns", ns))
		if err != nil {
			assert.Program("ip", "netns", "add", ns)
		}
		assert.Program("ip", "netns", "exec", ns, "sysctl", "-w", "net/ipv6/conf/all/rp_filter=0")
		ifname := netport.PortByNetPort[nd.NetPort]
		nd.Ifname = ifname
		assert.Program("ip", "link", "set", ifname, "up",
			"netns", ns)
		assert.Program("ip", "netns", "exec", ns,
			"ip", "-6", "address", "add", nd.Ifa,
			"dev", ifname)
		assert.Program("ip", "netns", "exec", ns, "sysctl", "-w", "net/ipv4/conf/"+ifname+"/rp_filter=0")
		for _, dIf := range nd.DummyIfs {
			assert.Program("ip", "netns", "exec", ns, "ip", "link", "add", dIf.Ifname, "type", "dummy")
			assert.Program("ip", "netns", "exec", ns, "ip", "link", "set", dIf.Ifname, "up")
			assert.Program("ip", "netns", "exec", ns, "ip", "-6", "addr", "add", dIf.Ifa, "dev", dIf.Ifname)
		}
	}
	test.Tests{
		staticRoute(netport.FourIp6Nets),
		pingRemotesP(netport.FourIp6Nets),
		removeLastRoute(netport.FourIp6Nets),
		pingRemotesP(netport.FourIp6Nets),
		pingGateways(netport.FourIp6Nets),
		removeRoutePingGW(netport.FourIp6Nets),
	}.Test(t)
}

type staticRoute []netport.NetDev

func (staticRoute) String() string { return "staticRoute" }

func (mp staticRoute) Test(t *testing.T) {
	assert := test.Assert{t}
	for _, nd := range []netport.NetDev(mp) {
		for _, r := range nd.Routes {
			assert.Program("ip", "netns", "exec", nd.Netns,
				"ip", "route", "append", r.Prefix, "via",
				r.GW)
		}
	}
}

type pingRemotesP []netport.NetDev

func (pingRemotesP) String() string { return "pingRemoteP" }

func (mp pingRemotesP) Test(t *testing.T) {
	assert := test.Assert{t}
	max_retries := 3
	wait_time := 2 * time.Second
	failed := false
	for n := 0; n < max_retries; n++ {
		for _, nd := range []netport.NetDev(mp) {
			for _, r := range nd.Remotes {
				if ok := assert.PingNonFatal(nd.Netns, r); !ok {
					failed = true
					if n == max_retries-1 {
						fmt.Println(nd.Netns, "ping", r, "failed")
					}
				}
			}
		}
		if !failed {
			break
		}
		if n < max_retries-1 {
			failed = false
			time.Sleep(wait_time)
		}
	}
	assert.False(failed)
}

type removeLastRoute []netport.NetDev

func (removeLastRoute) String() string { return "removeLastRoute" }

func (mp removeLastRoute) Test(t *testing.T) {
	assert := test.Assert{t}
	// remove route via to remote dummy from the last 2 nets
	dummy_ifa_h1 := "10.5.5.5"
	dummy_ifa_h2 := "10.6.6.6"
	for ni, nd := range []netport.NetDev(mp) {
		if ni < 4 {
			continue
		}
		for _, r := range nd.Routes {
			if r.Prefix == dummy_ifa_h1 || r.Prefix == dummy_ifa_h2 {
				assert.Program("ip", "netns", "exec", nd.Netns,
					"ip", "route", "del", r.Prefix, "via",
					r.GW)
			}
		}
	}
}

type removeRoutePingGW []netport.NetDev

func (removeRoutePingGW) String() string { return "removeRoutePingGw" }

func (mp removeRoutePingGW) Test(t *testing.T) {
	assert := test.Assert{t}
	var gw map[string]string
	gw = make(map[string]string)
	// get the gateway from first 4 nets
	for ni, nd := range []netport.NetDev(mp) {
		if ni > 3 {
			break
		}
		netns := nd.Netns
		if len(nd.Routes) > 0 {
			gw[netns] = nd.Routes[0].GW
		}
	}
	// remove all routes, leaving only local and glean
	for _, nd := range []netport.NetDev(mp) {
		for _, r := range nd.Routes {
			assert.ProgramNonFatal("ip", "netns", "exec", nd.Netns,
				"ip", "route", "del", r.Prefix, "via",
				r.GW)
		}
	}
	// now ping the gateway
	max_retries := 3
	wait_time := 2 * time.Second
	failed := false
	for n := 0; n < max_retries; n++ {
		for netns, ip := range gw {
			if ok := assert.PingNonFatal(netns, ip); !ok {
				failed = true
				if n == max_retries-1 {
					fmt.Println(netns, "ping", ip, "failed")
				}
			}
		}
		if !failed {
			break
		}
		if n < max_retries-1 {
			failed = false
			time.Sleep(wait_time)
		}
	}
	assert.False(failed)
}
