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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// Executor allows for execution of commands in containers.
type Executor interface {
	ExecCommandInContainerWithFullOutput(ctx context.Context, namespace, podName, containerName string, cmd ...string) (stdout, stderr string, err error)
}

// execOptions passed to ExecWithOptions
type execOptions struct {
	client        kubernetes.Interface
	command       []string
	namespace     string
	podName       string
	containerName string
	stdin         io.Reader
	captureStdout bool
	captureStderr bool
	// If false, whitespace in std{err,out} will be removed.
	preserveWhitespace bool
}

type defaultExecutor struct {
	client kubernetes.Interface
}

// NewExecutor creates a new instance of Executor for a specific
// Kubernetes client.
func NewExecutor(client kubernetes.Interface) Executor {
	return &defaultExecutor{client: client}
}

// ExecCommandInContainerWithFullOutput executes a command in the
// specified container and return stdout, stderr and error
func (e *defaultExecutor) ExecCommandInContainerWithFullOutput(ctx context.Context, namespace, podName, containerName string, cmd ...string) (stdout, stderr string, err error) {
	return execWithOptions(execOptions{
		client:             e.client,
		command:            cmd,
		namespace:          namespace,
		podName:            podName,
		containerName:      containerName,
		stdin:              nil,
		captureStdout:      true,
		captureStderr:      true,
		preserveWhitespace: false,
	})
}

// execWithOptions executes a command in the specified container,
// returning stdout, stderr and error. `options` allowed for
// additional parameters to be passed.
func execWithOptions(options execOptions) (stdout, stderr string, err error) {
	const tty = false
	req := options.client.Kubernetes().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(options.podName).
		Namespace(options.namespace).
		SubResource("exec").
		Param("container", options.containerName)
	req.VersionedParams(&corev1.PodExecOptions{
		Container: options.containerName,
		Command:   options.command,
		Stdin:     options.stdin != nil,
		Stdout:    options.captureStdout,
		Stderr:    options.captureStderr,
		TTY:       tty,
	}, scheme.ParameterCodec)

	var stdoutBuff, stderrBuff bytes.Buffer
	err = execute("POST", req.URL(), options.client.RESTConfig(), options.stdin, &stdoutBuff, &stderrBuff, tty)

	if options.preserveWhitespace {
		return stdoutBuff.String(), stderrBuff.String(), err
	}
	return strings.TrimSpace(stdoutBuff.String()), strings.TrimSpace(stderrBuff.String()), err
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
