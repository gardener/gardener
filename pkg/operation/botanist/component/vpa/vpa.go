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

package vpa

import (
	"context"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New creates a new instance of DeployWaiter for the Kubernetes Vertical Pod Autoscaler.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	v := &vpa{
		client:    client,
		namespace: namespace,
		values:    values,
	}

	if values.ClusterType == ClusterTypeSeed {
		v.registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	} else {
		v.registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	}

	return v
}

type vpa struct {
	client    client.Client
	namespace string
	values    Values

	registry *managedresources.Registry
}

// Values is a set of configuration values for the VPA components.
type Values struct {
	// ClusterType specifies the type of the cluster to which VPA is being deployed.
	// For seeds, all resources are being deployed as part of a ManagedResource (except for the CRDs - those must be
	// deployed separately because the VPA components themselves create VPA resources, hence the CRD must exist
	// beforehand).
	// For shoots, the VPA runs in the shoot namespace in the seed as part of the control plane. Hence, only the runtime
	// resources (like Deployment, Service, etc.) are being deployed directly (with the client). All other application-
	// related resources (like RBAC roles, CRD, etc.) are deployed as part of a ManagedResource.
	ClusterType clusterType

	// AdmissionController is a set of configuration values for the vpa-admission-controller.
	AdmissionController ValuesAdmissionController
	// Exporter is a set of configuration values for the vpa-exporter.
	Exporter ValuesExporter
	// Recommender is a set of configuration values for the vpa-recommender.
	Recommender ValuesRecommender
	// Updater is a set of configuration values for the vpa-updater.
	Updater ValuesUpdater
}

type clusterType string

const (
	// ClusterTypeSeed is a constant for the 'seed' cluster type.
	ClusterTypeSeed clusterType = "seed"
	// ClusterTypeShoot is a constant for the 'shoot' cluster type.
	ClusterTypeShoot clusterType = "shoot"
)

func (v *vpa) Deploy(ctx context.Context) error {
	allResources := mergeResourceConfigs(
		v.admissionControllerResourceConfigs(),
		v.exporterResourceConfigs(),
		v.recommenderResourceConfigs(),
		v.updaterResourceConfigs(),
		v.generalResourceConfigs(),
	)

	if v.values.ClusterType == ClusterTypeSeed {
		for _, r := range allResources {
			r.mutateFn()

			if err := v.registry.Add(r.obj); err != nil {
				return err
			}
		}

		if err := managedresources.CreateForSeed(ctx, v.client, v.namespace, v.managedResourceName(), false, v.registry.SerializedObjects()); err != nil {
			return err
		}

		// TODO(rfranzke): Remove in a future release.
		return kutil.DeleteObjects(ctx, v.client,
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:exporter"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:exporter"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:evictioner"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:evictioner"}},
		)
	}

	for _, name := range []string{
		v1beta1constants.DeploymentNameVPAUpdater,
	} {
		if err := gutil.NewShootAccessSecret(name, v.namespace).Reconcile(ctx, v.client); err != nil {
			return err
		}
	}

	for _, r := range allResources {
		switch r.class {
		case application:
			r.mutateFn()
			if err := v.registry.Add(r.obj); err != nil {
				return err
			}

		case runtime:
			if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, r.obj, func() error {
				r.mutateFn()
				return nil
			}); err != nil {
				return err
			}
		}
	}

	return managedresources.CreateForShoot(ctx, v.client, v.namespace, v.managedResourceName(), false, v.registry.SerializedObjects())
}

func (v *vpa) Destroy(ctx context.Context) error {
	if v.values.ClusterType == ClusterTypeSeed {
		return managedresources.DeleteForSeed(ctx, v.client, v.namespace, v.managedResourceName())
	}

	if err := managedresources.DeleteForShoot(ctx, v.client, v.namespace, v.managedResourceName()); err != nil {
		return err
	}

	return kutil.DeleteObjects(ctx, v.client, allRuntimeObjects(
		v.admissionControllerResourceConfigs(),
		v.exporterResourceConfigs(),
		v.recommenderResourceConfigs(),
		v.updaterResourceConfigs(),
		v.generalResourceConfigs(),
	)...)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (v *vpa) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, v.client, v.namespace, v.managedResourceName())
}

func (v *vpa) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, v.client, v.namespace, v.managedResourceName())
}

func (v *vpa) managedResourceName() string {
	if v.values.ClusterType == ClusterTypeSeed {
		return "vpa"
	}
	return "shoot-core-vpa"
}

func (v *vpa) emptyService(name string) *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyServiceAccount(name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: v.rbacNamePrefix() + name}}
}

func (v *vpa) emptyClusterRoleBinding(name string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: v.rbacNamePrefix() + name}}
}

func (v *vpa) emptyDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyVerticalPodAutoscaler(name string) *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) rbacNamePrefix() string {
	prefix := "gardener.cloud:vpa:"

	if v.values.ClusterType == ClusterTypeSeed {
		return prefix + "source:"
	}

	return prefix + "target:"
}

func (v *vpa) serviceAccountNamespace() string {
	if v.values.ClusterType == ClusterTypeSeed {
		return v.namespace
	}
	return metav1.NamespaceSystem
}

func getAppLabel(appValue string) map[string]string {
	return map[string]string{v1beta1constants.LabelApp: appValue}
}

func getRoleLabel() map[string]string {
	return map[string]string{v1beta1constants.GardenRole: "vpa"}
}

func getAllLabels(appValue string) map[string]string {
	return utils.MergeStringMaps(getAppLabel(appValue), getRoleLabel())
}

type resourceConfig struct {
	obj      client.Object
	class    class
	mutateFn func()
}

type class uint8

const (
	runtime class = iota
	application
)

type resourceConfigs []resourceConfig

func (r resourceConfigs) allRuntimeObjects() []client.Object {
	var out []client.Object

	for _, o := range r {
		if o.class == runtime {
			out = append(out, o.obj)
		}
	}

	return out
}

func allRuntimeObjects(configsLists ...resourceConfigs) []client.Object {
	var out []client.Object

	for _, list := range configsLists {
		out = append(out, list.allRuntimeObjects()...)
	}

	return out
}

func mergeResourceConfigs(configsLists ...resourceConfigs) resourceConfigs {
	var out resourceConfigs

	for _, list := range configsLists {
		out = append(out, list...)
	}

	return out
}
