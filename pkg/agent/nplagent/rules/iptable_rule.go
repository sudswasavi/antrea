// Copyright 2020 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rules

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/coreos/go-iptables/iptables"
	"k8s.io/klog"
)

type IPTableRule struct {
	name  string
	table *iptables.IPTables
}

var once sync.Once

func (ipt *IPTableRule) Init() (bool, error) {
	_, err := iptables.New()
	if err != nil {
		klog.Infof("init failed: %v\n", err)
		return false, errors.New("iptable init failed")
	}
	return true, nil
}

func (ipt *IPTableRule) CreateChains() { //userportstart int, userportend int) {
	exists, _ := ipt.table.Exists("filter", "NODE-PORT-LOCAL")
	if !exists {
		ipt.table.NewChain("filter", "NODE-PORT-LOCAL")
	}
	exists, _ = ipt.table.Exists("nat", "NODE-PORT-LOCAL")
	if !exists {
		ipt.table.NewChain("nat", "NODE-PORT-LOCAL")
	}

	exists, _ = ipt.table.Exists("filter", "FORWARD", "-j", "NODE-PORT-LOCAL")
	if !exists {
		ipt.table.Append("filter", "FORWARD", "-j", "NODE-PORT-LOCAL")
	}

	exists, _ = ipt.table.Exists("filter", "INPUT", "-p", "tcp", "-j", "NODE-PORT-LOCAL")
	if !exists {
		ipt.table.Append("filter", "INPUT", "-p", "tcp", "-j", "NODE-PORT-LOCAL")
	}

	exists, _ = ipt.table.Exists("nat", "PREROUTING", "-p", "tcp", "-j", "NODE-PORT-LOCAL")
	if !exists {
		ipt.table.Append("nat", "PREROUTING", "-p", "tcp", "-j", "NODE-PORT-LOCAL")
	}
}

func (ipt *IPTableRule) AddRule(port int, podip string) (bool, error) {
	//iptables -t nat -A NODE-PORT-LOCAL -p tcp -m tcp --dport 12000 -j DNAT --to-destination 10.244.0.8:80
	/*once.Do(func() {
		ipt.CreateChains()
	})*/
	exists, _ := ipt.table.Exists("nat", "NODE-PORT-LOCAL", "-p", "tcp", "-m", "tcp", "--dport",
		fmt.Sprint(port), "-j", "DNAT", "--to-destination", podip)
	if !exists {
		err := ipt.table.Append("nat", "NODE-PORT-LOCAL", "-p", "tcp", "-m", "tcp", "--dport",
			fmt.Sprint(port), "-j", "DNAT", "--to-destination", podip)

		if err != nil {
			fmt.Printf("%v", err)
			return false, err
		}
	}

	return true, nil
}

func (ipt *IPTableRule) DeleteRule(port int, podip string) (bool, error) {
	klog.Infof("Deleting rule with port %v and podip %v", port, podip)
	err := ipt.table.Delete("nat", "NODE-PORT-LOCAL", "-p", "tcp", "-m", "tcp", "--dport",
		fmt.Sprint(port), "-j", "DNAT", "--to-destination", podip)

	if err != nil {
		klog.Infof("%v", err)
		return false, err
	}
	return true, nil
}

func (ipt *IPTableRule) SyncState(podPort map[int]string) (bool, error) {

	m := make(map[int]string)
	var success = false
	for port, node := range podPort {
		success, _ := ipt.AddRule(port, node)
		if success == false {
			m[port] = node
			klog.Infof("Adding iptables failed for port %d and node %s", port, node)
			success = false
			continue
		}
	}
	podPort = m
	return success, nil
}

func (ipt *IPTableRule) GetAllRules(podPort map[int]string) (bool, error) {
	rules, _ := ipt.table.List("nat", "NODE-PORT-LOCAL")
	m := make(map[int]string)
	for i := range rules {
		split_rule := strings.Fields(rules[i])
		if len(split_rule) < 11 {
			continue
		}
		port, err := strconv.Atoi(split_rule[7])
		if err != nil {
			// handle error
			fmt.Println(err)
			continue
		}
		nodeip_port := strings.Split(split_rule[11], ":")
		if len(nodeip_port) != 2 {
			continue
		}
		//TODO: Need to check whether it's a proper ip:port combination
		m[port] = split_rule[11]
	}
	podPort = m
	return true, nil
}

func (ipt *IPTableRule) DeleteAllRules() (bool, error) {
	ipt.table.Delete("nat", "PREROUTING", "-p", "tcp", "-j", "NODE-PORT-LOCAL")
	ipt.table.Delete("filter", "FORWARD", "-j", "NODE-PORT-LOCAL")
	ipt.table.Delete("filter", "INPUT", "-p", "tcp", "-j", "NODE-PORT-LOCAL")
	ipt.table.ClearChain("nat", "NODE-PORT-LOCAL")
	ipt.table.DeleteChain("nat", "NODE-PORT-LOCAL")
	ipt.table.ClearChain("filter", "NODE-PORT-LOCAL")
	ipt.table.DeleteChain("filter", "NODE-PORT-LOCAL")
	ipt.table.ClearChain("input", "NODE-PORT-LOCAL")
	ipt.table.DeleteChain("input", "NODE-PORT-LOCAL")
	return true, nil
}

func (ipt *IPTableRule) Show() {
	chains, err := ipt.table.ListChains("filter")
	if err != nil {
		klog.Infof("ListChains of Initial failed: %v\n", err)
	}
	klog.Infof("chains: %v\n", chains)
}
