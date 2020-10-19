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

package k8s

import (
	"fmt"

	utils "github.com/vmware-tanzu/antrea/pkg/agent/nplagent/lib"
	"github.com/vmware-tanzu/antrea/pkg/agent/nplagent/portcache"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

var portTable *portcache.PortTable

// POD HANDLERS
// HandleAddPod handles pod annotations for an added pod
func (c *Controller) HandleAddPod(pod *corev1.Pod) {
	portTable = portcache.GetPortTable()
	//klog.Infof("pod: %s ADD", pod.Name)
	podIP, nodeIP := pod.Status.PodIP, pod.Status.HostIP

	if podIP == "" || nodeIP == "" {
		return
	}
	podContainers := pod.Spec.Containers

	for _, container := range podContainers {
		for _, cport := range container.Ports {
			port := fmt.Sprint(cport.ContainerPort)
			nodePort, _ := portTable.AddRule(podIP, int(cport.ContainerPort))
			assignPodAnnotation(pod, port, nodeIP, fmt.Sprint(nodePort))
		}
	}

	c.updatePodAnnotation(pod)
}

// HandleDeletePod handles pod annotations for a deleted pod
func (c *Controller) HandleDeletePod(pod *corev1.Pod) {
	portTable = portcache.GetPortTable()
	klog.Infof("pod: %s DELETE", pod.Name)
	podIP := pod.Status.PodIP
	for _, container := range pod.Spec.Containers {
		for _, cport := range container.Ports {
			portTable.DeleteRule(podIP, int(cport.ContainerPort))
		}
	}
}

// HandleUpdatePod handles pod annotations for a updated pod
func (c *Controller) HandleUpdatePod(old, newp *corev1.Pod) {
	portTable = portcache.GetPortTable()
	klog.Infof("pod: %s UPDATE", newp.Name)
	podIP := newp.Status.PodIP

	// if the namespace of the pod has changed and has gone out of our scope, we need to delete it
	if old.Namespace != newp.Namespace {
		c.HandleDeletePod(newp)
		return
	}

	var newPodPorts []string
	newPodContainers := newp.Spec.Containers
	for _, container := range newPodContainers {
		for _, cport := range container.Ports {
			port := fmt.Sprint(cport.ContainerPort)
			newPodPorts = append(newPodPorts, port)
			if !portTable.RuleExists(podIP, int(cport.ContainerPort)) {
				c.HandleAddPod(newp)
			}
		}
	}

	// oldPodPorts: [8080, 8081] newPodPorts: [8082, 8081] portsToRemove should have: [8080]
	oldPodContainers := old.Spec.Containers
	for _, container := range oldPodContainers {
		for _, cport := range container.Ports {
			port := fmt.Sprint(cport.ContainerPort)
			if !utils.HasElem(newPodPorts, port) {
				// removed port
				nodePort := getFromPodAnnotation(newp, port)
				if nodePort == "" {
					break
				}

				portTable.DeleteRule(podIP, int(cport.ContainerPort))
				removeFromPodAnnotation(newp, port)
			}
		}
	}
	c.updatePodAnnotation(newp)
}

func InPodArray(arr []corev1.Pod, elem corev1.Pod) bool {
	for _, pod := range arr {
		if pod.Name == elem.Name {
			return true
		}
	}
	return false
}
