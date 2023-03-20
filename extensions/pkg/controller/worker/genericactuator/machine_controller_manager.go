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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// McmShootResourceName is the name of the managed resource that contains the Machine Controller Manager
	McmShootResourceName = "extension-worker-mcm-shoot"

	// McmDeploymentName is the name of the deployment that spawn machine-cotroll-manager pods
	McmDeploymentName = "machine-controller-manager"
)

// ReplicaCount determines the number of replicas.
type ReplicaCount func() int32

func (a *genericActuator) deployMachineControllerManager(ctx context.Context, logger logr.Logger, workerObj *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster, workerDelegate WorkerDelegate, replicas ReplicaCount) error {
	logger.Info("Deploying the machine-controller-manager")

	mcmValues, err := workerDelegate.GetMachineControllerManagerChartValues(ctx)
	if err != nil {
		return err
	}

	mcmValues["useTokenRequestor"] = true
	mcmValues["useProjectedTokenMount"] = true

	if err := gardenerutils.NewShootAccessSecret(a.mcmName, workerObj.Namespace).Reconcile(ctx, a.client); err != nil {
		return err
	}
	mcmValues["genericTokenKubeconfigSecretName"] = extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster(cluster)

	replicaCount := replicas()
	mcmValues["replicas"] = replicaCount

	if err := a.mcmSeedChart.Apply(ctx, a.chartApplier, workerObj.Namespace,
		a.imageVector, a.gardenerClientset.Version(), cluster.Shoot.Spec.Kubernetes.Version, mcmValues); err != nil {
		return fmt.Errorf("could not apply MCM chart in seed for worker '%s': %w", kubernetesutils.ObjectName(workerObj), err)
	}

	if err := a.applyMachineControllerManagerShootChart(ctx, workerDelegate, workerObj, cluster); err != nil {
		return fmt.Errorf("could not apply machine-controller-manager shoot chart: %w", err)
	}

	logger.Info("Waiting until rollout of machine-controller-manager Deployment is completed")
	if err := kubernetes.WaitUntilDeploymentRolloutIsComplete(ctx, a.client, workerObj.Namespace, McmDeploymentName, 5*time.Second, 300*time.Second); err != nil {
		return fmt.Errorf("waiting until deployment/%s is updated: %w", McmDeploymentName, err)
	}

	return nil
}

func (a *genericActuator) deleteMachineControllerManager(ctx context.Context, logger logr.Logger, workerObj *extensionsv1alpha1.Worker) error {
	logger.Info("Deleting the machine-controller-manager")
	if err := managedresources.Delete(ctx, a.client, workerObj.Namespace, McmShootResourceName, false); err != nil {
		return fmt.Errorf("could not delete managed resource containing mcm chart for worker '%s': %w", kubernetesutils.ObjectName(workerObj), err)
	}

	logger.Info("Waiting for machine-controller-manager ManagedResource to be deleted")
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := managedresources.WaitUntilDeleted(timeoutCtx, a.client, workerObj.Namespace, McmShootResourceName); err != nil {
		return fmt.Errorf("error while waiting for managed resource containing mcm for '%s' to be deleted: %w", kubernetesutils.ObjectName(workerObj), err)
	}

	if err := a.mcmSeedChart.Delete(ctx, a.client, workerObj.Namespace); err != nil {
		return fmt.Errorf("cleaning up machine-controller-manager resources in seed failed: %w", err)
	}

	return kubernetesutils.DeleteObject(ctx, a.client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shoot-access-" + a.mcmName, Namespace: workerObj.Namespace}})
}

func (a *genericActuator) waitUntilMachineControllerManagerIsDeleted(ctx context.Context, logger logr.Logger, namespace string) error {
	logger.Info("Waiting until machine-controller-manager is deleted")
	return wait.PollUntil(5*time.Second, func() (bool, error) {
		machineControllerManagerDeployment := &appsv1.Deployment{}
		if err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: McmDeploymentName}, machineControllerManagerDeployment); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}

		return false, nil
	}, ctx.Done())
}

func (a *genericActuator) scaleMachineControllerManager(ctx context.Context, logger logr.Logger, worker *extensionsv1alpha1.Worker, replicas int32) error {
	logger.Info("Scaling machine-controller-manager", "replicas", replicas)
	return client.IgnoreNotFound(kubernetes.ScaleDeployment(ctx, a.client, kubernetesutils.Key(worker.Namespace, McmDeploymentName), replicas))
}

func (a *genericActuator) applyMachineControllerManagerShootChart(ctx context.Context, workerDelegate WorkerDelegate, workerObj *extensionsv1alpha1.Worker, cluster *controller.Cluster) error {
	// Create shoot chart renderer
	chartRenderer, err := a.chartRendererFactory.NewChartRendererForShoot(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return fmt.Errorf("could not create chart renderer for shoot '%s': %w", workerObj.Namespace, err)
	}

	// Get machine-controller-manager shoot chart values
	values, err := workerDelegate.GetMachineControllerManagerShootChartValues(ctx)
	if err != nil {
		return err
	}

	values["useTokenRequestor"] = true

	if err := managedresources.RenderChartAndCreate(ctx, workerObj.Namespace, McmShootResourceName, false, a.client, chartRenderer, a.mcmShootChart, values, a.imageVector, metav1.NamespaceSystem, cluster.Shoot.Spec.Kubernetes.Version, true, false); err != nil {
		return fmt.Errorf("could not apply control plane shoot chart for worker '%s': %w", kubernetesutils.ObjectName(workerObj), err)
	}

	return nil
}
