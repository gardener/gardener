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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
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
	Execute(ctx context.Context, namespace, name, containerName string, command ...string) (io.Reader, io.Reader, error)
	ExecuteWithStreams(ctx context.Context, namespace, name, containerName string, stdin io.Reader, stdout, stderr io.Writer, command ...string) error
}

type podExecutor struct {
	config *rest.Config
}

// ExecuteWithStreams executes a command on a pod with the given streams.
func (p *podExecutor) ExecuteWithStreams(ctx context.Context, namespace, name, containerName string, stdin io.Reader, stdout, stderr io.Writer, command ...string) error {
	client, err := corev1client.NewForConfig(p.config)
	if err != nil {
		return fmt.Errorf("failed creating corev1 client: %w", err)
	}

	request := client.RESTClient().
		Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec")
	request.VersionedParams(&corev1.PodExecOptions{
		Stdin:     stdin != nil,
		Stdout:    stdout != nil,
		Stderr:    stderr != nil,
		TTY:       false,
		Container: containerName,
		Command:   command,
	}, scheme.ParameterCodec)

	// Use a fallback executor with websocket as primary and spdy as fallback similar to kubectl.
	// https://github.com/kubernetes/kubectl/blob/2e38fc220409bbc92f8270c49612f0f9d8e36c89/pkg/cmd/exec/exec.go#L143-L155
	spdyExecutor, err := remotecommand.NewSPDYExecutor(p.config, http.MethodPost, request.URL())
	if err != nil {
		return fmt.Errorf("failed to initialize the spdy executor: %w", err)
	}

	websocketExecutor, err := remotecommand.NewWebSocketExecutor(p.config, http.MethodGet, request.URL().String())
	if err != nil {
		return fmt.Errorf("failed to initialize the websocket executor: %w", err)
	}

	executor, err := remotecommand.NewFallbackExecutor(websocketExecutor, spdyExecutor, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return fmt.Errorf("failed to initialize the command executor: %w", err)
	}

	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	}); err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	return nil
}

// Execute executes a command on a pod.
func (p *podExecutor) Execute(ctx context.Context, namespace, name, containerName string, command ...string) (io.Reader, io.Reader, error) {
	var stdout, stderr bytes.Buffer
	return &stdout, &stderr, p.ExecuteWithStreams(ctx, namespace, name, containerName, nil, &stdout, &stderr, command...)
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
