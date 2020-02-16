// Copyright © 2015-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/platinasystems/test"
	"github.com/platinasystems/test/docker"
)

func gobgpNetTest(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	gobgpTest(t, "testdata/gobgp/ebgp/conf.yaml.tmpl")
	test.SkipIfDryRun(t)
}

func gobgpVlanTest(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	gobgpTest(t, "testdata/gobgp/ebgp/vlan/conf.yaml.tmpl")
	test.SkipIfDryRun(t)
}

func gobgpTest(t *testing.T, tmpl string) {
	docket := &docker.Docket{Tmpl: tmpl}
	docket.Test(t,
		gobgpConnectivity{docket},
		gobgpDaemon{docket},
		gobgpNeighbors{docket},
		gobgpRoutes{docket},
		gobgpInterConnectivity{docket},
		gobgpFlap{docket},
		gobgpAdminDown{docket})
}

type gobgpConnectivity struct{ *docker.Docket }

func (gobgpConnectivity) String() string { return "connectivity" }

func (gobgp gobgpConnectivity) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		host   string
		target string
	}{
		{"R1", "192.168.120.10"},
		{"R1", "192.168.150.4"},
		{"R1", "192.168.1.5"},

		{"R2", "192.168.120.5"},
		{"R2", "192.168.222.2"},
		{"R2", "192.168.1.10"},

		{"R3", "192.168.222.10"},
		{"R3", "192.168.111.4"},
		{"R3", "192.168.2.2"},

		{"R4", "192.168.111.2"},
		{"R4", "192.168.150.5"},
		{"R4", "192.168.2.4"},
	} {
		err := gobgp.PingCmd(t, x.host, x.target)
		assert.Nil(err)
	}
}

type gobgpDaemon struct{ *docker.Docket }

func (gobgpDaemon) String() string { return "daemon" }

func (gobgp gobgpDaemon) Test(t *testing.T) {
	assert := test.Assert{t}
	time.Sleep(1 * time.Second)
	fail := false
	for _, r := range gobgp.Routers {
		assert.Comment("ing gobgp on", r.Hostname)
		out, err := gobgp.ExecCmd(t, r.Hostname, "ps", "ax")
		assert.Nil(err)
		//assert.Match(out, ".*gobgpd.*")
		//assert.Match(out, ".*zebra.*")
		timeout := 5 //for some reason, R4 gobpg takes longer to come up sometimes
		found := false
		for i := timeout; i > 0; i-- {
			if !assert.MatchNonFatal(out, ".*gobgpd.*") {
				if *test.VV {
					fmt.Printf("%v ps ax, no match on gobgpd, %v retries left\n", r.Hostname, i-1)
					fmt.Printf("%v\n", out)
				}
				time.Sleep(2 * time.Second)
				out, err = gobgp.ExecCmd(t, r.Hostname, "ps", "ax")
				continue
			}
			if !assert.MatchNonFatal(out, ".*zebra.*") {
				if *test.VV {
					fmt.Printf("%v ps ax, no match on zebra, %v retries left\n", r.Hostname, i-1)
					fmt.Printf("%v\n", out)
				}
				out, err = gobgp.ExecCmd(t, r.Hostname, "ps", "ax")
				time.Sleep(2 * time.Second)
				continue
			}
			found = true
		}
		if !found {
			fail = true
		}
	}
	if fail {
		assert.Nil(fmt.Errorf("check gobgpd and zebra failed\n"))
	}
}

type gobgpNeighbors struct{ *docker.Docket }

func (gobgpNeighbors) String() string { return "neighbors" }

func (gobgp gobgpNeighbors) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		peer     string
	}{
		{"R1", "192.168.120.10"},
		{"R1", "192.168.150.4"},

		{"R2", "192.168.120.5"},
		{"R2", "192.168.222.2"},

		{"R3", "192.168.222.10"},
		{"R3", "192.168.111.4"},

		{"R4", "192.168.111.2"},
		{"R4", "192.168.150.5"},
	} {
		found := false
		timeout := 120

		for i := timeout; i > 0; i-- {
			out, err := gobgp.ExecCmd(t, x.hostname,
				"/root/gobgp", "neighbor", x.peer)
			assert.Nil(err)
			if !assert.MatchNonFatal(out,
				".*state = established.*") {
				time.Sleep(1 * time.Second)
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("No bgp peer established for %v", x.hostname)
		}
		_, err := gobgp.ExecCmd(t, x.hostname,
			"/root/gobgp", "global", "rib")
		assert.Nil(err)
	}
}

