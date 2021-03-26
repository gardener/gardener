// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	// McmShootResourceName is the name of the managed resource that contains the Machine Controller Manager
	McmShootResourceName = "extension-worker-mcm-shoot"

	// McmDeploymentName is the name of the deployment that spawn machine-cotroll-manager pods
	McmDeploymentName = "machine-controller-manager"
)

// ReplicaCount determines the number of replicas.
type ReplicaCount func() (int32, error)

func (a *genericActuator) deployMachineControllerManager(ctx context.Context, logger logr.Logger, workerObj *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster, workerDelegate WorkerDelegate, replicas ReplicaCount) error {
	logger.Info("Deploying the machine-controller-manager")

	mcmValues, err := workerDelegate.GetMachineControllerManagerChartValues(ctx)
	if err != nil {
		return err
	}

	// Generate MCM kubeconfig and inject its checksum into the MCM values.
	mcmKubeconfigSecret, err := createKubeconfigForMachineControllerManager(ctx, a.client, workerObj.Namespace, a.mcmName)
	if err != nil {
		return err
	}
	injectPodAnnotation(mcmValues, "checksum/secret-machine-controller-manager", utils.ComputeChecksum(mcmKubeconfigSecret.Data))

	replicaCount, err := replicas()
	if err != nil {
		return err
	}
	mcmValues["replicas"] = replicaCount

	if err := a.mcmSeedChart.Apply(ctx, a.chartApplier, workerObj.Namespace,
		a.imageVector, a.gardenerClientset.Version(), cluster.Shoot.Spec.Kubernetes.Version, mcmValues); err != nil {
		return errors.Wrapf(err, "could not apply MCM chart in seed for worker '%s'", kutil.ObjectName(workerObj))
	}

	if err := a.applyMachineControllerManagerShootChart(ctx, workerDelegate, workerObj, cluster); err != nil {
		return errors.Wrapf(err, "could not apply machine-controller-manager shoot chart")
	}

	logger.Info("Waiting until rollout of machine-controller-manager Deployment is completed")
	if err := kubernetes.WaitUntilDeploymentRolloutIsComplete(ctx, a.client, workerObj.Namespace, McmDeploymentName, 5*time.Second, 300*time.Second); err != nil {
		return errors.Wrapf(err, "waiting until deployment/%s is updated", McmDeploymentName)
	}

	return nil
}

func (a *genericActuator) deleteMachineControllerManager(ctx context.Context, logger logr.Logger, workerObj *extensionsv1alpha1.Worker) error {
	logger.Info("Deleting the machine-controller-manager")
	if err := managedresources.Delete(ctx, a.client, workerObj.Namespace, McmShootResourceName, false); err != nil {
		return errors.Wrapf(err, "could not delete managed resource containing mcm chart for worker '%s'", kutil.ObjectName(workerObj))
	}

	logger.Info("Waiting for machine-controller-manager ManagedResource to be deleted")
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := managedresources.WaitUntilDeleted(timeoutCtx, a.client, workerObj.Namespace, McmShootResourceName); err != nil {
		return errors.Wrapf(err, "error while waiting for managed resource containing mcm for '%s' to be deleted", kutil.ObjectName(workerObj))
	}

	if err := a.mcmSeedChart.Delete(ctx, a.client, workerObj.Namespace); err != nil {
		return errors.Wrapf(err, "cleaning up machine-controller-manager resources in seed failed")
	}

	return nil
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
	return client.IgnoreNotFound(kubernetes.ScaleDeployment(ctx, a.client, kutil.Key(worker.Namespace, McmDeploymentName), replicas))
}

func (a *genericActuator) applyMachineControllerManagerShootChart(ctx context.Context, workerDelegate WorkerDelegate, workerObj *extensionsv1alpha1.Worker, cluster *controller.Cluster) error {
	// Create shoot chart renderer
	chartRenderer, err := a.chartRendererFactory.NewChartRendererForShoot(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return errors.Wrapf(err, "could not create chart renderer for shoot '%s'", workerObj.Namespace)
	}

	// Get machine-controller-manager shoot chart values
	values, err := workerDelegate.GetMachineControllerManagerShootChartValues(ctx)
	if err != nil {
		return err
	}

	if err := managedresources.RenderChartAndCreate(ctx, workerObj.Namespace, McmShootResourceName, false, a.client, chartRenderer, a.mcmShootChart, values, a.imageVector, metav1.NamespaceSystem, cluster.Shoot.Spec.Kubernetes.Version, true, false); err != nil {
		return errors.Wrapf(err, "could not apply control plane shoot chart for worker '%s'", kutil.ObjectName(workerObj))
	}

	return nil
}

// createKubeconfigForMachineControllerManager generates a new certificate and kubeconfig for the machine-controller-manager. If
// such credentials already exist then they will be returned.
func createKubeconfigForMachineControllerManager(ctx context.Context, c client.Client, namespace, name string) (*corev1.Secret, error) {
	certConfig := secrets.CertificateSecretConfig{
		Name:       name,
		CommonName: fmt.Sprintf("system:%s", name),
	}

	return util.GetOrCreateShootKubeconfig(ctx, c, certConfig, namespace)
}

func injectPodAnnotation(values map[string]interface{}, key string, value interface{}) {
	podAnnotations, ok := values["podAnnotations"]
	if !ok {
		values["podAnnotations"] = map[string]interface{}{
			key: value,
		}
	} else {
		podAnnotations.(map[string]interface{})[key] = value
	}
}
