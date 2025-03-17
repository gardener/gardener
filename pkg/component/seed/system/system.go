// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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
	// Configs configures additional excess capacity reservation deployments for shoot control planes in the seed.
	Configs []gardencorev1beta1.SeedSettingExcessCapacityReservationConfig
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
	var registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	if s.values.ReserveExcessCapacity.Enabled {
		for i, config := range s.values.ReserveExcessCapacity.Configs {
			name := fmt.Sprintf("reserve-excess-capacity-%d", i)
			if err := s.addReserveExcessCapacityDeployment(registry, name, config); err != nil {
				return nil, err
			}
		}
	}

	if err := addPriorityClasses(registry); err != nil {
		return nil, err
	}

	return registry.SerializedObjects()
}

func (s *seedSystem) addReserveExcessCapacityDeployment(registry *managedresources.Registry, name string, config gardencorev1beta1.SeedSettingExcessCapacityReservationConfig) error {
	return registry.Add(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: s.namespace,
				Labels:    getExcessCapacityReservationLabels(),
				Annotations: map[string]string{
					resourcesv1alpha1.SkipHealthCheck: "true",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             &s.values.ReserveExcessCapacity.Replicas,
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector:             &metav1.LabelSelector{MatchLabels: getExcessCapacityReservationLabels()},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getExcessCapacityReservationLabels(),
					},
					Spec: corev1.PodSpec{
						TerminationGracePeriodSeconds: ptr.To[int64](5),
						Containers: []corev1.Container{{
							Name:            "pause-container",
							Image:           s.values.ReserveExcessCapacity.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Resources: corev1.ResourceRequirements{
								Requests: config.Resources,
								Limits:   config.Resources,
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
							},
						}},
						NodeSelector:      config.NodeSelector,
						PriorityClassName: v1beta1constants.PriorityClassNameReserveExcessCapacity,
						Tolerations:       config.Tolerations,
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
