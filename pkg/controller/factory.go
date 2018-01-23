// Copyright 2018 The Gardener Authors.
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

package controller

import (
	"os"
	"time"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	seedcontroller "github.com/gardener/gardener/pkg/controller/seed"
	shootcontroller "github.com/gardener/gardener/pkg/controller/shoot"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/version"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

// GardenControllerFactory contains information relevant to controllers for the Garden API group.
type GardenControllerFactory struct {
	config             *componentconfig.ControllerManagerConfiguration
	identity           *gardenv1beta1.Gardener
	gardenNamespace    string
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory
	recorder           record.EventRecorder
}

// NewGardenControllerFactory creates a new factory for controllers for the Garden API group.
func NewGardenControllerFactory(k8sGardenClient kubernetes.Client, gardenInformerFactory gardeninformers.SharedInformerFactory, config *componentconfig.ControllerManagerConfiguration, identity *gardenv1beta1.Gardener, gardenNamespace string, recorder record.EventRecorder) *GardenControllerFactory {
	return &GardenControllerFactory{
		config:             config,
		identity:           identity,
		gardenNamespace:    gardenNamespace,
		k8sGardenClient:    k8sGardenClient,
		k8sGardenInformers: gardenInformerFactory,
		recorder:           recorder,
	}
}

// Run starts all the controllers for the Garden API group. It also performs bootstrapping tasks.
func (f *GardenControllerFactory) Run(stopCh <-chan struct{}) {
	var (
		cloudProfileInformer         = f.k8sGardenInformers.Garden().V1beta1().CloudProfiles().Informer()
		privateSecretBindingInformer = f.k8sGardenInformers.Garden().V1beta1().PrivateSecretBindings().Informer()
		crossSecretBindingInformer   = f.k8sGardenInformers.Garden().V1beta1().CrossSecretBindings().Informer()
		quotaInformer                = f.k8sGardenInformers.Garden().V1beta1().Quotas().Informer()
		seedInformer                 = f.k8sGardenInformers.Garden().V1beta1().Seeds().Informer()
		shootInformer                = f.k8sGardenInformers.Garden().V1beta1().Shoots().Informer()
	)

	f.k8sGardenInformers.Start(stopCh)
	if !cache.WaitForCacheSync(make(<-chan struct{}), cloudProfileInformer.HasSynced, privateSecretBindingInformer.HasSynced, crossSecretBindingInformer.HasSynced, quotaInformer.HasSynced, seedInformer.HasSynced, shootInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}

	secrets, err := garden.ReadGardenSecrets(f.k8sGardenClient, f.config.GardenNamespace, f.config.ClientConnection.KubeConfigFile == "")
	if err != nil {
		panic(err)
	}

	imageVector, err := imagevector.ReadImageVector()
	if err != nil {
		panic(err)
	}

	if err := garden.BootstrapCluster(f.k8sGardenClient, f.config.GardenNamespace, secrets); err != nil {
		logger.Logger.Errorf("Failed to bootstrap the Garden cluster: %s", err.Error())
		return
	}
	logger.Logger.Info("Successfully bootstrapped the Garden cluster.")

	var (
		workerCount = f.config.Controller.Reconciliation.ConcurrentSyncs

		shootController = shootcontroller.NewShootController(f.k8sGardenClient, f.k8sGardenInformers, f.config, f.identity, f.gardenNamespace, secrets, imageVector, f.recorder)
		seedController  = seedcontroller.NewSeedController(f.k8sGardenClient, f.k8sGardenInformers, secrets, imageVector, f.recorder)
	)

	go shootController.Run(workerCount, stopCh)
	go seedController.Run(workerCount, stopCh)

	logger.Logger.Infof("Garden controller manager (version %s) initialized.", version.Version)

	// Shutdown handling
	<-stopCh
	logger.Logger.Info("I have received a stop signal and will no longer watch events of my API group.")
	logger.Logger.Info("I will terminate as soon as all my running workers have terminated.")

	for {
		if shootController.RunningWorkers() == 0 && seedController.RunningWorkers() == 0 {
			logger.Logger.Info("All controllers have been terminated.")
			break
		}
		time.Sleep(5 * time.Second)
	}

	os.Exit(0)
}
