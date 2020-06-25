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

package extensions

import (
	"context"
	"sync"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Controller watches the extension resources and has several control loops.
type Controller struct {
	log *logrus.Entry

	manager manager.Manager

	waitGroup              sync.WaitGroup
	workerCh               chan int
	numberOfRunningWorkers int

	controllerInstallationWorkers       int
	controllerInstallationRequiredQueue workqueue.RateLimitingInterface

	controllerInstallationControl controllerInstallationControl
	shootStateControl             shootStateControl
}

const EndpointDisabledAddress = "0"

// NewController creates new controller that syncs extensions states to ShootState
func NewController(ctx context.Context, gardenClient, seedClient kubernetes.Interface, seedName string, log *logrus.Entry, recorder record.EventRecorder, controllerInstallationWorkers, shootStateWorkers int) (*Controller, error) {
	controllerInstallationRequiredQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerinstallation-extension-required")

	mgr, err := manager.New(seedClient.RESTConfig(), manager.Options{
		Scheme: kubernetes.SeedScheme,
		NewClient: func(_ runtimecache.Cache, _ *rest.Config, _ client.Options) (client.Client, error) {
			return seedClient.Client(), nil
		},
		NewCache: func(_ *rest.Config, _ runtimecache.Options) (runtimecache.Cache, error) {
			// use no-op cache because starting the cache multiple times is not possible.
			return &kubernetes.NoOpCache{Cache: seedClient.Cache()}, nil
		},
		MapperProvider: func(_ *rest.Config) (meta.RESTMapper, error) {
			return seedClient.RESTMapper(), nil
		},
		HealthProbeBindAddress: EndpointDisabledAddress,
		MetricsBindAddress:     EndpointDisabledAddress,
	})
	if err != nil {
		return nil, err
	}

	controller := &Controller{
		manager:                       mgr,
		log:                           log,
		workerCh:                      make(chan int),
		controllerInstallationWorkers: controllerInstallationWorkers,

		controllerInstallationRequiredQueue: controllerInstallationRequiredQueue,
		controllerInstallationControl: controllerInstallationControl{
			k8sGardenClient:             gardenClient,
			seedClient:                  seedClient,
			seedName:                    seedName,
			log:                         log,
			controllerInstallationQueue: controllerInstallationRequiredQueue,
			lock:                        &sync.RWMutex{},
			kindToRequiredTypes:         make(map[string]sets.String),
		},
		shootStateControl: shootStateControl{
			k8sGardenClient: gardenClient,
			seedClient:      seedClient,
			log:             log,
			recorder:        recorder,
			shootRetriever:  NewShootRetriever(),
		},
	}

	if err := addToManager(ctx, mgr, controllerInstallationWorkers, shootStateWorkers, controller.controllerInstallationControl, controller.shootStateControl); err != nil {
		return nil, err
	}

	return controller, nil
}

// Run creates workers that reconciles extension resources.
func (s *Controller) Run(ctx context.Context) <-chan error {
	// Count number of running workers.
	go func() {
		for res := range s.workerCh {
			s.numberOfRunningWorkers += res
			s.log.Debugf("Current number of running extension controller workers is %d", s.numberOfRunningWorkers)
		}
	}()

	for i := 0; i < s.controllerInstallationWorkers; i++ {
		controllerutils.CreateWorker(ctx, s.controllerInstallationRequiredQueue, "ControllerInstallation-Required", s.controllerInstallationControl.createControllerInstallationRequiredReconcileFunc(ctx), &s.waitGroup, s.workerCh)
	}

	errs := make(chan error)
	s.waitGroup.Add(1)

	go func() {
		defer s.waitGroup.Done()
		if err := s.manager.Start(ctx.Done()); err != nil {
			errs <- err
		}
		close(errs)
	}()

	return errs
}

// Stop the controller
func (s *Controller) Stop() {
	s.controllerInstallationRequiredQueue.ShutDown()
	s.waitGroup.Wait()
}

func extensionStateOrResourcesChanged(e event.UpdateEvent) bool {
	return extensionPredicateFunc(
		func(new, old extensionsv1alpha1.Object) bool {
			return !apiequality.Semantic.DeepEqual(new.GetExtensionStatus().GetState(), old.GetExtensionStatus().GetState()) ||
				!apiequality.Semantic.DeepEqual(new.GetExtensionStatus().GetResources(), old.GetExtensionStatus().GetResources())
		},
	)(e.ObjectNew, e.ObjectOld)
}

func extensionTypeChanged(e event.UpdateEvent) bool {
	return extensionPredicateFunc(
		func(new, old extensionsv1alpha1.Object) bool {
			return old.GetExtensionSpec().GetExtensionType() != new.GetExtensionSpec().GetExtensionType()
		},
	)(e.ObjectNew, e.ObjectOld)
}

func dnsTypeChanged(e event.UpdateEvent) bool {
	var (
		newExtensionObj, ok1 = e.ObjectNew.(*dnsv1alpha1.DNSProvider)
		oldExtensionObj, ok2 = e.ObjectOld.(*dnsv1alpha1.DNSProvider)
	)
	return ok1 && ok2 && oldExtensionObj.Spec.Type != newExtensionObj.Spec.Type
}

func extensionPredicateFunc(f func(extensionsv1alpha1.Object, extensionsv1alpha1.Object) bool) func(interface{}, interface{}) bool {
	return func(newObj, oldObj interface{}) bool {
		var (
			newExtensionObj, ok1 = newObj.(extensionsv1alpha1.Object)
			oldExtensionObj, ok2 = oldObj.(extensionsv1alpha1.Object)
		)
		return ok1 && ok2 && f(newExtensionObj, oldExtensionObj)
	}
}
