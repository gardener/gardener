// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (o *options) start(ctx context.Context) error {
	conf, err := ctrl.GetConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return err
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		0,
		informers.WithNamespace(o.namespace),
		informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.FieldSelector = fields.OneTermEqualSelector("metadata.name", o.deploymentName).String()
			lo.Limit = 1
		}))

	c := &controller{
		opts:        o,
		actualCount: 0,
		queue:       workqueue.New(),
		getter:      factory.Apps().V1().Deployments().Lister().Deployments(o.namespace).Get,
	}

	inf := factory.Apps().V1().Deployments().Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(_ interface{}) {
			klog.V(6).Info("Add event")

			c.queue.Add(o.deploymentName)
		},
		UpdateFunc: func(_, _ interface{}) {
			klog.V(6).Info("Update event")

			c.queue.Add(o.deploymentName)
		},
	})

	klog.V(1).Info("Starting cache...")
	factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), inf.HasSynced) {
		return errors.New("waiting for caches failed")
	}

	klog.V(1).Info("Caches are synced")

	go c.start(ctx)

	<-ctx.Done()
	c.queue.ShutDown()

	klog.V(1).Info("Shutdown called. Bye...")

	return nil
}

type options struct {
	command        []string
	namespace      string
	deploymentName string
	jitter         time.Duration
	jitterFactor   float64
}

type controller struct {
	opts *options
	sync.Mutex
	actualCount int32
	lastCommand *exec.Cmd
	queue       *workqueue.Type
	getter      func(name string) (*v1.Deployment, error)
}

func (c *controller) start(ctx context.Context) {
	wait.UntilWithContext(ctx, c.runWorker, time.Second)
}

func (c *controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		klog.V(2).Info("Queue is shutdown")

		return false
	}

	defer c.queue.Done(obj)

	if err := c.reconcile(ctx); err != nil {
		return false
	}

	return false
}

func (c *controller) reconcile(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	klog.V(4).InfoS("Reconciling deployment", "name", c.opts.deploymentName, "namespace", c.opts.namespace)

	dep, err := c.getter(c.opts.deploymentName)
	if err != nil {
		return err
	}

	if *dep.Spec.Replicas == c.actualCount {
		klog.V(4).InfoS("Servercount is the same. Skipping...", "oldReplica", c.actualCount)

		return nil
	}

	if c.lastCommand != nil {
		jitter := wait.Jitter(c.opts.jitter, c.opts.jitterFactor)

		klog.V(2).InfoS(
			"Deployment replica count changed. Restarting server.",
			"oldReplica", c.actualCount,
			"newReplica", *dep.Spec.Replicas,
			"jitter", jitter,
		)

		if jitter > 0 {
			time.Sleep(jitter)
		}

		if err := c.lastCommand.Process.Signal(os.Interrupt); err != nil {
			return err
		}

		ps, err := c.lastCommand.Process.Wait()
		if err != nil {
			return err
		}

		klog.V(4).InfoS("Previous command exited", "existed", ps.Exited(), "code", ps.ExitCode())

	}

	fullCmd := append(c.opts.command, strconv.FormatInt(int64(*dep.Spec.Replicas), 10))
	c.actualCount = *dep.Spec.Replicas

	klog.V(4).InfoS("Starting command", "command", fullCmd[0], "args", fullCmd[1:])

	c.lastCommand = exec.CommandContext(ctx, fullCmd[0], fullCmd[1:]...)

	c.lastCommand.Stdout = os.Stdout
	c.lastCommand.Stderr = os.Stderr

	return c.lastCommand.Start()
}
