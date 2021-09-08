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

package networkpolicy

import (
	"context"
	"sync"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/helper"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/hostnameresolver"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller watching the endpoints resource "kubernetes" of the Seeds's kube-apiserver in the default namespace
// to keep the NetworkPolicy "allow-to-seed-apiserver" in sync.
type Controller struct {
	ctx                     context.Context
	log                     *logrus.Logger
	recorder                record.EventRecorder
	seedClient              client.Client
	namespaceReconciler     reconcile.Reconciler
	endpointsInformer       runtimecache.Informer
	networkPoliciesInformer runtimecache.Informer
	namespaceQueue          workqueue.RateLimitingInterface
	namespaceInformer       runtimecache.Informer
	hostnameProvider        hostnameresolver.Provider
	shootNamespaceSelector  labels.Selector
	workerCh                chan int
	numberOfRunningWorkers  int
	waitGroup               sync.WaitGroup
}

// NewController instantiates a new networkpolicy controller.
func NewController(ctx context.Context, seedClient kubernetes.Interface, logger *logrus.Logger, recorder record.EventRecorder, seedName string) (*Controller, error) {
	endpointsInformer, err := seedClient.Cache().GetInformer(ctx, &corev1.Endpoints{})
	if err != nil {
		return nil, err
	}
	networkPoliciesInformer, err := seedClient.Cache().GetInformer(ctx, &networkingv1.NetworkPolicy{})
	if err != nil {
		return nil, err
	}
	namespaceInformer, err := seedClient.Cache().GetInformer(ctx, &corev1.Namespace{})
	if err != nil {
		return nil, err
	}

	provider, err := hostnameresolver.CreateForCluster(seedClient, logger)
	if err != nil {
		return nil, err
	}

	shootNamespaceSelector := labels.SelectorFromSet(labels.Set{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
	})

	controller := &Controller{
		ctx: ctx,
		log: logger,
		namespaceReconciler: newNamespaceReconciler(
			logger,
			seedClient.Client(),
			seedName,
			shootNamespaceSelector,
			provider,
		),
		recorder:                recorder,
		seedClient:              seedClient.Client(),
		endpointsInformer:       endpointsInformer,
		namespaceInformer:       namespaceInformer,
		networkPoliciesInformer: networkPoliciesInformer,
		namespaceQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "namespace"),
		shootNamespaceSelector:  shootNamespaceSelector,
		workerCh:                make(chan int),
		hostnameProvider:        provider,
	}

	go controller.hostnameProvider.Start(ctx)

	return controller, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*2)
	defer cancel()

	if !cache.WaitForCacheSync(
		timeoutCtx.Done(),
		c.endpointsInformer.HasSynced,
		c.namespaceInformer.HasSynced,
		c.networkPoliciesInformer.HasSynced,
		c.hostnameProvider.HasSynced,
	) {
		c.log.Fatal("timeout waiting for caches to sync")
		return
	}

	c.hostnameProvider.WithCallback(func() {
		c.enqueueNamespaces()
	})

	c.endpointsInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			endpoints, ok := obj.(*corev1.Endpoints)
			if !ok {
				return false
			}
			return endpoints.Namespace == corev1.NamespaceDefault && endpoints.Name == "kubernetes"
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.endpointAdd,
			UpdateFunc: c.endpointUpdate,
			DeleteFunc: c.endpointDelete,
		},
	})

	c.networkPoliciesInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			policy, ok := obj.(*networkingv1.NetworkPolicy)
			if !ok {
				return false
			}
			return policy.Name == helper.AllowToSeedAPIServer
		},
		Handler: cache.ResourceEventHandlerFuncs{
			UpdateFunc: c.networkPolicyUpdate,
			DeleteFunc: c.networkPolicyDelete,
		},
	})

	c.namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.namespaceAdd,
		UpdateFunc: c.namespaceUpdate,
	})

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.namespaceQueue, "namespace", c.namespaceReconciler, &c.waitGroup, c.workerCh)
	}

	c.log.Info("Seed API server network policy controller initialized.")
}

// Stop the controller
func (c *Controller) Stop() {
	c.namespaceQueue.ShutDown()

	for {
		if c.namespaceQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.log.Debug("No running namespace workers and no items left in the queues. Terminating seed API server network policy controller...")
			break
		}
		c.log.Debugf("Waiting for %d endpoints worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.namespaceQueue.Len())
		time.Sleep(5 * time.Second)
	}

	c.waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *Controller) RunningWorkers() int {
	return c.numberOfRunningWorkers
}

// CollectMetrics implements gardenmetrics.ControllerMetricsCollector interface
func (c *Controller) CollectMetrics(ch chan<- prometheus.Metric) {
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "networkpolicy")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "networkpolicy-controller"}).Inc()
		return
	}
	ch <- metric
}
