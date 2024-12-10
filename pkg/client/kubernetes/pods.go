// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"

	"github.com/gardener/gardener/pkg/utils"
)

// NewPodExecutor returns a podExecutor
func NewPodExecutor(config *rest.Config) PodExecutor {
	return &podExecutor{
		config: config,
	}
}

// PodExecutor is the pod executor interface
type PodExecutor interface {
	Execute(ctx context.Context, namespace, name, containerName, command, commandArg string) (io.Reader, error)
}

type podExecutor struct {
	config *rest.Config
}

// Execute executes a command on a pod
func (p *podExecutor) Execute(ctx context.Context, namespace, name, containerName, command, commandArg string) (io.Reader, error) {
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
		Param("command", command).
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false")

	executor, err := remotecommand.NewSPDYExecutor(p.config, http.MethodPost, request.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to initialized the command executor: %w", err)
	}

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  strings.NewReader(commandArg),
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
func GetPodLogs(ctx context.Context, podInterface corev1client.PodInterface, name string, options *corev1.PodLogOptions) ([]byte, error) {
	request := podInterface.GetLogs(name, options)

	stream, err := request.Stream(ctx)
	if err != nil {
		return nil, err
	}

	defer func() { utilruntime.HandleError(stream.Close()) }()

	return io.ReadAll(stream)
}

// CheckForwardPodPort tries to open a portForward connection with the passed PortForwarder.
// It returns nil if the port forward connection has been established successfully or an error otherwise.
func CheckForwardPodPort(fw PortForwarder) error {
	errChan := make(chan error, 1)
	go func() {
		errChan <- fw.ForwardPorts()
	}()

	select {
	case err := <-errChan:
		return fmt.Errorf("error forwarding ports: %w", err)
	case <-fw.Ready():
		return nil
	}
}

// PortForwarder knows how to forward a port connection
// Ready channel is expected to be closed once the connection becomes ready
type PortForwarder interface {
	ForwardPorts() error
	Ready() chan struct{}
}

// SetupPortForwarder sets up a PortForwarder which forwards the <remote> port of the pod with name <name> in namespace <namespace>
// to the <local> port. If <local> equals zero, a free port will be chosen randomly.
// When calling ForwardPorts on the returned PortForwarder, it will run until the given context is cancelled.
// Hence, the given context should carry a timeout and should be cancelled once the forwarding is no longer needed.
func SetupPortForwarder(ctx context.Context, config *rest.Config, namespace, name string, local, remote int) (PortForwarder, error) {
	var (
		readyChan = make(chan struct{}, 1)
		out       = io.Discard
		localPort int
	)

	client, err := corev1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	u := client.RESTClient().Post().Resource("pods").Namespace(namespace).Name(name).SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", u)

	if local == 0 {
		localPort, err = utils.FindFreePort()
		if err != nil {
			return nil, err
		}
	}

	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, remote)}, ctx.Done(), readyChan, out, out)
	if err != nil {
		return nil, err
	}
	return portForwarder{fw}, nil
}

type portForwarder struct {
	*portforward.PortForwarder
}

func (p portForwarder) Ready() chan struct{} {
	return p.PortForwarder.Ready
}
