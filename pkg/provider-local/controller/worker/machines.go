// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"context"
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// DeployMachineClasses generates and creates the local provider specific machine classes.
func (w *workerDelegate) DeployMachineClasses(ctx context.Context) error {
	if w.machineClasses == nil {
		if err := w.generateMachineConfig(); err != nil {
			return err
		}
	}

	for _, obj := range w.machineClassSecrets {
		if err := w.client.Patch(ctx, obj, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return fmt.Errorf("failed to apply machine class secret %s: %w", obj.GetName(), err)
		}
	}

	for _, obj := range w.machineClasses {
		if err := w.client.Patch(ctx, obj, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return fmt.Errorf("failed to apply machine class %s: %w", obj.GetName(), err)
		}
	}

	return nil
}

// GenerateMachineDeployments generates the configuration for the desired machine deployments.
func (w *workerDelegate) GenerateMachineDeployments(_ context.Context) (worker.MachineDeployments, error) {
	if w.machineDeployments == nil {
		if err := w.generateMachineConfig(); err != nil {
			return nil, err
		}
	}
	return w.machineDeployments, nil
}

func (w *workerDelegate) generateMachineConfig() error {
	var (
		machineClassSecrets []*corev1.Secret
		machineClasses      []*machinev1alpha1.MachineClass
		machineImages       []api.MachineImage
		machineDeployments  worker.MachineDeployments
	)

	for _, pool := range w.worker.Spec.Pools {
		workerPoolHash, err := worker.WorkerPoolHash(pool, w.cluster)
		if err != nil {
			return err
		}

		image, err := w.findMachineImage(pool.MachineImage.Name, pool.MachineImage.Version)
		if err != nil {
			return err
		}
		machineImages = appendMachineImage(machineImages, api.MachineImage{
			Name:    pool.MachineImage.Name,
			Version: pool.MachineImage.Version,
			Image:   image,
		})

		var (
			deploymentName = fmt.Sprintf("%s-%s", w.worker.Namespace, pool.Name)
			className      = fmt.Sprintf("%s-%s", deploymentName, workerPoolHash)
		)

		machineClassSecrets = append(machineClassSecrets, &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      className,
				Namespace: w.worker.Namespace,
				Labels:    map[string]string{v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"userData": pool.UserData},
		})

		machineClasses = append(machineClasses, &machinev1alpha1.MachineClass{
			TypeMeta: metav1.TypeMeta{
				APIVersion: machinev1alpha1.SchemeGroupVersion.String(),
				Kind:       "MachineClass",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      className,
				Namespace: w.worker.Namespace,
			},
			SecretRef: &corev1.SecretReference{
				Name:      className,
				Namespace: w.worker.Namespace,
			},
			CredentialsSecretRef: &corev1.SecretReference{
				Name:      w.worker.Spec.SecretRef.Name,
				Namespace: w.worker.Spec.SecretRef.Namespace,
			},
			Provider:     local.Type,
			ProviderSpec: runtime.RawExtension{Raw: []byte(`{"image":"` + image + `"}`)},
		})

		machineDeployments = append(machineDeployments, worker.MachineDeployment{
			Name:                 deploymentName,
			ClassName:            className,
			SecretName:           className,
			Minimum:              pool.Minimum,
			Maximum:              pool.Maximum,
			MaxSurge:             pool.MaxSurge,
			MaxUnavailable:       pool.MaxUnavailable,
			Labels:               pool.Labels,
			Annotations:          pool.Annotations,
			Taints:               pool.Taints,
			MachineConfiguration: genericworkeractuator.ReadMachineConfiguration(pool),
		})
	}

	w.machineClassSecrets = machineClassSecrets
	w.machineClasses = machineClasses
	w.machineImages = machineImages
	w.machineDeployments = machineDeployments

	return nil
}

func (w *workerDelegate) PreReconcileHook(_ context.Context) error  { return nil }
func (w *workerDelegate) PostReconcileHook(_ context.Context) error { return nil }
func (w *workerDelegate) PreDeleteHook(_ context.Context) error     { return nil }
func (w *workerDelegate) PostDeleteHook(_ context.Context) error    { return nil }
