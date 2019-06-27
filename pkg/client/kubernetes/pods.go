// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
)

// NewPodExecutor returns a podExecutor
func NewPodExecutor(config *rest.Config) PodExecutor {
	return &podExecutor{
		config: config,
	}
}

// PodExecutor is the pod executor interface
type PodExecutor interface {
	Execute(ctx context.Context, namespace, name, containerName, command string) (io.Reader, error)
}

type podExecutor struct {
	config *rest.Config
}

// Execute executes a command on a pod
func (p *podExecutor) Execute(ctx context.Context, namespace, name, containerName, command string) (io.Reader, error) {
	client, err := corev1client.NewForConfig(p.config)
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	request := client.RESTClient().
		Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec").
		Param("container", containerName).
		Param("command", "/bin/sh").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false").
		Context(ctx)

	executor, err := remotecommand.NewSPDYExecutor(p.config, http.MethodPost, request.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to initialized the command exector: %v", err)
	}

	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  strings.NewReader(command),
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return &stderr, err
	}

	return &stdout, nil
}

// GetPodLogs retrieves the pod logs of the pod of the given name with the given options.
func GetPodLogs(podInterface corev1client.PodInterface, name string, options *corev1.PodLogOptions) ([]byte, error) {
	request := podInterface.GetLogs(name, options)

	stream, err := request.Stream()
	if err != nil {
		return nil, err
	}
	defer func() { utilruntime.HandleError(stream.Close()) }()

	return ioutil.ReadAll(stream)
}

// ForwardPodPort tries to forward the <remote> port of the pod with name <name> in namespace <namespace> to
// the <local> port. If <local> equals zero, a free port will be chosen randomly.
// It returns the stop channel which must be closed when the port forward connection should be terminated.
func (c *Clientset) ForwardPodPort(namespace, name string, local, remote int) (chan struct{}, error) {
	fw, stopChan, err := c.setupForwardPodPort(namespace, name, local, remote)
	if err != nil {
		return nil, err
	}
	return stopChan, fw.ForwardPorts()
}

// CheckForwardPodPort tries to forward the <remote> port of the pod with name <name> in namespace <namespace> to
// the <local> port. If <local> equals zero, a free port will be chosen randomly.
// It returns true if the port forward connection has been established successfully or false otherwise.
func (c *Clientset) CheckForwardPodPort(namespace, name string, local, remote int) error {
	fw, stopChan, err := c.setupForwardPodPort(namespace, name, local, remote)
	if err != nil {
		return fmt.Errorf("could not setup pod port forwarding: %v", err)
	}

	errChan := make(chan error)
	go func() {
		errChan <- fw.ForwardPorts()
	}()
	defer close(stopChan)

	select {
	case err = <-errChan:
		return fmt.Errorf("error forwarding ports: %v", err)
	case <-fw.Ready:
		return nil
	case <-time.After(time.Second * 5):
		return errors.New("port forward connection could not be established within five seconds")
	}
}

func (c *Clientset) setupForwardPodPort(namespace, name string, local, remote int) (*portforward.PortForwarder, chan struct{}, error) {
	var (
		stopChan  = make(chan struct{}, 1)
		readyChan = make(chan struct{}, 1)
		out       = ioutil.Discard
		localPort int
	)

	u := c.kubernetes.CoreV1().RESTClient().Post().Resource("pods").Namespace(namespace).Name(name).SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(c.config)
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
