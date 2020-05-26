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

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	managedresources "github.com/gardener/gardener/pkg/utils/managedresources"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

// McmShootResourceName is the name of the managed resource that contains the Machine Controller Manager
const McmShootResourceName = "extension-worker-mcm-shoot"

// McmDeploymentName is the name of the deployment that spawn machine-cotroll-manager pods
const McmDeploymentName = "machine-controller-manager"

// ReplicaCount determines the number of replicas.
type ReplicaCount func() (int32, error)

func (a *genericActuator) deployMachineControllerManager(ctx context.Context, workerObj *extensionsv1alpha1.Worker, cluster *controller.Cluster, workerDelegate WorkerDelegate, replicas ReplicaCount) error {
	mcmValues, err := workerDelegate.GetMachineControllerManagerChartValues(ctx)
	if err != nil {
		return err
	}

	// Generate MCM kubeconfig and inject its checksum into the MCM values.
	mcmKubeconfigSecret, err := createKubeconfigForMachineControllerManager(ctx, a.client, workerObj.Namespace, a.mcmName)
	if err != nil {
		return err
	}
	injectPodAnnotation(mcmValues, "checksum/secret-machine-controller-manager", util.ComputeChecksum(mcmKubeconfigSecret.Data))

	replicaCount, err := replicas()
	if err != nil {
		return err
	}
	mcmValues["replicas"] = replicaCount

	if err := a.mcmSeedChart.Apply(ctx, a.chartApplier, workerObj.Namespace,
		a.imageVector, a.gardenerClientset.Version(), cluster.Shoot.Spec.Kubernetes.Version, mcmValues); err != nil {
		return errors.Wrapf(err, "could not apply MCM chart in seed for worker '%s'", util.ObjectName(workerObj))
	}

	if err := a.applyMachineControllerManagerShootChart(ctx, workerDelegate, workerObj, cluster); err != nil {
		return errors.Wrapf(err, "could not apply machine-controller-manager shoot chart")
	}

	return nil
}

func (a *genericActuator) deleteMachineControllerManager(ctx context.Context, workerObj *extensionsv1alpha1.Worker) error {
	a.logger.Info("Deleting the machine-controller-manager", "worker", fmt.Sprintf("%s/%s", workerObj.Namespace, workerObj.Name))

	if err := managedresources.DeleteManagedResource(ctx, a.client, workerObj.Namespace, McmShootResourceName); err != nil {
		return errors.Wrapf(err, "could not delete managed resource containing mcm chart for worker '%s'", util.ObjectName(workerObj))
	}

	timeoutCtx3, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := managedresources.WaitUntilManagedResourceDeleted(timeoutCtx3, a.client, workerObj.Namespace, McmShootResourceName); err != nil {
		return errors.Wrapf(err, "error while waiting for managed resource containing mcm for '%s' to be deleted", util.ObjectName(workerObj))
	}

	if err := a.mcmSeedChart.Delete(ctx, a.client, workerObj.Namespace); err != nil {
		return errors.Wrapf(err, "cleaning up machine-controller-manager resources in seed failed")
	}

	return nil
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

	if err := extensionscontroller.RenderChartAndCreateManagedResource(ctx, workerObj.Namespace, McmShootResourceName, a.client, chartRenderer, a.mcmShootChart, values, a.imageVector, metav1.NamespaceSystem, cluster.Shoot.Spec.Kubernetes.Version, true, false); err != nil {
		return errors.Wrapf(err, "could not apply control plane shoot chart for worker '%s'", util.ObjectName(workerObj))
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
