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

package portcache

import (
	"fmt"
	"sync"

	utils "github.com/vmware-tanzu/antrea/pkg/agent/nplagent/lib"
	"github.com/vmware-tanzu/antrea/pkg/agent/nplagent/rules"
)

const (
	successStatus = 0
	pendingStatus = 1
	failStatus    = 2
)

type NodePortData struct {
	nodeport int
	podport  int
	podip    string
	status   int
	//podname  string
}

type PortTable struct {
	table        map[int]NodePortData
	startPort    int
	endPort      int
	podPortRules rules.PodPortRules
	tableLock    sync.RWMutex
}

var once sync.Once
var ptable PortTable

func NewPortTable(start, end int) *PortTable {
	once.Do(func() {
		ptable = PortTable{startPort: start, endPort: end}
		ptable.table = make(map[int]NodePortData)
		ptable.podPortRules = rules.Initrules()
	})
	return &ptable
}

func GetPortTable() *PortTable {
	return &ptable
}

func (pt *PortTable) PopulatePortTable(r rules.PodPortRules) {
	portMap := make(map[int]string)
	ok, _ := r.GetAllRules(portMap)
	if !ok {
		return
	}
	table := make(map[int]NodePortData)
	for nodeport, podip := range portMap {
		entry := NodePortData{
			nodeport: nodeport,
			podip:    podip,
		}
		table[nodeport] = entry
	}
	pt.tableLock.Lock()
	defer pt.tableLock.Unlock()
	pt.table = table
}

func (pt *PortTable) AddUpdateEntry(nodeport, podport int, podip string) {
	pt.tableLock.Lock()
	defer pt.tableLock.Unlock()
	data := NodePortData{nodeport: nodeport, podport: podport, podip: podip}
	pt.table[nodeport] = data
}

func (pt *PortTable) DeleteEntry(nodeport int) {
	pt.tableLock.Lock()
	defer pt.tableLock.Unlock()
	delete(pt.table, nodeport)
}

func (pt *PortTable) DeleteEntryByPodIP(ip string) {
	pt.tableLock.Lock()
	defer pt.tableLock.Unlock()
	for i, data := range pt.table {
		if data.podip == ip {
			delete(pt.table, i)
		}
	}
}

func (pt *PortTable) DeleteEntryByPodIPPort(ip string, port int) {
	pt.tableLock.Lock()
	defer pt.tableLock.Unlock()
	for i, data := range pt.table {
		if data.podip == ip && data.podport == port {
			delete(pt.table, i)
		}
	}
}

func (pt *PortTable) GetEntry(nodeport int) *NodePortData {
	pt.tableLock.RLock()
	defer pt.tableLock.RUnlock()
	data, _ := pt.table[nodeport]
	return &data
}

func (pt *PortTable) GetEntryByPodIPPort(ip string, port int) *NodePortData {
	pt.tableLock.RLock()
	defer pt.tableLock.RUnlock()
	for _, data := range pt.table {
		if data.podip == ip && data.podport == port {
			return &data
		}
	}
	return nil
}

func (pt *PortTable) getFreePort() int {
	for i := pt.startPort; i <= pt.endPort; i++ {
		if _, ok := pt.table[i]; !ok && utils.IsPortAvailable(i) {
			return i
		}
	}
	return -1
}

func (pt *PortTable) AddRule(podip string, podport int) (int, bool) {
	nodeport := pt.getFreePort()
	if nodeport < 0 {
		return -1, false
	}
	if pt == nil {
		pt = NewPortTable(40000, 45000)
	}
	ok, _ := pt.podPortRules.AddRule(nodeport, fmt.Sprintf("%s:%d", podip, podport))
	if !ok {
		return -1, false
	}
	pt.AddUpdateEntry(nodeport, podport, podip)
	return nodeport, false
}

func (pt *PortTable) DeleteRule(podip string, podport int) (bool, error) {
	data := pt.GetEntryByPodIPPort(podip, podport)
	ok, err := pt.podPortRules.DeleteRule(data.nodeport, fmt.Sprintf("%s:%d", podip, podport))
	if !ok {
		return false, err
	}
	pt.DeleteEntry(data.nodeport)
	return true, nil
}

func (pt *PortTable) RuleExists(podip string, podport int) bool {
	data := pt.GetEntryByPodIPPort(podip, podport)
	if data != nil {
		return true
	}
	return false
}
