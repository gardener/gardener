// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package genericactuator

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// TODO(rfranzke): Remove this function after v1.85 has been released.
func (a *genericActuator) cleanupLegacyMachineControllerManagerResources(ctx context.Context, logger logr.Logger, workerObj *extensionsv1alpha1.Worker) error {
	logger.Info("Skip machine-controller-manager deployment since gardenlet manages it - deleting monitoring ConfigMap and extension-worker-mcm-shoot ManagedResource")
	if err := a.seedClient.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager-monitoring-config", Namespace: workerObj.Namespace}}); client.IgnoreNotFound(err) != nil {
		return err
	}
	return managedresources.Delete(ctx, a.seedClient, workerObj.Namespace, "extension-worker-mcm-shoot", false)
}

func scaleMachineControllerManager(ctx context.Context, logger logr.Logger, cl client.Client, worker *extensionsv1alpha1.Worker, replicas int32) error {
	logger.Info("Scaling machine-controller-manager", "replicas", replicas)
	return client.IgnoreNotFound(kubernetes.ScaleDeployment(ctx, cl, kubernetesutils.Key(worker.Namespace, v1beta1constants.DeploymentNameMachineControllerManager), replicas))
}
