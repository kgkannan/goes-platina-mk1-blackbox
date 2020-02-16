// Copyright © 2015-2017 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"testing"
	"time"

	"github.com/platinasystems/test"
	"github.com/platinasystems/test/docker"
)

func frrNetTest(t *testing.T) {
	t.Run("bgp", frrNetBgpTest)
	t.Run("ospf", frrNetOspfTest)
	t.Run("isis", frrNetIsisTest)
	test.SkipIfDryRun(t)
}

func frrVlanTest(t *testing.T) {
	t.Run("bgp", frrVlanBgpTest)
	t.Run("ospf", frrVlanOspfTest)
	t.Run("isis", frrVlanIsisTest)
	test.SkipIfDryRun(t)
}

func frrNetBgpTest(t *testing.T) {
	frrBgpTest(t, "testdata/frr/bgp/conf.yaml.tmpl")
}

func frrVlanBgpTest(t *testing.T) {
	frrBgpTest(t, "testdata/frr/bgp/vlan/conf.yaml.tmpl")
}

func frrBgpTest(t *testing.T, tmpl string) {
	docket := &docker.Docket{Tmpl: tmpl}
	docket.Test(t,
		frrBgpConnectivity{docket},
		frrBgpDaemons{docket},
		frrBgpNeighbors{docket},
		frrBgpRoutes{docket},
		frrBgpInterConnectivity{docket},
		frrBgpFlap{docket},
		frrBgpConnectivity{docket},
		frrBgpAdminDown{docket})
}

func frrNetOspfTest(t *testing.T) {
	frrOspfTest(t, "testdata/frr/ospf/conf.yaml.tmpl")
}

func frrVlanOspfTest(t *testing.T) {
	frrOspfTest(t, "testdata/frr/ospf/vlan/conf.yaml.tmpl")
}

func frrOspfTest(t *testing.T, tmpl string) {
	docket := &docker.Docket{Tmpl: tmpl}
	docket.Test(t,
		frrOspfCarrier{docket},
		frrOspfConnectivity{docket},
		frrOspfDaemons{docket},
		frrOspfNeighbors{docket},
		frrOspfRoutes{docket},
		frrOspfInterConnectivity{docket},
		frrOspfFlap{docket},
		frrOspfConnectivity{docket},
		frrOspfAdminDown{docket})
}

func frrNetIsisTest(t *testing.T) {
	frrIsisTest(t, "testdata/frr/isis/conf.yaml.tmpl")
}

func frrVlanIsisTest(t *testing.T) {
	frrIsisTest(t, "testdata/frr/isis/vlan/conf.yaml.tmpl")
}

func frrIsisTest(t *testing.T, tmpl string) {
	docket := &docker.Docket{Tmpl: tmpl}
	docket.Test(t,
		frrIsisConnectivity{docket},
		frrIsisDaemons{docket},
		frrIsisAddIntfConf{docket},
		frrIsisNeighbors{docket},
		frrIsisRoutes{docket},
		frrIsisInterConnectivity{docket},
		frrIsisFlap{docket},
		frrIsisConnectivity{docket},
		frrIsisAdminDown{docket})
}

type frrBgpConnectivity struct{ *docker.Docket }

func (frrBgpConnectivity) String() string { return "connectivity" }

func (frr frrBgpConnectivity) Test(t *testing.T) {
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
		assert.Nil(frr.PingCmd(t, x.host, x.target))
	}
}

type frrBgpDaemons struct{ *docker.Docket }

func (frrBgpDaemons) String() string { return "daemons" }

func (frr frrBgpDaemons) Test(t *testing.T) {
	assert := test.Assert{t}
	time.Sleep(1 * time.Second)
	for _, r := range frr.Routers {
		assert.Comment("ing FRR on", r.Hostname)
		out, err := frr.ExecCmd(t, r.Hostname, "ps", "ax")
		assert.Nil(err)
		assert.Match(out, ".*bgpd.*")
		assert.Match(out, ".*zebra.*")
	}
}

type frrBgpNeighbors struct{ *docker.Docket }

func (frrBgpNeighbors) String() string { return "neighbors" }

