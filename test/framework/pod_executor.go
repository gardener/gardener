// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"k8s.io/client-go/tools/remotecommand"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// NewPodExecutor returns a podExecutor
func NewPodExecutor(client kubernetes.Interface) PodExecutor {
	return &podExecutor{
		client: client,
	}
}

// PodExecutor is the pod executor interface
type PodExecutor interface {
	Execute(ctx context.Context, namespace, name, containerName, command string) (io.Reader, error)
}

type podExecutor struct {
	client kubernetes.Interface
}

// Execute executes a command on a pod
func (p *podExecutor) Execute(ctx context.Context, namespace, name, containerName, command string) (io.Reader, error) {
	var stdout, stderr bytes.Buffer
	request := p.client.Kubernetes().CoreV1().RESTClient().
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
		Param("tty", "false")

	executor, err := remotecommand.NewSPDYExecutor(p.client.RESTConfig(), http.MethodPost, request.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to initialized the command exector: %v", err)
	}

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
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
