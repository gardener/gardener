// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package csrapprover

import (
	"fmt"

	"github.com/spf13/pflag"
	certificatesv1 "k8s.io/api/certificates/v1"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of the controller.
const ControllerName = "kubelet-csr-approver"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerOptions are options for adding the controller to a Manager.
type ControllerOptions struct {
	maxConcurrentWorkers int
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	MaxConcurrentWorkers int
	TargetCluster        cluster.Cluster
	Namespace            string
}

// AddToManagerWithOptions adds the controller to a Manager with the given config.
func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	if conf.MaxConcurrentWorkers == 0 {
		return nil
	}

	kubernetesClient, err := kubernetesclientset.NewForConfig(conf.TargetCluster.GetConfig())
	if err != nil {
		return fmt.Errorf("failed creating Kubernetes client: %w", err)
	}

	c, err := controller.New(ControllerName, mgr,
		controller.Options{
			MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
			Reconciler: &Reconciler{
				SourceClient:       mgr.GetClient(),
				TargetClient:       conf.TargetCluster.GetClient(),
				CertificatesClient: kubernetesClient.CertificatesV1().CertificateSigningRequests(),
				Namespace:          conf.Namespace,
			},
			RecoverPanic: true,
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&certificatesv1.CertificateSigningRequest{}, conf.TargetCluster.GetCache()),
		&handler.EnqueueRequestForObject{},
		predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update),
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			csr, ok := obj.(*certificatesv1.CertificateSigningRequest)
			return ok && csr.Spec.SignerName == certificatesv1.KubeletServingSignerName
		}),
	)
}

// AddToManager adds the controller to a Manager using the default config.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, defaultControllerConfig)
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&o.maxConcurrentWorkers, "kubelet-csr-approver-max-concurrent-workers", 0, "number of worker threads for concurrent kubelet csr approval reconciliations (default: 0)")
}

// Complete completes the given command line flags and set the defaultControllerConfig accordingly.
func (o *ControllerOptions) Complete() error {
	defaultControllerConfig = ControllerConfig{
		MaxConcurrentWorkers: o.maxConcurrentWorkers,
	}
	return nil
}

// Completed returns the completed ControllerConfig.
func (o *ControllerOptions) Completed() *ControllerConfig {
	return &defaultControllerConfig
}