func (frr frrBgpNeighbors) Test(t *testing.T) {
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
			out, err := frr.ExecCmd(t, x.hostname,
				"vtysh", "-c", "show ip bgp neighbor "+x.peer)
			assert.Nil(err)
			if !assert.MatchNonFatal(out, ".*state = Established.*") {
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

type frrBgpRoutes struct{ *docker.Docket }

func (frrBgpRoutes) String() string { return "routes" }

func (frr frrBgpRoutes) Test(t *testing.T) {
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
			out, err := frr.ExecCmd(t, x.hostname,
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

type frrBgpInterConnectivity struct{ *docker.Docket }

func (frrBgpInterConnectivity) String() string { return "inter-connectivity" }

func (frr frrBgpInterConnectivity) Test(t *testing.T) {
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
		err := frr.PingCmd(t, x.hostname, x.target)
		assert.Nil(err)
		assert.Program(*Goes, "fe1", "switch", "fib")
	}
}

type frrBgpFlap struct{ *docker.Docket }

func (frrBgpFlap) String() string { return "flap" }

func (frr frrBgpFlap) Test(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	assert := test.Assert{t}

	for _, r := range frr.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := frr.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			_, err = frr.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "up", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			assert.Program(*Goes, "fe1", "switch", "fib")
		}
	}
}

type frrBgpAdminDown struct{ *docker.Docket }

func (frrBgpAdminDown) String() string { return "admin-down" }

func (frr frrBgpAdminDown) Test(t *testing.T) {
	assert := test.Assert{t}

	num_intf := 0
	for _, r := range frr.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := frr.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			num_intf++
		}
	}
	AssertNoAdjacencies(t)
}

type frrOspfCarrier struct{ *docker.Docket }

func (frrOspfCarrier) String() string { return "carrier" }

func (frr frrOspfCarrier) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, r := range frr.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			assert.Comment("check carrier for", r.Hostname,
				"on", intf)
			assert.Nil(test.Carrier(r.Hostname, intf))
		}
	}
}

type frrOspfConnectivity struct{ *docker.Docket }

func (frrOspfConnectivity) String() string { return "connectivity" }

func (frr frrOspfConnectivity) Test(t *testing.T) {
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
		assert.Nil(frr.PingCmd(t, x.host, x.target))
	}
}

type frrOspfDaemons struct{ *docker.Docket }

func (frrOspfDaemons) String() string { return "daemons" }

func (frr frrOspfDaemons) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, r := range frr.Routers {
		assert.Comment("ing FRR on", r.Hostname)
		out, err := frr.ExecCmd(t, r.Hostname, "ps", "ax")
		assert.Nil(err)
		assert.Match(out, ".*ospfd.*")
		assert.Match(out, ".*zebra.*")
	}
}

type frrOspfNeighbors struct{ *docker.Docket }

func (frrOspfNeighbors) String() string { return "neighbors" }

func (frr frrOspfNeighbors) Test(t *testing.T) {
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
			out, err := frr.ExecCmd(t, x.hostname,
				"vtysh", "-c", "show ip ospf neighbor")
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

type frrOspfRoutes struct{ *docker.Docket }

func (frrOspfRoutes) String() string { return "routes" }

func (frr frrOspfRoutes) Test(t *testing.T) {
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
			out, err := frr.ExecCmd(t, x.hostname,
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

type frrOspfInterConnectivity struct{ *docker.Docket }

func (frrOspfInterConnectivity) String() string { return "inter-connectivity" }

func (frr frrOspfInterConnectivity) Test(t *testing.T) {
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
		assert.Nil(frr.PingCmd(t, x.hostname, x.target))
		assert.Program(*Goes, "fe1", "switch", "fib")
	}
}

type frrOspfFlap struct{ *docker.Docket }

func (frrOspfFlap) String() string { return "flap" }

func (frr frrOspfFlap) Test(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	assert := test.Assert{t}

	for _, r := range frr.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := frr.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			_, err = frr.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "up", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			assert.Program(*Goes, "fe1", "switch", "fib")
		}
	}
}

type frrOspfAdminDown struct{ *docker.Docket }

func (frrOspfAdminDown) String() string { return "admin-down" }

