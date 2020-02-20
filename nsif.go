// Copyright © 2015-2016 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/platinasystems/test"
	"github.com/platinasystems/test/netport"
)

func nsifNetTest(t *testing.T) {
	nsifTest(t, netport.OneNet)
}

func nsifIp6NetTest(t *testing.T) {
	nsifTest(t, netport.OneNetIp6)
}

func nsifTest(t *testing.T, netdevs netport.NetDevs) {
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
		ifname := netport.PortByNetPort[nd.NetPort]
		nd.Ifname = ifname
		family := test.IpFamily(nd.Ifa)
		assert.Program("ip", "link", "set", ifname, "up",
			"netns", ns)
		assert.Program("ip", "netns", "exec", ns,
			"ip", family, "address", "add", nd.Ifa,
			"dev", ifname)
	}
	test.Tests{
		nsifPing(netdevs),
		nsifNeighbor(netdevs),
		nsifDelNets(netdevs),
		nsifNoNeighbor(netdevs),
	}.Test(t)
}

type nsifPing []netport.NetDev

func (nsifPing) String() string { return "ping" }

func (nsif nsifPing) Test(t *testing.T) {
	assert := test.Assert{t}
	for _, nd := range []netport.NetDev(nsif) {
		for _, r := range nd.Remotes {
			assert.Ping(nd.Netns, r)
		}
	}
}

type nsifNeighbor []netport.NetDev

func (nsifNeighbor) String() string { return "neighbor" }

func (nsif nsifNeighbor) Test(t *testing.T) {
	assert := test.Assert{t}
	retries := 3
	var not_found bool
	//FIXME, this is just the xeth, not necessary what got added to TH
	xargs := []string{*Goes, "fe1", "xeth", "neigh"}
	time.Sleep(1 * time.Second)
	for i := retries; i > 0; i-- {
		not_found = false
		if *test.VVV {
			t.Log(xargs)
		}
		out, _ := exec.Command(xargs[0], xargs[1:]...).Output()
		sout := strings.TrimSpace(string(out))
		for _, nd := range []netport.NetDev(nsif) {
			for _, r := range nd.Remotes {
				t.Log("matching", r, "with", sout)
				re := regexp.MustCompile(r)
				match := re.FindAllStringSubmatch(sout, -1)
				if len(match) == 0 {
					not_found = true
				}
			}
		}
		if not_found && *test.VV {
			t.Log(i-1, "retries left\n", sout)
		}
		time.Sleep(1 * time.Second)
	}
	if not_found {
		test.Pause.Prompt("Failed")
		assert.Nil(fmt.Errorf("no neighbor found"))
	}
}

// delete namespace without first moving interface(s) out to default ns
// verify interface is now back in default namespace anyway
type nsifDelNets []netport.NetDev

func (nsifDelNets) String() string { return "del-netns" }

func (nsif nsifDelNets) Test(t *testing.T) {
	assert := test.Assert{t}
	for _, nd := range []netport.NetDev(nsif) {
		ns := nd.Netns
		_, err := os.Stat(filepath.Join("/var/run/netns", ns))
		if err == nil {
			assert.Program("ip", "netns", "del", ns)
		}
	}
}

type nsifNoNeighbor []netport.NetDev

func (nsifNoNeighbor) String() string { return "no-neighbor" }

func (nsif nsifNoNeighbor) Test(t *testing.T) {
	assert := test.Assert{t}
	//FIXME, this is just the xeth, not necessary what got added to TH
	xargs := []string{*Goes, "fe1", "xeth", "neigh"}
	if *test.VVV {
		t.Log(xargs)
	}
	out, _ := exec.Command(xargs[0], xargs[1:]...).Output()
	sout := strings.TrimSpace(string(out))
	found := false
	for _, nd := range []netport.NetDev(nsif) {
		for _, r := range nd.Remotes {
			re := regexp.MustCompile(r)
			match := re.FindAllStringSubmatch(sout, -1)
			if len(match) > 0 {
				found = true
			}
		}
	}
	if found {
		if *test.VV {
			t.Log(sout)
		}
		assert.Nil(fmt.Errorf("leftover neighbor found"))
	}
	// check leftover adjacencies as well
	AssertNoAdjacencies(t)
}
