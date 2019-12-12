// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootstate

import (
	"context"
	"fmt"
	"sync"
	"time"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsclientset "github.com/gardener/gardener/pkg/client/extensions/clientset/versioned"
	extensionsinformers "github.com/gardener/gardener/pkg/client/extensions/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SyncController replicates the extensions' resources states in Shoot's ShootState resource
type SyncController struct {
	k8sGardenClient         kubernetes.Interface
	seedClient              kubernetes.Interface
	extensionsInformers     extensionsinformers.SharedInformerFactory
	extensionsSeedClient    extensionsclientset.Interface
	syncControllerArtifacts *syncControllerArtifacts
	log                     *logrus.Entry
	recorder                record.EventRecorder

	waitGroup              sync.WaitGroup
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSyncController creates new controller that syncs extensions states to ShootState
func NewSyncController(ctx context.Context, gardenClient, seedClient kubernetes.Interface, extensionsSeedClient extensionsclientset.Interface, extensionsInformers extensionsinformers.SharedInformerFactory, config *config.GardenletConfiguration, log *logrus.Entry, recorder record.EventRecorder) *SyncController {
	controller := &SyncController{
		k8sGardenClient:      gardenClient,
		seedClient:           seedClient,
		extensionsSeedClient: extensionsSeedClient,
		extensionsInformers:  extensionsInformers,
		log:                  log,

		recorder:  recorder,
		waitGroup: sync.WaitGroup{},
		workerCh:  make(chan int),
	}

	controller.syncControllerArtifacts = &syncControllerArtifacts{
		controllerArtifacts: make(map[string]*artifacts),
	}

	controller.syncControllerArtifacts.initialize(extensionsInformers)
	controller.syncControllerArtifacts.addEventHandlers()

	controller.extensionsInformers.Start(ctx.Done())
	return controller
}

// Run creates workers that reconciles extension resources and syncs their state into ShootState
func (s *SyncController) Run(ctx context.Context, shootStateSyncWorkersCount int) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*2)
	defer cancel()

	if !cache.WaitForCacheSync(timeoutCtx.Done(), s.syncControllerArtifacts.hasSyncedFuncs...) {
		return fmt.Errorf("Timeout waiting for extension informers to sync")
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-s.workerCh:
				s.numberOfRunningWorkers += res
				s.log.Debugf("Current number of running shoot state sync workers is %d", s.numberOfRunningWorkers)
			}
		}
	}()

	for i := 0; i <= shootStateSyncWorkersCount; i++ {
		s.createWorkers(ctx)
	}

	s.log.Info("ShootState sync controller initialized.")
	return nil
}

func (s *SyncController) createWorkers(ctx context.Context) {
	for kind, artifact := range s.syncControllerArtifacts.controllerArtifacts {
		workerName := fmt.Sprintf("%s-Sync", kind)
		controllerutils.CreateWorker(ctx, artifact.workqueue, workerName, reconcile.Func(s.createShootStateSyncReconcileFunc(ctx, kind, artifact.objectCreator)), &s.waitGroup, s.workerCh)
	}
}

// Stop the controller
func (s *SyncController) Stop() {
	s.syncControllerArtifacts.shutdownQueues()
	s.waitGroup.Wait()
}

type syncControllerArtifacts struct {
	controllerArtifacts map[string]*artifacts
	hasSyncedFuncs      []cache.InformerSynced
}

type artifacts struct {
	objectCreator func() runtime.Object
	informer      cache.SharedIndexInformer
	workqueue     workqueue.RateLimitingInterface
}

func (sca *syncControllerArtifacts) initialize(extensionsInformers extensionsinformers.SharedInformerFactory) {
	var (
		infraInformer        = extensionsInformers.Extensions().V1alpha1().Infrastructures()
		workerInformer       = extensionsInformers.Extensions().V1alpha1().Workers()
		backupEntryInformer  = extensionsInformers.Extensions().V1alpha1().BackupEntries()
		extensionInformer    = extensionsInformers.Extensions().V1alpha1().Extensions()
		controlPlaneInformer = extensionsInformers.Extensions().V1alpha1().ControlPlanes()
		networkInformer      = extensionsInformers.Extensions().V1alpha1().Networks()
		oscInformer          = extensionsInformers.Extensions().V1alpha1().OperatingSystemConfigs()
	)

	sca.registerExtensionControllerArtifacts(extensionsv1alpha1.InfrastructureResource, func() runtime.Object { return &extensionsv1alpha1.Infrastructure{} }, infraInformer.Informer())
	sca.registerExtensionControllerArtifacts(extensionsv1alpha1.WorkerResource, func() runtime.Object { return &extensionsv1alpha1.Worker{} }, workerInformer.Informer())
	sca.registerExtensionControllerArtifacts(extensionsv1alpha1.BackupEntryResource, func() runtime.Object { return &extensionsv1alpha1.BackupEntry{} }, backupEntryInformer.Informer())
	sca.registerExtensionControllerArtifacts(extensionsv1alpha1.ExtensionResource, func() runtime.Object { return &extensionsv1alpha1.Extension{} }, extensionInformer.Informer())
	sca.registerExtensionControllerArtifacts(extensionsv1alpha1.ControlPlaneResource, func() runtime.Object { return &extensionsv1alpha1.ControlPlane{} }, controlPlaneInformer.Informer())
	sca.registerExtensionControllerArtifacts(extensionsv1alpha1.NetworkResource, func() runtime.Object { return &extensionsv1alpha1.Network{} }, networkInformer.Informer())
	sca.registerExtensionControllerArtifacts(extensionsv1alpha1.OperatingSystemConfigResource, func() runtime.Object { return &extensionsv1alpha1.OperatingSystemConfig{} }, oscInformer.Informer())
}

func (sca *syncControllerArtifacts) registerExtensionControllerArtifacts(kind string, objectCreator func() runtime.Object, informer cache.SharedIndexInformer) {
	workqueueName := fmt.Sprintf("%s-controller", kind)
	workqueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workqueueName)

	sca.hasSyncedFuncs = append(sca.hasSyncedFuncs, informer.HasSynced)
	sca.controllerArtifacts[kind] = &artifacts{
		objectCreator: objectCreator,
		informer:      informer,
		workqueue:     workqueue,
	}
}

func (sca *syncControllerArtifacts) addEventHandlers() {
	for _, artifact := range sca.controllerArtifacts {
		artifact.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    createEnqueueOnAddFunc(artifact.workqueue),
			UpdateFunc: createEnqueueOnUpdateFunc(artifact.workqueue),
		})
	}
}

func (sca *syncControllerArtifacts) shutdownQueues() {
	for _, artifact := range sca.controllerArtifacts {
		artifact.workqueue.ShutDown()
	}
}
