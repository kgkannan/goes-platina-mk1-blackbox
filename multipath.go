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

func mpNetTest(t *testing.T) {
	mpTest(t, netport.FourNets)
}

func mpNetIp6Test(t *testing.T) {
	mpTest(t, netport.FourNetsIp6)
}

func mpTest(t *testing.T, netdevs netport.NetDevs) {
	test.SkipIfDryRun(t)
	assert := test.Assert{t}
	defer nsifDelNets(netdevs).Test(t)
	for i := range netdevs {
		nd := &netdevs[i]
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
		staticRoute(netdevs),
		pingRemotesP(netdevs),
		removeLastRoute(netdevs),
		pingRemotesP(netdevs),
		pingGateways(netdevs),
		removeRoutePingGW(netdevs),
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
						test.Pause.Prompt("ping failed")
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
	if failed {
		test.Pause.Prompt("Failed")
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
			is_ip6 := test.IsIPv6(r.Prefix)
			if is_ip6 {
				dummy_ifa_h1 = "fc:5:5:5:5:5:5:5"
				dummy_ifa_h2 = "fc:6:6:6:6:6:6:6"
			}
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
	if failed {
		test.Pause.Prompt("Failed")
	}
	assert.False(failed)
}
