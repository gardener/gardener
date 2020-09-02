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

package federatedseed

import (
	"context"
	"fmt"
	"sync"

	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/federatedseed/extensions"
	seedapiservernetworkpolicy "github.com/gardener/gardener/pkg/gardenlet/controller/federatedseed/networkpolicy"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller is responsible for maintaining multiple federated Seeds' controllers
type Controller struct {
	federatedSeedControllerManager *federatedSeedControllerManager
	clientMap                      clientmap.ClientMap

	config *config.GardenletConfiguration

	gardenClient client.Client

	seedInformer runtimecache.Informer
	seedQueue    workqueue.RateLimitingInterface

	recorder record.EventRecorder

	numberOfRunningWorkers int
	workerCh               chan int

	lock sync.RWMutex
}

// NewFederatedSeedController creates new controller that reconciles extension resources.
func NewFederatedSeedController(ctx context.Context, clientMap clientmap.ClientMap, config *config.GardenletConfiguration, recorder record.EventRecorder) (*Controller, error) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}
	seedInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Seed{})
	if err != nil {
		return nil, err
	}

	controller := &Controller{
		clientMap:    clientMap,
		gardenClient: gardenClient.Client(),
		seedInformer: seedInformer,
		seedQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed-controllers"),
		config:       config,
		workerCh:     make(chan int),
		recorder:     recorder,
	}

	controller.federatedSeedControllerManager = &federatedSeedControllerManager{
		clientMap:            clientMap,
		extensionControllers: make(map[string]*extensionsController),
		namespaceControllers: make(map[string]*namespaceController),
	}

	return controller, nil
}

func (c *Controller) seedAdd(obj interface{}) {
	seedObj := obj.(*gardencorev1beta1.Seed)
	bootstrappedCondition := helper.GetCondition(seedObj.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
	if bootstrappedCondition == nil || bootstrappedCondition.Status != gardencorev1beta1.ConditionTrue {
		return
	}
	c.addSeedToQueue(obj)
}

func (c *Controller) addSeedToQueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.seedQueue.Add(key)
}

func (c *Controller) seedUpdate(_, newObj interface{}) {
	var newSeedObj = newObj.(*gardencorev1beta1.Seed)

	newSeedBootstrappedCondition := helper.GetCondition(newSeedObj.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
	c.lock.Lock()
	defer c.lock.Unlock()
	_, found := c.federatedSeedControllerManager.extensionControllers[newSeedObj.Name]
	if !found && newSeedBootstrappedCondition != nil && newSeedBootstrappedCondition.Status == gardencorev1beta1.ConditionTrue {
		c.addSeedToQueue(newSeedObj)
	}
}

func (c *Controller) seedDelete(obj interface{}) {
	seed := obj.(*gardencorev1beta1.Seed)
	c.federatedSeedControllerManager.removeController(seed.GetName())
}

// Run starts the FederatedSeed Controller
func (c *Controller) Run(ctx context.Context, workers int) {
	c.seedInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(c.config.SeedConfig), c.config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.seedAdd,
			UpdateFunc: c.seedUpdate,
			DeleteFunc: c.seedDelete,
		},
	})

	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.seedInformer.HasSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Seed Federated controller workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Infof("Starting federated controllers")
	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.seedQueue, "Seed", c.createReconcileSeedRequestFunc(ctx), &waitGroup, c.workerCh)
	}

	<-ctx.Done()

	logger.Logger.Infof("Shutting down federated controllers")
	c.federatedSeedControllerManager.ShutDownControllers()
	c.seedQueue.ShutDown()
	waitGroup.Wait()
}

