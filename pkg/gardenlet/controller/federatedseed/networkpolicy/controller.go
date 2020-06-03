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
	"fmt"
	"sync"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/controller/federatedseed/networkpolicy/helper"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	corev1listers "k8s.io/client-go/listers/core/v1"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller watching the endpoints resource "kubernetes" of the Seeds's kube-apiserver in the default namespace
// to keep the NetworkPolicy "allow-to-seed-apiserver" in sync.
type Controller struct {
	log      *logrus.Entry
	recorder record.EventRecorder

	namespaceReconciler reconcile.Reconciler

	endpointSynced  cache.InformerSynced
	endpointsLister corev1listers.EndpointsLister

	networkPoliciesSynced cache.InformerSynced
	networkPoliciesLister networkingv1listers.NetworkPolicyLister

	namespaceSynced cache.InformerSynced
	namespaceQueue  workqueue.RateLimitingInterface
	namespaceLister corev1listers.NamespaceLister

	shootNamespaceSelector labels.Selector

	workerCh               chan int
	numberOfRunningWorkers int
	waitGroup              sync.WaitGroup
}

// NewController instantiates a new controller.
func NewController(ctx context.Context, seedClient kubernetes.Interface, seedDefaultNamespaceKubeInformer kubeinformers.SharedInformerFactory, seedKubeInformerFactory kubeinformers.SharedInformerFactory, seedLogger *logrus.Entry, recorder record.EventRecorder, seedName string) *Controller {
	var (
		endpointsInformer = seedDefaultNamespaceKubeInformer.Core().V1().Endpoints()
		endpointsLister   = endpointsInformer.Lister()

		networkPoliciesInformer = seedKubeInformerFactory.Networking().V1().NetworkPolicies()
		networkPoliciesLister   = networkPoliciesInformer.Lister()

		namespaceInformer = seedKubeInformerFactory.Core().V1().Namespaces()
		namespaceLister   = namespaceInformer.Lister()
		namespaceQueue    = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "namespace")
	)

	shootNamespaceSelector := labels.SelectorFromSet(labels.Set{
		v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleShoot,
		v1beta1constants.GardenRole:           v1beta1constants.GardenRoleShoot})

	controller := &Controller{
		log:                 seedLogger,
		namespaceReconciler: newNamespaceReconciler(ctx, seedLogger, seedClient.Client(), endpointsLister, networkPoliciesLister, namespaceLister, seedName, shootNamespaceSelector),
		recorder:            recorder,

		endpointSynced:  endpointsInformer.Informer().HasSynced,
		endpointsLister: endpointsLister,

		networkPoliciesLister: networkPoliciesLister,
		networkPoliciesSynced: networkPoliciesInformer.Informer().HasSynced,

		namespaceSynced: namespaceInformer.Informer().HasSynced,
		namespaceQueue:  namespaceQueue,
		namespaceLister: namespaceLister,

		shootNamespaceSelector: shootNamespaceSelector,

		workerCh: make(chan int),
	}

	endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			endpoints, ok := obj.(*corev1.Endpoints)
			if !ok {
				return false
			}
			return endpoints.Name == "kubernetes"
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.endpointAdd,
			UpdateFunc: controller.endpointUpdate,
			DeleteFunc: controller.endpointDelete,
		},
	})

	networkPoliciesInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			policy, ok := obj.(*networkingv1.NetworkPolicy)
			if !ok {
				return false
			}
			return policy.Name == helper.AllowToSeedAPIServer
		},
		Handler: cache.ResourceEventHandlerFuncs{
			UpdateFunc: controller.networkPolicyUpdate,
			DeleteFunc: controller.networkPolicyDelete,
		},
	})

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.namespaceAdd,
		UpdateFunc: controller.namespaceUpdate,
	})

	// start informer & sync caches
	seedDefaultNamespaceKubeInformer.Start(ctx.Done())
	seedKubeInformerFactory.Start(ctx.Done())

	return controller
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*2)
	defer cancel()

	if !cache.WaitForCacheSync(timeoutCtx.Done(), c.endpointSynced, c.namespaceSynced, c.networkPoliciesSynced) {
		return fmt.Errorf("timeout waiting for endpoints informers to sync")
	}

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.namespaceQueue, "namespace", c.namespaceReconciler, &c.waitGroup, c.workerCh)
	}

	c.log.Info("Seed API server network policy controller initialized.")
	return nil
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
