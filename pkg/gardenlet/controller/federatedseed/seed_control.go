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
	"sync"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	extensionsclientset "github.com/gardener/gardener/pkg/client/extensions/clientset/versioned"
	extensionsinformers "github.com/gardener/gardener/pkg/client/extensions/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/gardenlet/controller/federatedseed/extensions"
	seedapiservernetworkpolicy "github.com/gardener/gardener/pkg/gardenlet/controller/federatedseed/networkpolicy"
	"github.com/gardener/gardener/pkg/logger"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"

	dnsclientset "github.com/gardener/external-dns-management/pkg/client/dns/clientset/versioned"
	dnsinformers "github.com/gardener/external-dns-management/pkg/client/dns/informers/externalversions"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller is responsible for maintaining multiple federated Seeds' controllers
type Controller struct {
	federatedSeedControllerManager *federatedSeedControllerManager
	k8sGardenClient                kubernetes.Interface

	config *config.GardenletConfiguration

	seedLister    gardencorelisters.SeedLister
	seedHasSynced cache.InformerSynced
	seedQueue     workqueue.RateLimitingInterface

	recorder record.EventRecorder

	numberOfRunningWorkers int
	workerCh               chan int

	lock sync.RWMutex
}

// NewFederatedSeedController creates new controller that reconciles extension resources.
func NewFederatedSeedController(k8sGardenClient kubernetes.Interface, gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory, config *config.GardenletConfiguration, recorder record.EventRecorder) *Controller {
	var seedInformer = gardenCoreInformerFactory.Core().V1beta1().Seeds()

	controller := &Controller{
		k8sGardenClient: k8sGardenClient,
		seedLister:      seedInformer.Lister(),
		seedQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed-controllers"),
		config:          config,
		workerCh:        make(chan int),
		recorder:        recorder,
	}

	controller.federatedSeedControllerManager = &federatedSeedControllerManager{
		extensionControllers: make(map[string]*extensionsController),
		namespaceControllers: make(map[string]*namespaceController),
	}

	seedInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig), config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.seedAdd,
			UpdateFunc: controller.seedUpdate,
			DeleteFunc: controller.seedDelete,
		},
	})

	controller.seedHasSynced = seedInformer.Informer().HasSynced
	return controller
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
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.seedHasSynced) {
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
		seed, err := c.seedLister.Get(req.Name)
		if apierrors.IsNotFound(err) {
			logger.Logger.Infof("Skipping - Seed %s was not found", req.Name)
			return reconcile.Result{}, nil
		}
		if err != nil {
			return reconcile.Result{}, err
		}

		if seed.DeletionTimestamp != nil {
			c.federatedSeedControllerManager.removeController(seed.GetName())
			return reconcile.Result{}, nil
		}

		condition := helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
		if condition.Status != gardencorev1beta1.ConditionTrue {
			return reconcile.Result{}, nil
		}

		seedLogger := logger.Logger.WithField("Seed", seed.GetName())
		if err := c.federatedSeedControllerManager.createExtensionControllers(ctx, seedLogger, seed.GetName(), c.k8sGardenClient, c.config, c.recorder); err != nil {
			return reconcile.Result{}, err
		}

		if err := c.federatedSeedControllerManager.createNamespaceControllers(ctx, seedLogger, seed.GetName(), c.k8sGardenClient, c.config, c.recorder); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}

type federatedSeedControllerManager struct {
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

func (f *federatedSeedControllerManager) createExtensionControllers(ctx context.Context, seedLogger *logrus.Entry, seedName string, k8sGardenClient kubernetes.Interface, config *config.GardenletConfiguration, recorder record.EventRecorder) error {
	f.extensionControllersLock.Lock()
	if _, found := f.extensionControllers[seedName]; found {
		f.extensionControllersLock.Unlock()
		logger.Logger.Debugf("Extension controllers are already started for seed %s", seedName)
		return nil
	}
	f.extensionControllersLock.Unlock()

	k8sSeedClient, err := seedpkg.GetSeedClient(ctx, k8sGardenClient.Client(), config.SeedClientConnection.ClientConnectionConfiguration, config.SeedSelector == nil, seedName)
	if err != nil {
		return err
	}

	dnsClient, err := dnsclientset.NewForConfig(k8sSeedClient.RESTConfig())
	if err != nil {
		return err
	}
	extensionsClient, err := extensionsclientset.NewForConfig(k8sSeedClient.RESTConfig())
	if err != nil {
		return err
	}

	var (
		extensionCtx, extensionCancelFunc = context.WithCancel(ctx)
		dnsInformers                      = dnsinformers.NewSharedInformerFactory(dnsClient, 0)
		extensionsInformers               = extensionsinformers.NewSharedInformerFactory(extensionsClient, 0)
		extensionsController              = extensions.NewController(ctx, k8sGardenClient, k8sSeedClient, seedName, dnsInformers, extensionsInformers, seedLogger, recorder)
	)

	seedLogger.Info("Run extensions controller")
	if err := extensionsController.Run(extensionCtx, *config.Controllers.ControllerInstallationRequired.ConcurrentSyncs, *config.Controllers.ShootStateSync.ConcurrentSyncs); err != nil {
		seedLogger.Infof("There was an error running the extension controllers: %v", err)
		extensionCancelFunc()
		return err
	}

	f.addExtensionController(seedName, extensionsController, extensionCancelFunc)
	return nil
}

func (f *federatedSeedControllerManager) createNamespaceControllers(ctx context.Context, seedLogger *logrus.Entry, seedName string, k8sGardenClient kubernetes.Interface, config *config.GardenletConfiguration, recorder record.EventRecorder) error {
	f.namespaceControllersLock.Lock()
	if _, found := f.namespaceControllers[seedName]; found {
		f.namespaceControllersLock.Unlock()
		logger.Logger.Debugf("Namespace controllers are already started for seed %s", seedName)
		return nil
	}
	f.namespaceControllersLock.Unlock()

	k8sSeedClient, err := seedpkg.GetSeedClient(ctx, k8sGardenClient.Client(), config.SeedClientConnection.ClientConnectionConfiguration, config.SeedSelector == nil, seedName)
	if err != nil {
		return err
	}

	var (
		namespaceCtx, namespaceCancelFunc = context.WithCancel(ctx)
		seedKubeInformerFactory           = kubeinformers.NewSharedInformerFactory(k8sSeedClient.Kubernetes(), 0)
		// if another controller also needs to work with endpoint resources in the Seed cluster, consider replacing this informer factory with a reusable informer factory.
		// if this informer factory is not bound to the default namespace, make sure that the endpoint event handlers FilterFunc() also filters for the default namespace.
		seedDefaultNamespaceKubeInformer = kubeinformers.NewSharedInformerFactoryWithOptions(k8sSeedClient.Kubernetes(), 0, kubeinformers.WithNamespace(corev1.NamespaceDefault))
		namespaceController              = seedapiservernetworkpolicy.NewController(ctx, k8sSeedClient, seedDefaultNamespaceKubeInformer, seedKubeInformerFactory, seedLogger, recorder, seedName)
	)

	seedLogger.Info("Run namespace controller")
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