func (c *Controller) createReconcileSeedRequestFunc(ctx context.Context) reconcile.Func {
	return func(req reconcile.Request) (reconcile.Result, error) {
		seed := &gardencorev1beta1.Seed{}
		if err := c.gardenClient.Get(ctx, kutil.Key(req.Name), seed); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Logger.Infof("Skipping federated seed setup - Seed %s was not found", req.Name)
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		if seed.DeletionTimestamp != nil {
			logger.Logger.Infof("Skipping federated seed setup - Seed %s is being deleted", req.Name)
			return reconcile.Result{}, nil
		}

		condition := helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
		if condition.Status != gardencorev1beta1.ConditionTrue {
			return reconcile.Result{}, nil
		}

		seedLogger := logger.Logger.WithField("Seed", seed.GetName())
		if err := c.federatedSeedControllerManager.createExtensionControllers(ctx, seedLogger, seed.GetName(), c.clientMap, c.config, c.recorder); err != nil {
			return reconcile.Result{}, err
		}

		if err := c.federatedSeedControllerManager.createNamespaceControllers(ctx, seedLogger, seed.GetName(), c.clientMap, c.config, c.recorder); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}

type federatedSeedControllerManager struct {
	clientMap                clientmap.ClientMap
	extensionControllers     map[string]*extensionsController
	namespaceControllers     map[string]*namespaceController
	extensionControllersLock sync.RWMutex
	namespaceControllersLock sync.RWMutex
}

type extensionsController struct {
	controller *extensions.Controller
	cancelFunc context.CancelFunc
}

func (s *extensionsController) Stop() {
	s.cancelFunc()
	s.controller.Stop()
}

type namespaceController struct {
	controller *seedapiservernetworkpolicy.Controller
	cancelFunc context.CancelFunc
}

func (s *namespaceController) Stop() {
	logger.Logger.Infof("Stopping extension controller")
	s.cancelFunc()
	s.controller.Stop()
}

func (f *federatedSeedControllerManager) createExtensionControllers(ctx context.Context, seedLogger *logrus.Entry, seedName string, clientMap clientmap.ClientMap, config *config.GardenletConfiguration, recorder record.EventRecorder) error {
	f.extensionControllersLock.Lock()
	if _, found := f.extensionControllers[seedName]; found {
		f.extensionControllersLock.Unlock()
		logger.Logger.Debugf("Extension controllers are already started for seed %s", seedName)
		return nil
	}
	f.extensionControllersLock.Unlock()

	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	seedClient, err := clientMap.GetClient(ctx, keys.ForSeedWithName(seedName))
	if err != nil {
		return fmt.Errorf("failed to get seed client: %w", err)
	}

	extensionsController, err := extensions.NewController(ctx, gardenClient, seedClient, seedName, seedLogger, recorder)
	if err != nil {
		return err
	}
	extensionCtx, extensionCancelFunc := context.WithCancel(ctx)

	seedLogger.Info("Run extensions controller")
	if err := extensionsController.Run(extensionCtx, *config.Controllers.ControllerInstallationRequired.ConcurrentSyncs, *config.Controllers.ShootStateSync.ConcurrentSyncs); err != nil {
		seedLogger.Infof("There was an error running the extension controllers: %v", err)
		extensionCancelFunc()
		return err
	}

	f.addExtensionController(seedName, extensionsController, extensionCancelFunc)
	return nil
}

func (f *federatedSeedControllerManager) createNamespaceControllers(ctx context.Context, seedLogger *logrus.Entry, seedName string, clientMap clientmap.ClientMap, config *config.GardenletConfiguration, recorder record.EventRecorder) error {
	f.namespaceControllersLock.Lock()
	if _, found := f.namespaceControllers[seedName]; found {
		f.namespaceControllersLock.Unlock()
		logger.Logger.Debugf("Namespace controllers are already started for seed %s", seedName)
		return nil
	}
	f.namespaceControllersLock.Unlock()

	seedClient, err := clientMap.GetClient(ctx, keys.ForSeedWithName(seedName))
	if err != nil {
		return fmt.Errorf("failed to get seed client: %w", err)
	}

	// if another controller also needs to work with endpoint resources in the Seed cluster, consider replacing this informer factory with a reusable informer factory.
	// if this informer factory is not bound to the default namespace, make sure that the endpoint event handlers FilterFunc() also filters for the default namespace.
	seedDefaultNamespaceKubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(seedClient.Kubernetes(), 0, kubeinformers.WithNamespace(corev1.NamespaceDefault))

	namespaceController, err := seedapiservernetworkpolicy.NewController(ctx, seedClient, seedDefaultNamespaceKubeInformer, seedLogger, recorder, seedName)
	if err != nil {
		return err
	}

	seedLogger.Info("Run namespace controller")
	namespaceCtx, namespaceCancelFunc := context.WithCancel(ctx)
	if err := namespaceController.Run(namespaceCtx, *config.Controllers.SeedAPIServerNetworkPolicy.ConcurrentSyncs); err != nil {
		seedLogger.Infof("There was an error running the namespace controller: %v", err)
		namespaceCancelFunc()
		return err
	}

	f.addNamespaceController(seedName, namespaceController, namespaceCancelFunc)
	return nil
}

func (f *federatedSeedControllerManager) addExtensionController(seedName string, controller *extensions.Controller, cancelFunc context.CancelFunc) {
	logger.Logger.Debugf("Adding extension controllers for seed %s", seedName)
	f.extensionControllersLock.Lock()
	defer f.extensionControllersLock.Unlock()
	f.extensionControllers[seedName] = &extensionsController{
		controller: controller,
		cancelFunc: cancelFunc,
	}
}

func (f *federatedSeedControllerManager) addNamespaceController(seedName string, controller *seedapiservernetworkpolicy.Controller, cancelFunc context.CancelFunc) {
	logger.Logger.Debugf("Adding namespace controller for seed %s", seedName)
	f.namespaceControllersLock.Lock()
	defer f.namespaceControllersLock.Unlock()
	f.namespaceControllers[seedName] = &namespaceController{
		controller: controller,
		cancelFunc: cancelFunc,
	}
}

func (f *federatedSeedControllerManager) removeController(seedName string) {
	if controller, ok := f.extensionControllers[seedName]; ok {
		logger.Logger.Debugf("Removing extension controller for seed %s", seedName)
		controller.Stop()
		f.extensionControllersLock.Lock()
		defer f.extensionControllersLock.Unlock()
		delete(f.extensionControllers, seedName)
	}
	if controller, ok := f.namespaceControllers[seedName]; ok {
		logger.Logger.Debugf("Removing namespace controller for seed %s", seedName)
		controller.Stop()
		f.namespaceControllersLock.Lock()
		defer f.namespaceControllersLock.Unlock()
		delete(f.namespaceControllers, seedName)
	}

	if err := f.clientMap.InvalidateClient(keys.ForSeedWithName(seedName)); err != nil {
		logger.Logger.Errorf("Failed to invalidate seed client: %v", err)
	}
}

func (f *federatedSeedControllerManager) ShutDownControllers() {
	logger.Logger.Infof("Federated controllers are being stopped")
	for _, controller := range f.extensionControllers {
		controller.Stop()
	}
	for _, controller := range f.namespaceControllers {
		controller.Stop()
	}
}
