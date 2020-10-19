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
	"context"
	"encoding/json"

	utils "github.com/vmware-tanzu/antrea/pkg/agent/nplagent/lib"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

type NPLEPAnnotation struct {
	PodPort  string `json:"podport"`
	NodeIP   string `json:"nodeip"`
	NodePort string `json:"nodeport"`
}

func IsNodePortInAnnotation(s []NPLEPAnnotation, nodeport string) bool {
	for _, i := range s {
		if i.NodePort == nodeport {
			return true
		}
	}
	return false
}

func assignPodAnnotation(pod *corev1.Pod, containerPort, nodeIP, nodePort string) {
	var err error
	current := make(map[string]string)
	if pod.Annotations != nil {
		current = pod.Annotations
	}

	klog.Infof("Building annotation for pod: %s\tport: %s --> %s:%s", pod.Name, containerPort, nodeIP, nodePort)

	// nplEP annotation exists
	var annotations []NPLEPAnnotation
	if current[utils.NPLEP_ANNOTATION] != "" {
		if err = json.Unmarshal([]byte(current[utils.NPLEP_ANNOTATION]), &annotations); err != nil {
			klog.Warningf("Unable to unmarshal NPLEP annotation")
		}

		if !IsNodePortInAnnotation(annotations, nodePort) {
			annotations = append(annotations, NPLEPAnnotation{
				PodPort:  containerPort,
				NodeIP:   nodeIP,
				NodePort: nodePort,
			})
		} else {
			// mapping for the containerPort already exists
			// TODO
		}
	} else {
		annotations = []NPLEPAnnotation{NPLEPAnnotation{
			PodPort:  containerPort,
			NodeIP:   nodeIP,
			NodePort: nodePort,
		}}
	}

	klog.Infof("pod: %s\tNPLEP annotation %+v", pod.Name, annotations)
	current[utils.NPLEP_ANNOTATION] = utils.Stringify(annotations)
	pod.Annotations = current
}

func removeFromPodAnnotation(pod *corev1.Pod, containerPort string) {
	var err error
	current := pod.Annotations

	klog.Infof("Removing annotation from pod: %s\tport: %s", pod.Name, containerPort)
	var annotations []NPLEPAnnotation
	if err = json.Unmarshal([]byte(current[utils.NPLEP_ANNOTATION]), &annotations); err != nil {
		klog.Warningf("Unable to unmarshal NPLEP annotation")
	}

	for i, ann := range annotations {
		if ann.PodPort == containerPort {
			annotations = append(annotations[:i], annotations[i+1:]...)
			break
		}
	}

	klog.Infof("pod: %s\tNPLEP annotation %+v", pod.Name, annotations)
	current[utils.NPLEP_ANNOTATION] = utils.Stringify(annotations)
	pod.Annotations = current
}

func (c *Controller) updatePodAnnotation(pod *corev1.Pod) error {

	if _, err := c.KubeClient.CoreV1().Pods(pod.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{}); err != nil {
		klog.Warningf("Unable to update pod %s with annotation: %+v", pod.Name, err)
		return err
	}

	klog.Infof("Successfuly updated pod %s annotation", pod.Name)
	return nil
}

// returns nodeport for podport
func getFromPodAnnotation(pod *corev1.Pod, port string) string {
	current := pod.Annotations

	var annotations []NPLEPAnnotation
	if err := json.Unmarshal([]byte(current[utils.NPLEP_ANNOTATION]), &annotations); err != nil {
		klog.Warningf("Unable to unmarshal NPLEP annotation")
	}

	for _, i := range annotations {
		if i.PodPort == port {
			return i.NodePort
		}
	}

	klog.Warningf("Corresponding nodeport for pod: %s port: %s NOTFOUND", pod.Name, port)
	return ""
}
