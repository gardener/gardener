// Copyright 2018 The Gardener Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetesbase

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"time"

	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

var podPath = []string{"api", "v1", "pods"}

// GetPod will return the Pod object for the given <name> in the given <namespace>.
func (c *Client) GetPod(namespace, name string) (*corev1.Pod, error) {
	return c.
		Clientset.
		CoreV1().
		Pods(namespace).
		Get(name, metav1.GetOptions{})
}

// ListPods will list all the Pods in the given <namespace> for the given <listOptions>.
func (c *Client) ListPods(namespace string, listOptions metav1.ListOptions) (*corev1.PodList, error) {
	pods, err := c.
		Clientset.
		CoreV1().
		Pods(namespace).
		List(listOptions)
	if err != nil {
		return nil, err
	}
	sort.Slice(pods.Items, func(i, j int) bool {
		return pods.Items[i].ObjectMeta.CreationTimestamp.Before(&pods.Items[j].ObjectMeta.CreationTimestamp)
	})
	return pods, nil
}

// GetPodLogs will get the logs of all containers within the Pod for the given <name> in the given <namespace>
// for the given <podLogOptions>.
func (c *Client) GetPodLogs(namespace, name string, podLogOptions *corev1.PodLogOptions) (*bytes.Buffer, error) {
	request := c.
		Clientset.
		CoreV1().
		Pods(namespace).
		GetLogs(name, podLogOptions)

	stream, err := request.Stream()
	if err != nil {
		return nil, err
	}

	defer stream.Close()
	buffer := bytes.NewBuffer(nil)
	_, err = io.Copy(buffer, stream)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

// ForwardPodPort tries to forward the <remote> port of the pod with name <name> in namespace <namespace> to
// the <local> port. If <local> equals zero, a free port will be chosen randomly.
// It returns the stop channel which must be closed when the port forward connection should be terminated.
func (c *Client) ForwardPodPort(namespace, name string, local, remote int) (chan struct{}, error) {
	fw, stopChan, err := c.setupForwardPodPort(namespace, name, local, remote)
	if err != nil {
		return nil, err
	}
	return stopChan, fw.ForwardPorts()
}

// CheckForwardPodPort tries to forward the <remote> port of the pod with name <name> in namespace <namespace> to
// the <local> port. If <local> equals zero, a free port will be chosen randomly.
// It returns true if the port forward connection has been established successfully or false otherwise.
func (c *Client) CheckForwardPodPort(namespace, name string, local, remote int) (bool, error) {
	fw, stopChan, err := c.setupForwardPodPort(namespace, name, local, remote)
	if err != nil {
		return false, err
	}

	errChan := make(chan error)
	go func() {
		errChan <- fw.ForwardPorts()
	}()
	defer close(stopChan)

	select {
	case err = <-errChan:
		return false, fmt.Errorf("forwarding ports: %v", err)
	case <-fw.Ready:
		return true, nil
	case <-time.After(time.Second * 5):
		return false, errors.New("port forward connection could not be established within five seconds")
	}
}

// DeletePod will delete a Pod with the given <name> in the given <namespace>.
func (c *Client) DeletePod(namespace, name string) error {
	return c.
		Clientset.
		CoreV1().
		Pods(namespace).
		Delete(name, &defaultDeleteOptions)
}

// CleanupPods deletes all the Pods in the cluster other than those stored in the
// exceptions map <exceptions>.
func (c *Client) CleanupPods(exceptions map[string]bool) error {
	return c.CleanupResource(exceptions, true, podPath...)
}

// CheckPodCleanup will check whether all the Pods in the cluster other than those
// stored in the exceptions map <exceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (c *Client) CheckPodCleanup(exceptions map[string]bool) (bool, error) {
	return c.CheckResourceCleanup(exceptions, true, podPath...)
}

func (c *Client) setupForwardPodPort(namespace, name string, local, remote int) (*portforward.PortForwarder, chan struct{}, error) {
	var (
		stopChan  = make(chan struct{}, 1)
		readyChan = make(chan struct{}, 1)
		out       = ioutil.Discard
		localPort int
	)

	u := c.
		Clientset.
		Core().
		RESTClient().
		Post().
		Resource("pods").
		Namespace(namespace).
		Name(name).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(c.Config)
	if err != nil {
		return nil, nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", u)

	if local == 0 {
		localPort, err = utils.FindFreePort()
		if err != nil {
			return nil, nil, err
		}
	}

	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, remote)}, stopChan, readyChan, out, out)
	if err != nil {
		return nil, nil, err
	}
	return fw, stopChan, nil
}