type gobgpRoutes struct{ *docker.Docket }

func (gobgpRoutes) String() string { return "routes" }

func (gobgp gobgpRoutes) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		route    string
	}{
		{"R1", "192.168.222.0/24"},
		{"R1", "192.168.111.0/24"},
		{"R1", "192.168.1.10/32"},
		{"R1", "192.168.2.2/32"},
		{"R1", "192.168.2.4/32"},

		{"R2", "192.168.150.0/24"},
		{"R2", "192.168.111.0/24"},
		{"R2", "192.168.1.5/32"},
		{"R2", "192.168.2.2/32"},
		{"R2", "192.168.2.4/32"},

		{"R3", "192.168.120.0/24"},
		{"R3", "192.168.150.0/24"},
		{"R3", "192.168.1.5/32"},
		{"R3", "192.168.1.10/32"},
		{"R3", "192.168.2.4/32"},

		{"R4", "192.168.120.0/24"},
		{"R4", "192.168.222.0/24"},
		{"R4", "192.168.1.5/32"},
		{"R4", "192.168.1.10/32"},
		{"R4", "192.168.2.2/32"},
	} {
		found := false
		timeout := 60
		for i := timeout; i > 0; i-- {
			out, err := gobgp.ExecCmd(t, x.hostname,
				"vtysh", "-c", "show ip route "+x.route)
			assert.Nil(err)
			if !assert.MatchNonFatal(out, x.route) {
				time.Sleep(1 * time.Second)
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("No bgp route for %v: %v", x.hostname, x.route)
		}
	}
}

type gobgpInterConnectivity struct{ *docker.Docket }

func (gobgpInterConnectivity) String() string { return "inter-connectivity" }

func (gobgp gobgpInterConnectivity) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		target   string
	}{

		{"R1", "192.168.222.2"},
		{"R1", "192.168.111.2"},
		{"R1", "192.168.111.4"},
		{"R1", "192.168.1.10"},
		{"R1", "192.168.2.2"},
		{"R1", "192.168.2.4"},

		{"R2", "192.168.111.4"},
		{"R2", "192.168.111.2"},
		{"R2", "192.168.150.4"},
		{"R2", "192.168.1.5"},
		{"R2", "192.168.2.2"},
		{"R2", "192.168.2.4"},

		{"R3", "192.168.120.5"},
		{"R3", "192.168.150.4"},
		{"R3", "192.168.150.5"},
		{"R3", "192.168.1.5"},
		{"R3", "192.168.1.10"},
		{"R3", "192.168.2.4"},

		{"R4", "192.168.120.10"},
		{"R4", "192.168.222.2"},
		{"R4", "192.168.222.10"},
		{"R4", "192.168.1.5"},
		{"R4", "192.168.1.10"},
		{"R4", "192.168.2.2"},
	} {
		assert.Nil(gobgp.PingCmd(t, x.hostname, x.target))
		assert.Program(*Goes, "fe1", "switch", "fib")
	}
}

type gobgpFlap struct{ *docker.Docket }

func (gobgpFlap) String() string { return "flap" }

func (gobgp gobgpFlap) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, r := range gobgp.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := gobgp.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			_, err = gobgp.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "up", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			assert.Program(*Goes, "fe1", "switch", "fib")
		}
	}
}

type gobgpAdminDown struct{ *docker.Docket }

func (gobgpAdminDown) String() string { return "admin-down" }

func (gobgp gobgpAdminDown) Test(t *testing.T) {
	assert := test.Assert{t}

	num_intf := 0
	for _, r := range gobgp.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := gobgp.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			num_intf++
		}
	}
	AssertNoAdjacencies(t)
}
