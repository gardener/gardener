// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shootstate/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shootstate/secret"
)

// AddToManager adds all controllers to the given manager.
func AddToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	cfg config.GardenletConfiguration,
) error {
	for objectKind, newObjectFunc := range map[string]func() client.Object{
		extensionsv1alpha1.BackupEntryResource:           func() client.Object { return &extensionsv1alpha1.BackupEntry{} },
		extensionsv1alpha1.ContainerRuntimeResource:      func() client.Object { return &extensionsv1alpha1.ContainerRuntime{} },
		extensionsv1alpha1.ControlPlaneResource:          func() client.Object { return &extensionsv1alpha1.ControlPlane{} },
		extensionsv1alpha1.DNSRecordResource:             func() client.Object { return &extensionsv1alpha1.DNSRecord{} },
		extensionsv1alpha1.ExtensionResource:             func() client.Object { return &extensionsv1alpha1.Extension{} },
		extensionsv1alpha1.InfrastructureResource:        func() client.Object { return &extensionsv1alpha1.Infrastructure{} },
		extensionsv1alpha1.NetworkResource:               func() client.Object { return &extensionsv1alpha1.Network{} },
		extensionsv1alpha1.OperatingSystemConfigResource: func() client.Object { return &extensionsv1alpha1.OperatingSystemConfig{} },
		extensionsv1alpha1.WorkerResource:                func() client.Object { return &extensionsv1alpha1.Worker{} },
	} {
		if err := (&extensions.Reconciler{
			Config:        *cfg.Controllers.ShootStateSync,
			ObjectKind:    objectKind,
			NewObjectFunc: newObjectFunc,
		}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
			return fmt.Errorf("failed adding extensions reconciler for %s: %w", objectKind, err)
		}
	}

	if err := (&secret.Reconciler{
		Config: *cfg.Controllers.ShootSecret,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding secret reconciler: %w", err)
	}

	return nil
}
