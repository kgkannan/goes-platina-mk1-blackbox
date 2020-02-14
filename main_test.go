// Copyright © 2015-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/platinasystems/test"
	"github.com/platinasystems/test/ethtool"
	"github.com/platinasystems/test/netport"
)

func TestMain(m *testing.M) {
	var ecode int
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, r)
			ecode = 1
		}
		if *XethStat {
			showXethStats()
		}
		/* FIXME_1.3; is this legit build error with go.mod
		 * pointing to local /test; Pause() is no longer method
		 * but a type struct; should this be test.Pause.xxx?
		 */
		if ecode != 0 {
			test.Pause.Flag()
			os.Exit(ecode)
		}
	}()
	assertFlags()
	if *test.DryRun {
		m.Run()
		return
	}
	if os.Geteuid() != 0 {
		panic("you aren't root")
	}
	if b, err := ioutil.ReadFile("/proc/net/unix"); err == nil {
		for _, atsock := range []string{
			"@xeth",
		} {
			if bytes.Index(b, []byte(atsock)) < 0 {
				panic(fmt.Errorf("no %s, are modules loaded?",
					atsock))
			}
		}
		for _, atsock := range []string{
			"@redisd",
			"@redis.reg",
			"@redis.pub",
		} {
			if bytes.Index(b, []byte(atsock)) < 0 {
				panic(fmt.Errorf("no %s, is goes  running?",
					atsock))
			}
		}
		var vnetproc = [...]string{
			"@vnet",
			"@vnetd",
			"@vnet-platina-mk1",
		}
		vnetActive := 0
		for _, atsock := range vnetproc {
			if bytes.Index(b, []byte(atsock)) >= 0 {
				vnetActive += 1
			}
		}
		if vnetActive < 2 {
			panic(fmt.Errorf("need two of %+v, is goes running?",
				vnetproc))
		}
	}
	netport.Init(*Goes)
	ethtool.Init()
	if testing.Verbose() {
		uutInfo()
	}
	test.IsHighVer = IsHigherVer()
	ecode = m.Run()
}

func Test(t *testing.T) {
	mayRun(t, "net", func(t *testing.T) {
		mayRun(t, "ping", pingNetTest)
		mayRun(t, "dhcp", dhcpNetTest)
		mayRun(t, "static", staticNetTest)
		mayRun(t, "gobgp", gobgpNetTest)
		mayRun(t, "bird", birdNetTest)
		mayRun(t, "frr", frrNetTest)
		test.SkipIfDryRun(t)
	})
	mayRun(t, "vlan", func(t *testing.T) {
		mayRun(t, "ping", pingVlanTest)
		mayRun(t, "dhcp", dhcpVlanTest)
		mayRun(t, "slice", sliceVlanTest)
		mayRun(t, "static", staticVlanTest)
		mayRun(t, "gobgp", gobgpVlanTest)
		mayRun(t, "bird", birdVlanTest)
		mayRun(t, "frr", frrVlanTest)
		test.SkipIfDryRun(t)
	})
	mayRun(t, "bridge", func(t *testing.T) {
		mayRun(t, "ping", pingBridgeTest)
		test.SkipIfDryRun(t)
	})
	mayRun(t, "nsif", nsifTest)
	mayRun(t, "multipath", mpTest)
	//TODOIP6: temp placeholder for ip6 tests
	mayRun(t, "net6", func(t *testing.T) {
		mayRun(t, "ping", pingIp6NetTest)
	})
	mayRun(t, "nsifIp6", nsifIp6Test)
	mayRun(t, "multipathIp6", mpIp6Test)
	test.SkipIfDryRun(t)
}

func mayRun(t *testing.T, name string, f func(*testing.T)) bool {
	var ret bool
	t.Helper()
	if !t.Failed() {
		ret = t.Run(name, f)
	}
	return ret
}

//set environment info based on platform software
func IsHigherVer() (higher bool) {
	pd := *PlatformDriver
	o, err := exec.Command("/sbin/modinfo", pd).Output()
	if err == nil && len(o) > 0 {
		const depends = "depends:"
		s := string(o)
		i := strings.Index(s, depends)
		if i > 0 {
			s = s[i+len(depends):]
			dependencies := 0
			drvdepends := [...]string{
				"onie",
				"xeth",
			}
			for _, d := range drvdepends {
				if strings.Index(s, d) >= 0 {
					dependencies += 1
				}
			}
			if dependencies == 2 {
				higher = true
			}
		}
	}
	fmt.Printf("\n IsHigherVer %v \n", higher)
	return
}

func uutInfo() {
	fmt.Println("---")
	defer fmt.Println("...")
	o, err := exec.Command(*Goes, "show", "buildid").Output()
	if err == nil && len(o) > 0 {
		fmt.Print(*Goes, ": |\n    buildid/", string(o))
	}
	o, err = exec.Command(*Goes, "vnetd", "-path").Output()
	if err == nil && len(o) > 0 {
		vnet := string(o[:len(o)-1])
		o, err = exec.Command(*Goes, "show", "buildid", vnet).Output()
		if err == nil && len(o) > 0 {
			fmt.Print(vnet, ": |\n    buildid/", string(o))
		}
	}
	pd := *PlatformDriver
	ko := pd
	if !strings.HasSuffix(ko, ".ko") {
		ko += ".ko"
	}
	if _, err = os.Stat(ko); err == nil {
		pd = ko
	}
	o, err = exec.Command("/sbin/modinfo", pd).Output()
	if err == nil && len(o) > 0 {
		const srcversion = "srcversion:"
		s := string(o)
		i := strings.Index(s, srcversion)
		if i > 0 {
			s = s[i+len(srcversion):]
			i = strings.Index(s, "\n")
			fmt.Print(pd, ": |\n    ",
				strings.TrimLeft(s[:i+1], " \t"))
		}
	}
}

func showXethStats() {
	const dn = "/sys/kernel/platina-mk1/xeth"
	fmt.Println("---")
	defer fmt.Println("...")
	fis, err := ioutil.ReadDir(dn)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, fi := range fis {
		bn := fi.Name()
		b, err := ioutil.ReadFile(filepath.Join(dn, bn))
		if err != nil {
			fmt.Print(bn, ": ", err, "\n")
		} else if s := string(b); s != "0\n" {
			fmt.Print(bn, ": ", s)
		}
	}
}