func (frr frrOspfAdminDown) Test(t *testing.T) {
	assert := test.Assert{t}

	num_intf := 0
	for _, r := range frr.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := frr.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			num_intf++
		}
	}
	AssertNoAdjacencies(t)
}

type frrIsisConnectivity struct{ *docker.Docket }

func (frrIsisConnectivity) String() string { return "connectivity" }

func (frr frrIsisConnectivity) Test(t *testing.T) {
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
		assert.Nil(frr.PingCmd(t, x.host, x.target))
	}
}

type frrIsisDaemons struct{ *docker.Docket }

func (frrIsisDaemons) String() string { return "daemons" }

func (frr frrIsisDaemons) Test(t *testing.T) {
	assert := test.Assert{t}
	time.Sleep(1 * time.Second)

	for _, r := range frr.Routers {
		assert.Comment("ing FRR on", r.Hostname)
		out, err := frr.ExecCmd(t, r.Hostname, "ps", "ax")
		assert.Nil(err)
		assert.Match(out, ".*isisd.*")
		assert.Match(out, ".*zebra.*")
	}
}

type frrIsisAddIntfConf struct{ *docker.Docket }

func (frrIsisAddIntfConf) String() string { return "add-intf-conf" }

func (frr frrIsisAddIntfConf) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, r := range frr.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := frr.ExecCmd(t, r.Hostname,
				"vtysh", "-c", "conf t",
				"-c", "interface "+intf,
				"-c", "ip router isis "+r.Hostname)
			assert.Nil(err)
		}
	}
}

type frrIsisNeighbors struct{ *docker.Docket }

func (frrIsisNeighbors) String() string { return "neighbors" }

func (frr frrIsisNeighbors) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		peer     string
		address  string
	}{
		{"R1", "R2", "192.168.120.10"},
		{"R1", "R4", "192.168.150.4"},
		{"R2", "R1", "192.168.120.5"},
		{"R2", "R3", "192.168.222.2"},
		{"R3", "R2", "192.168.222.10"},
		{"R3", "R4", "192.168.111.4"},
		{"R4", "R3", "192.168.111.2"},
		{"R4", "R1", "192.168.150.5"},
	} {
		timeout := 60
		found := false
		for i := timeout; i > 0; i-- {
			out, err := frr.ExecCmd(t, x.hostname,
				"vtysh", "-c", "show isis neighbor "+x.peer)
			assert.Nil(err)
			if !assert.MatchNonFatal(out, x.address) {
				time.Sleep(1 * time.Second)
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("No isis neighbor for %v: %v",
				x.hostname, x.peer)
		}
	}
}

type frrIsisRoutes struct{ *docker.Docket }

func (frrIsisRoutes) String() string { return "routes" }

func (frr frrIsisRoutes) Test(t *testing.T) {
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
			out, err := frr.ExecCmd(t, x.hostname,
				"vtysh", "-c", "show ip route isis")
			assert.Nil(err)
			if !assert.MatchNonFatal(out, x.route) {
				time.Sleep(1 * time.Second)
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("No isis route for %v: %v", x.hostname, x.route)
		}
	}
}

type frrIsisInterConnectivity struct{ *docker.Docket }

func (frrIsisInterConnectivity) String() string { return "inter-connectivity" }

func (frr frrIsisInterConnectivity) Test(t *testing.T) {
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
		assert.Nil(frr.PingCmd(t, x.hostname, x.target))
		assert.Program(*Goes, "fe1", "switch", "fib")
	}
}

type frrIsisFlap struct{ *docker.Docket }

func (frrIsisFlap) String() string { return "flap" }

func (frr frrIsisFlap) Test(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	assert := test.Assert{t}

	for _, r := range frr.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := frr.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			_, err = frr.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "up", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			assert.Program(*Goes, "fe1", "switch", "fib")
		}
	}
}

type frrIsisAdminDown struct{ *docker.Docket }

func (frrIsisAdminDown) String() string { return "admin-down" }

func (frr frrIsisAdminDown) Test(t *testing.T) {
	assert := test.Assert{t}

	num_intf := 0
	for _, r := range frr.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := frr.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			num_intf++
		}
	}
	AssertNoAdjacencies(t)
}
