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

package seedsystem

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "system"

// Values is a set of configuration values for the system resources.
type Values struct {
	// ReserveExcessCapacity contains configuration for the deployment of the excess capacity reservation resources.
	ReserveExcessCapacity ReserveExcessCapacityValues
}

// ReserveExcessCapacityValues contains configuration for the deployment of the excess capacity reservation resources.
type ReserveExcessCapacityValues struct {
	// Enabled specifies whether excess capacity reservation should be enabled.
	Enabled bool
	// Image is the container image.
	Image string
	// Replicas is the number of replicas.
	Replicas int32
}

// New creates a new instance of DeployWaiter for seed system resources.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &seedSystem{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type seedSystem struct {
	client    client.Client
	namespace string
	values    Values
}

func (s *seedSystem) Deploy(ctx context.Context) error {
	data, err := s.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, s.client, s.namespace, ManagedResourceName, false, data)
}

func (s *seedSystem) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, s.client, s.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (s *seedSystem) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, s.client, s.namespace, ManagedResourceName)
}

func (s *seedSystem) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, s.client, s.namespace, ManagedResourceName)
}

func (s *seedSystem) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	)

	if s.values.ReserveExcessCapacity.Enabled {
		if err := s.addReserveExcessCapacityDeployment(registry); err != nil {
			return nil, err
		}
	}

	if err := addPriorityClasses(registry); err != nil {
		return nil, err
	}

	return registry.SerializedObjects(), nil
}

func (s *seedSystem) addReserveExcessCapacityDeployment(registry *managedresources.Registry) error {
	return registry.Add(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reserve-excess-capacity",
			Namespace: s.namespace,
			Labels:    getExcessCapacityReservationLabels(),
			Annotations: map[string]string{
				resourcesv1alpha1.SkipHealthCheck: "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &s.values.ReserveExcessCapacity.Replicas,
			RevisionHistoryLimit: pointer.Int32(2),
			Selector:             &metav1.LabelSelector{MatchLabels: getExcessCapacityReservationLabels()},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: getExcessCapacityReservationLabels(),
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: pointer.Int64(5),
					Containers: []corev1.Container{{
						Name:            "pause-container",
						Image:           s.values.ReserveExcessCapacity.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Resources: corev1.ResourceRequirements{
							// This roughly corresponds to a single, moderately large control-plane.
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("6Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("6Gi"),
							},
						},
					}},
					PriorityClassName: v1beta1constants.PriorityClassNameReserveExcessCapacity,
				},
			},
		},
	})
}

// remember to update docs/development/priority-classes.md when making changes here
var gardenletManagedPriorityClasses = []struct {
	name        string
	value       int32
	description string
}{
	{v1beta1constants.PriorityClassNameSeedSystem900, 999998900, "PriorityClass for Seed system components"},
	{v1beta1constants.PriorityClassNameSeedSystem800, 999998800, "PriorityClass for Seed system components"},
	{v1beta1constants.PriorityClassNameSeedSystem700, 999998700, "PriorityClass for Seed system components"},
	{v1beta1constants.PriorityClassNameSeedSystem600, 999998600, "PriorityClass for Seed system components"},
	{v1beta1constants.PriorityClassNameReserveExcessCapacity, -5, "PriorityClass for reserving excess capacity on a Seed cluster"},
	{v1beta1constants.PriorityClassNameShootControlPlane500, 999998500, "PriorityClass for Shoot control plane components"},
	{v1beta1constants.PriorityClassNameShootControlPlane400, 999998400, "PriorityClass for Shoot control plane components"},
	{v1beta1constants.PriorityClassNameShootControlPlane300, 999998300, "PriorityClass for Shoot control plane components"},
	{v1beta1constants.PriorityClassNameShootControlPlane200, 999998200, "PriorityClass for Shoot control plane components"},
	{v1beta1constants.PriorityClassNameShootControlPlane100, 999998100, "PriorityClass for Shoot control plane components"},
}

func addPriorityClasses(registry *managedresources.Registry) error {
	for _, class := range gardenletManagedPriorityClasses {
		if err := registry.Add(&schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: class.name,
			},
			Description:   class.description,
			GlobalDefault: false,
			Value:         class.value,
		}); err != nil {
			return err
		}
	}

	return nil
}

func getExcessCapacityReservationLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: "reserve-excess-capacity",
	}
}
