// Copyright © 2015-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/platinasystems/test"
	"github.com/platinasystems/test/docker"
)

func dhcpNetTest(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	dhcpTest(t, "testdata/net/dhcp/conf.yaml.tmpl")
}

func dhcpVlanTest(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	dhcpTest(t, "testdata/net/dhcp/vlan/conf.yaml.tmpl")
}

func dhcpTest(t *testing.T, tmpl string) {
	docket := &docker.Docket{Tmpl: tmpl}
	docket.Test(t,
		dhcpConnectivity{docket},
		dhcpServer{docket},
		dhcpClient{docket},
		dhcpConnectivity2{docket},
		dhcpVlanTag{docket})
}

type dhcpConnectivity struct{ *docker.Docket }

func (dhcpConnectivity) String() string { return "connectivity" }

func (dhcp dhcpConnectivity) Test(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		host   string
		target string
	}{
		{"R1", "192.168.120.10"},
		{"R2", "192.168.120.5"},
	} {
		assert.Nil(dhcp.PingCmd(t, x.host, x.target))
		//FIXME
		//assert.Program(*Goes, "vnet", "show", "ip", "fib", "table",
		//	x.host)
	}
}

type dhcpServer struct{ *docker.Docket }

func (dhcpServer) String() string { return "server" }

func (dhcp dhcpServer) Test(t *testing.T) {
	assert := test.Assert{t}

	assert.Comment("Checking dhcp server on", "R2")
	time.Sleep(1 * time.Second)
	out, err := dhcp.ExecCmd(t, "R2", "ps", "ax")
	assert.Nil(err)
	//assert.Match(out, ".*dhcpd.*")
	timeout := 5
	found := false
	for i := timeout; i > 0; i-- {
		if !assert.MatchNonFatal(out, ".*dhcpd.*") {
			if *test.VV {
				fmt.Printf("check R2 ps ax, no match on dhcpd, %v retries left\n", i-1)
				fmt.Printf("%v\n", out)
			}
			time.Sleep(2 * time.Second)
			out, err = dhcp.ExecCmd(t, "R2", "ps", "ax")
			continue
		}
		found = true
	}
	if !found {
		test.Pause.Prompt("dhcpd not found")
		assert.Nil(fmt.Errorf("check dhcpd failed\n"))
	}
}

type dhcpClient struct{ *docker.Docket }

func (dhcpClient) String() string { return "client" }

func (dhcp dhcpClient) Test(t *testing.T) {
	assert := test.Assert{t}

	r, err := docker.FindHost(dhcp.Config, "R1")
	intf := r.Intfs[0]

	// remove existing IP address
	_, err = dhcp.ExecCmd(t, "R1",
		"ip", "address", "delete", "192.168.120.5", "dev", intf.Name)
	assert.Nil(err)

	assert.Comment("Verify ping fails")
	_, err = dhcp.ExecCmd(t, "R1", "ping", "-c1", "192.168.120.10")
	assert.NonNil(err)

	assert.Comment("Request dhcp address")
	out, err := dhcp.ExecCmd(t, "R1", "dhclient", "-4", "-v", intf.Name)
	assert.Nil(err)
	assert.Match(out, "bound to")
}

type dhcpConnectivity2 struct{ *docker.Docket }

func (dhcpConnectivity2) String() string { return "connectivity2" }

func (dhcp dhcpConnectivity2) Test(t *testing.T) {
	assert := test.Assert{t}

	assert.Comment("Check connectivity with dhcp address")
	assert.Nil(dhcp.PingCmd(t, "R1", "192.168.120.10"))
	//FIXME
	//assert.Program(*Goes, "vnet", "show", "ip", "fib", "table", "R1")
	//assert.Program(*Goes, "vnet", "show", "ip", "fib", "table", "R2")
}

type dhcpVlanTag struct{ *docker.Docket }

func (dhcpVlanTag) String() string { return "vlanTag" }

func (dhcp dhcpVlanTag) Test(t *testing.T) {
	assert := test.Assert{t}

	assert.Comment("Check for invalid vlan tag") // issue #92

	r1, err := docker.FindHost(dhcp.Config, "R1")
	r1Intf := r1.Intfs[0]

	// remove existing IP address
	_, err = dhcp.ExecCmd(t, "R1",
		"ip", "address", "flush", "dev", r1Intf.Name)
	assert.Nil(err)

	r2, err := docker.FindHost(dhcp.Config, "R2")
	r2Intf := r2.Intfs[0]

	done := make(chan bool, 1)

	go func(done chan bool) {
		out, err := dhcp.ExecCmd(t, "R2",
			"timeout", "10",
			"tcpdump", "-c1", "-nvvvei", r2Intf.Name, "port", "67")
		assert.Nil(err)
		match, err := regexp.MatchString("vlan 0", out)
		assert.Nil(err)
		if match {
			t.Error("Invalid vlan 0 tag found")
		}
		done <- true
	}(done)

	time.Sleep(1 * time.Second)
	_, err = dhcp.ExecCmd(t, "R1", "dhclient", "-4", "-v", r1Intf.Name)
	assert.Nil(err)
	<-done
}
