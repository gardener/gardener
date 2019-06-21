// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package executor

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// Executor allows for execution of commands in containers.
type Executor interface {
	ExecCommandInContainerWithFullOutput(ctx context.Context, namespace, podName, containerName string, cmd ...string) (stdout, stderr string, err error)
	ExecCommandInContainer(ctx context.Context, namespace, podName, containerName string, cmd ...string) (stdout string)
	ExecShellInContainer(ctx context.Context, namespace, podName, containerName string, cmd string) (stdout string)
}

// ExecOptions passed to ExecWithOptions
type ExecOptions struct {
	Client        kubernetes.Interface
	Command       []string
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         io.Reader
	CaptureStdout bool
	CaptureStderr bool
	// If false, whitespace in std{err,out} will be removed.
	PreserveWhitespace bool
}

type defaultExecutor struct {
	client kubernetes.Interface
}

// NewExecutor creates a new instance of Executor for a specific
// Kubernetes client.
func NewExecutor(client kubernetes.Interface) Executor {
	return &defaultExecutor{client: client}
}

// ExecWithOptions executes a command in the specified container,
// returning stdout, stderr and error. `options` allowed for
// additional parameters to be passed.
func ExecWithOptions(ctx context.Context, options ExecOptions) (stdout, stderr string, err error) {
	const tty = false
	req := options.Client.Kubernetes().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(options.PodName).
		Namespace(options.Namespace).
		SubResource("exec").
		Param("container", options.ContainerName).
		Context(ctx)
	req.VersionedParams(&v1.PodExecOptions{
		Container: options.ContainerName,
		Command:   options.Command,
		Stdin:     options.Stdin != nil,
		Stdout:    options.CaptureStdout,
		Stderr:    options.CaptureStderr,
		TTY:       tty,
	}, scheme.ParameterCodec)

	var stdoutBuff, stderrBuff bytes.Buffer
	err = execute("POST", req.URL(), options.Client.RESTConfig(), options.Stdin, &stdoutBuff, &stderrBuff, tty)

	if options.PreserveWhitespace {
		return stdoutBuff.String(), stderrBuff.String(), err
	}
	return strings.TrimSpace(stdoutBuff.String()), strings.TrimSpace(stderrBuff.String()), err
}

// ExecCommandInContainerWithFullOutput executes a command in the
// specified container and return stdout, stderr and error
func (e *defaultExecutor) ExecCommandInContainerWithFullOutput(ctx context.Context, namespace, podName, containerName string, cmd ...string) (stdout, stderr string, err error) {
	return ExecWithOptions(ctx, ExecOptions{
		Client:             e.client,
		Command:            cmd,
		Namespace:          namespace,
		PodName:            podName,
		ContainerName:      containerName,
		Stdin:              nil,
		CaptureStdout:      true,
		CaptureStderr:      true,
		PreserveWhitespace: false,
	})
}

// ExecCommandInContainer executes a command in the specified container.
// Command is expect to succeed.
func (e *defaultExecutor) ExecCommandInContainer(ctx context.Context, namespace, podName, containerName string, cmd ...string) (stdout string) {
	stdout, stderr, err := e.ExecCommandInContainerWithFullOutput(ctx, namespace, podName, containerName, cmd...)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), `failed to execute command %in pod "%s/%s", container %s: %s`, podName, containerName, stderr)
	return stdout
}

// ExecShellInContainer executes the specified command on the pod's container.
func (e *defaultExecutor) ExecShellInContainer(ctx context.Context, namespace, podName, containerName string, cmd string) (stdout string) {
	return e.ExecCommandInContainer(ctx, namespace, podName, containerName, "/bin/sh", "-c", cmd)
}

func execute(method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool) error {
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
}
