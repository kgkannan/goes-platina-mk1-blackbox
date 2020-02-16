// Copyright © 2015-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"testing"
	"time"

	"github.com/platinasystems/test"
	"github.com/platinasystems/test/docker"
)

func birdNetTest(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Run("bgp", birdNetBgpTest)
	t.Run("ospf", birdNetOspfTest)
	test.SkipIfDryRun(t)
}

func birdVlanTest(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Run("bgp", birdVlanBgpTest)
	t.Run("ospf", birdVlanOspfTest)
	test.SkipIfDryRun(t)
}

func birdNetBgpTest(t *testing.T) {
	birdBgpTest(t, "testdata/bird/bgp/conf.yaml.tmpl")
}

func birdVlanBgpTest(t *testing.T) {
	birdBgpTest(t, "testdata/bird/bgp/vlan/conf.yaml.tmpl")
}

func birdBgpTest(t *testing.T, tmpl string) {
	docket := &docker.Docket{Tmpl: tmpl}
	docket.Test(t,
		birdBgpConnectivity{docket},
		birdBgpDaemon{docket},
		birdBgpNeighbors{docket},
		birdBgpRoutes{docket},
		birdBgpInterConnectivity{docket},
		birdBgpFlap{docket},
		birdBgpConnectivity{docket},
		birdBgpAdminDown{docket})
}

func birdNetOspfTest(t *testing.T) {
	birdOspfTest(t, "testdata/bird/ospf/conf.yaml.tmpl")
}

func birdVlanOspfTest(t *testing.T) {
	birdOspfTest(t, "testdata/bird/ospf/vlan/conf.yaml.tmpl")
}

func birdOspfTest(t *testing.T, tmpl string) {
	docket := &docker.Docket{Tmpl: tmpl}
	docket.Test(t,
		birdOspfConnectivity{docket},
		birdOspfDaemon{docket},
		birdOspfNeighbors{docket},
		birdOspfRoutes{docket},
		birdOspfInterConnectivity{docket},
		birdOspfFlap{docket},
		birdOspfReconnectivity{docket},
		birdOspfAdminDown{docket})
}

type birdBgpConnectivity struct{ *docker.Docket }

func (birdBgpConnectivity) String() string { return "connectivity" }

func (bird birdBgpConnectivity) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		host   string
		target string
	}{
		{"R1", "192.168.120.10"},
		{"R1", "192.168.150.4"},
		{"R2", "192.168.222.2"},
		{"R2", "192.168.120.5"},
		{"R3", "192.168.222.10"},
		{"R3", "192.168.111.4"},
		{"R4", "192.168.111.2"},
		{"R4", "192.168.150.5"},
	} {
		assert.Nil(bird.PingCmd(t, x.host, x.target))
	}
}

type birdBgpDaemon struct{ *docker.Docket }

func (birdBgpDaemon) String() string { return "daemon" }

func (bird birdBgpDaemon) Test(t *testing.T) {
	assert := test.Assert{t}
	time.Sleep(1 * time.Second)
	for _, r := range bird.Routers {
		assert.Comment("Checking BIRD on", r.Hostname)
		out, err := bird.ExecCmd(t, r.Hostname, "ps", "ax")
		assert.Nil(err)
		assert.Match(out, ".*bird.*")
	}
}

type birdBgpNeighbors struct{ *docker.Docket }

func (birdBgpNeighbors) String() string { return "neighbors" }

func (bird birdBgpNeighbors) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		peer     string
	}{
		{"R1", "R2"},
		{"R1", "R4"},
		{"R2", "R1"},
		{"R2", "R3"},
		{"R3", "R2"},
		{"R3", "R4"},
		{"R4", "R1"},
		{"R4", "R3"},
	} {
		found := false
		timeout := 120

		for i := timeout; i > 0; i-- {
			out, err := bird.ExecCmd(t, x.hostname,
				"birdc", "show", "protocols", "all", x.peer)
			assert.Nil(err)
			if !assert.MatchNonFatal(out, ".*Established.*") {
				time.Sleep(1 * time.Second)
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("No bgp peer established for %v", x.hostname)
		}
	}
}

type birdBgpRoutes struct{ *docker.Docket }

func (birdBgpRoutes) String() string { return "routes" }

func (bird birdBgpRoutes) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		route    string
	}{
		{"R1", "192.168.222.0/24"},
		{"R1", "192.168.111.0/24"},
		{"R2", "192.168.150.0/24"},
		{"R2", "192.168.111.0/24"},
		{"R3", "192.168.120.0/24"},
		{"R3", "192.168.150.0/24"},
		{"R4", "192.168.120.0/24"},
		{"R4", "192.168.222.0/24"},
	} {
		found := false
		timeout := 60
		for i := timeout; i > 0; i-- {
			out, err := bird.ExecCmd(t, x.hostname,
				"ip", "route", "show", x.route)
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

type birdBgpInterConnectivity struct{ *docker.Docket }

func (birdBgpInterConnectivity) String() string { return "inter-connectivity" }

func (bird birdBgpInterConnectivity) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		target   string
	}{
		{"R1", "192.168.222.2"},
		{"R1", "192.168.111.2"},
		{"R2", "192.168.111.4"},
		{"R2", "192.168.150.4"},
		{"R3", "192.168.120.5"},
		{"R3", "192.168.150.5"},
		{"R4", "192.168.120.10"},
		{"R4", "192.168.222.10"},
	} {
		assert.Nil(bird.PingCmd(t, x.hostname, x.target))
		assert.Program(*Goes, "fe1", "switch", "fib")
	}
}

