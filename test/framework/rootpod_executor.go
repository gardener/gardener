// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// RootPodExecutor enables the execution of command on the operating system of a node.
// The executor deploys a pod with root privileged on a specified node.
// This pod is then used to execute commands on the host operating system.
type RootPodExecutor interface {
	Execute(ctx context.Context, command ...string) ([]byte, error)
	Clean(ctx context.Context) error
}

// rootPodExecutor is the RootPodExecutor implementation
type rootPodExecutor struct {
	log    logr.Logger
	client kubernetes.Interface

	nodeName  *string
	namespace string

	Pod *corev1.Pod
}

// NewRootPodExecutor creates a new root pod executor to run commands on a node.
func NewRootPodExecutor(log logr.Logger, c kubernetes.Interface, nodeName *string, namespace string) RootPodExecutor {
	return &rootPodExecutor{
		log:       log,
		client:    c,
		nodeName:  nodeName,
		namespace: namespace,
	}
}

// Clean delete the deployed pod
func (e *rootPodExecutor) Clean(ctx context.Context) error {
	if e.Pod == nil {
		return nil
	}

	return DeleteAndWaitForResource(ctx, e.client, e.Pod, 2*time.Minute)
}

// Execute executes a command on the node the root pod is running
func (e *rootPodExecutor) Execute(ctx context.Context, command ...string) ([]byte, error) {
	isRunning, err := e.checkPodRunning(ctx)
	if err != nil {
		return nil, err
	}
	if !isRunning {
		if err := e.deploy(ctx); err != nil {
			return nil, err
		}
	}

	stdout, stderr, err := e.client.PodExecutor().Execute(ctx, e.Pod.Namespace, e.Pod.Name, e.Pod.Spec.Containers[0].Name,
		append([]string{"chroot", "/hostroot"}, command...)...,
	)

	var stderrBytes, stdoutBytes []byte
	if stderr != nil {
		var err2 error
		stderrBytes, err2 = io.ReadAll(stderr)
		if err != nil {
			return nil, err2
		}
	}
	if stdout != nil {
		var err2 error
		stdoutBytes, err2 = io.ReadAll(stdout)
		if err != nil {
			return nil, err2
		}
	}

	if err != nil || len(stderrBytes) > 0 {
		return stderrBytes, err
	}

	if len(stdoutBytes) > 0 {
		return stdoutBytes, nil
	}

	return nil, fmt.Errorf("no output from command execution")
}

// deploy deploys a root pod on the specified node and waits until it is running
func (e *rootPodExecutor) deploy(ctx context.Context) error {
	rootPod, err := DeployRootPod(ctx, e.client.Client(), e.namespace, e.nodeName)
	if err != nil {
		return err
	}
	if err := WaitUntilPodIsRunning(ctx, e.log, rootPod.Name, rootPod.Namespace, e.client); err != nil {
		return err
	}

	e.Pod = rootPod
	return nil
}

// checkPodRunning checks if the root pod is still running.
func (e *rootPodExecutor) checkPodRunning(ctx context.Context) (bool, error) {
	if e.Pod == nil {
		return false, nil
	}

	pod := e.Pod.DeepCopy()
	if err := e.client.Client().Get(ctx, client.ObjectKeyFromObject(e.Pod), pod); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