type birdBgpFlap struct{ *docker.Docket }

func (birdBgpFlap) String() string { return "flap" }

func (bird birdBgpFlap) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, r := range bird.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := bird.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			_, err = bird.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "up", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			assert.Program(*Goes, "fe1", "switch", "fib")
		}
	}
}

type birdBgpAdminDown struct{ *docker.Docket }

func (birdBgpAdminDown) String() string { return "admin-down" }

func (bird birdBgpAdminDown) Test(t *testing.T) {
	assert := test.Assert{t}

	num_intf := 0
	for _, r := range bird.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := bird.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			num_intf++
		}
	}
	AssertNoAdjacencies(t)
}

type birdOspfConnectivity struct{ *docker.Docket }

func (birdOspfConnectivity) String() string { return "connectivity" }

func (bird birdOspfConnectivity) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		host   string
		target string
	}{
		{"R1", "192.168.120.10"},
		{"R1", "192.168.150.4"},
		{"R2", "192.168.222.2"},
		{"R2", "192.168.120.5"},
		{"R3", "192.168.222.10"},
		{"R3", "192.168.111.4"},
		{"R4", "192.168.111.2"},
		{"R4", "192.168.150.5"},
	} {
		assert.Nil(bird.PingCmd(t, x.host, x.target))
	}
}

type birdOspfReconnectivity struct{ *docker.Docket }

func (birdOspfReconnectivity) String() string { return "repeat-connectivity" }

func (bird birdOspfReconnectivity) Test(t *testing.T) {
	birdOspfConnectivity(bird).Test(t)
}

type birdOspfDaemon struct{ *docker.Docket }

func (birdOspfDaemon) String() string { return "daemon" }

func (bird birdOspfDaemon) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, r := range bird.Routers {
		assert.Comment("Checking BIRD on", r.Hostname)
		out, err := bird.ExecCmd(t, r.Hostname, "ps", "ax")
		assert.Nil(err)
		assert.Match(out, ".*bird.*")
	}
}

type birdOspfNeighbors struct{ *docker.Docket }

func (birdOspfNeighbors) String() string { return "neighbors" }

func (bird birdOspfNeighbors) Test(t *testing.T) {
	assert := test.Assert{t}

	timeout := 120

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
		for i := timeout; i > 0; i-- {
			out, err := bird.ExecCmd(t, x.hostname,
				"birdc", "show", "ospf", "neighbor")
			assert.Nil(err)
			if !assert.MatchNonFatal(out, x.peer) {
				time.Sleep(1 * time.Second)
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("No ospf neighbor found for %v", x.hostname)
		}
	}
}

type birdOspfRoutes struct{ *docker.Docket }

func (birdOspfRoutes) String() string { return "routes" }

func (bird birdOspfRoutes) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		route    string
	}{
		{"R1", "192.168.222.0/24"},
		{"R1", "192.168.111.0/24"},
		{"R2", "192.168.150.0/24"},
		{"R2", "192.168.111.0/24"},
		{"R3", "192.168.120.0/24"},
		{"R3", "192.168.150.0/24"},
		{"R4", "192.168.120.0/24"},
		{"R4", "192.168.222.0/24"},
	} {
		found := false
		timeout := 60
		for i := timeout; i > 0; i-- {
			out, err := bird.ExecCmd(t, x.hostname,
				"ip", "route", "show", x.route)
			assert.Nil(err)
			if !assert.MatchNonFatal(out, x.route) {
				time.Sleep(1 * time.Second)
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("No ospf route for %v: %v", x.hostname, x.route)
		}
	}
}

type birdOspfInterConnectivity struct{ *docker.Docket }

func (birdOspfInterConnectivity) String() string { return "inter-connectivity" }

func (bird birdOspfInterConnectivity) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		target   string
	}{
		{"R1", "192.168.222.2"},
		{"R1", "192.168.111.2"},
		{"R2", "192.168.111.4"},
		{"R2", "192.168.150.4"},
		{"R3", "192.168.120.5"},
		{"R3", "192.168.150.5"},
		{"R4", "192.168.120.10"},
		{"R4", "192.168.222.10"},
	} {
		assert.Nil(bird.PingCmd(t, x.hostname, x.target))
		assert.Program(*Goes, "fe1", "switch", "fib")
	}
}

type birdOspfFlap struct{ *docker.Docket }

func (birdOspfFlap) String() string { return "flap" }

func (bird birdOspfFlap) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, r := range bird.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := bird.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			_, err = bird.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "up", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			assert.Program(*Goes, "fe1", "switch", "fib")
		}
	}
}

type birdOspfAdminDown struct{ *docker.Docket }

func (birdOspfAdminDown) String() string { return "admin-down" }

func (bird birdOspfAdminDown) Test(t *testing.T) {
	assert := test.Assert{t}

	num_intf := 0
	for _, r := range bird.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := bird.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			num_intf++
		}
	}
	AssertNoAdjacencies(t)
}
