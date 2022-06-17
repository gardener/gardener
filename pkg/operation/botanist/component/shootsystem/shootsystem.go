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

package shootsystem

import (
	"context"
	"sort"
	"strings"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "shoot-core-system"

// Values is a set of configuration values for the system resources.
type Values struct {
	// ProjectName is the name of the project of the shoot cluster.
	ProjectName string
	// Shoot is an object containing information about the shoot cluster.
	Shoot *shootpkg.Shoot
}

// New creates a new instance of DeployWaiter for shoot system resources.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &shootSystem{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type shootSystem struct {
	client    client.Client
	namespace string
	values    Values
}

func (s *shootSystem) Deploy(ctx context.Context) error {
	data, err := s.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, s.client, s.namespace, ManagedResourceName, false, data)
}

func (s *shootSystem) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, s.client, s.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (s *shootSystem) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, s.client, s.namespace, ManagedResourceName)
}

func (s *shootSystem) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, s.client, s.namespace, ManagedResourceName)
}

func (s *shootSystem) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		shootInfoConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.ConfigMapNameShootInfo,
				Namespace: metav1.NamespaceSystem,
			},
			Data: s.shootInfoData(),
		}
	)

	for _, name := range s.getServiceAccountNamesToInvalidate() {
		if err := registry.Add(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.KeepObject: "true"},
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}); err != nil {
			return nil, err
		}
	}

	return registry.AddAllAndSerialize(
		shootInfoConfigMap,
	)
}

func (s *shootSystem) getServiceAccountNamesToInvalidate() []string {
	// Well-known {kube,cloud}-controller-manager controllers using a token for ServiceAccounts in the shoot
	// To maintain this list for each new Kubernetes version:
	// * Run hack/compare-k8s-controllers.sh <old-version> <new-version> (e.g. 'hack/compare-k8s-controllers.sh 1.22 1.23').
	//   It will present 2 lists of controllers: those added and those removed in <new-version> compared to <old-version>.
	// * Double check whether such ServiceAccount indeed appears in the kube-system namespace when creating a cluster
	//   with <new-version>. Note that it sometimes might be hidden behind a default-off feature gate.
	//   If it appears, add all added controllers to the list if the Kubernetes version is high enough.
	// * For any removed controllers, add them only to the Kubernetes version if it is low enough.
	kubeControllerManagerServiceAccountNames := []string{
		"attachdetach-controller",
		"bootstrap-signer",
		"certificate-controller",
		"clusterrole-aggregation-controller",
		"controller-discovery",
		"cronjob-controller",
		"daemon-set-controller",
		"deployment-controller",
		"disruption-controller",
		"endpoint-controller",
		"endpointslice-controller",
		"expand-controller",
		"generic-garbage-collector",
		"horizontal-pod-autoscaler",
		"job-controller",
		"metadata-informers",
		"namespace-controller",
		"persistent-volume-binder",
		"pod-garbage-collector",
		"pv-protection-controller",
		"pvc-protection-controller",
		"replicaset-controller",
		"replication-controller",
		"resourcequota-controller",
		"root-ca-cert-publisher",
		"service-account-controller",
		"shared-informers",
		"statefulset-controller",
		"token-cleaner",
		"tokens-controller",
		"ttl-after-finished-controller",
		"ttl-controller",
	}

	if versionutils.ConstraintK8sGreaterEqual119.Check(s.values.Shoot.KubernetesVersion) {
		kubeControllerManagerServiceAccountNames = append(kubeControllerManagerServiceAccountNames,
			"endpointslicemirroring-controller",
			"ephemeral-volume-controller",
		)
	}

	if versionutils.ConstraintK8sGreaterEqual120.Check(s.values.Shoot.KubernetesVersion) {
		kubeControllerManagerServiceAccountNames = append(kubeControllerManagerServiceAccountNames,
			"storage-version-garbage-collector",
		)
	}

	// The cloud-controller-manager library was only adapted beginning with Kubernetes 1.21 to not rely on the static
	// ServiceAccount secrets anymore. Prior versions still need them, so let's add the ServiceAccount names for
	// controllers which are part of cloud-controller-managers only for 1.21+.
	// See https://github.com/kubernetes/kubernetes/pull/99291 for more details.
	if versionutils.ConstraintK8sGreaterEqual121.Check(s.values.Shoot.KubernetesVersion) {
		kubeControllerManagerServiceAccountNames = append(kubeControllerManagerServiceAccountNames,
			"node-controller",
			"route-controller",
			"service-controller",
		)
	}

	return append(kubeControllerManagerServiceAccountNames, "default")
}

func (s *shootSystem) shootInfoData() map[string]string {
	data := map[string]string{
		"projectName":       s.values.ProjectName,
		"shootName":         s.values.Shoot.GetInfo().Name,
		"provider":          s.values.Shoot.GetInfo().Spec.Provider.Type,
		"region":            s.values.Shoot.GetInfo().Spec.Region,
		"kubernetesVersion": s.values.Shoot.GetInfo().Spec.Kubernetes.Version,
		"podNetwork":        s.values.Shoot.Networks.Pods.String(),
		"serviceNetwork":    s.values.Shoot.Networks.Services.String(),
		"maintenanceBegin":  s.values.Shoot.GetInfo().Spec.Maintenance.TimeWindow.Begin,
		"maintenanceEnd":    s.values.Shoot.GetInfo().Spec.Maintenance.TimeWindow.End,
	}

	if domain := s.values.Shoot.ExternalClusterDomain; domain != nil {
		data["domain"] = *domain
	}

	if nodeNetwork := s.values.Shoot.GetInfo().Spec.Networking.Nodes; nodeNetwork != nil {
		data["nodeNetwork"] = *nodeNetwork
	}

	var extensions []string
	for extensionType := range s.values.Shoot.Components.Extensions.Extension.Extensions() {
		extensions = append(extensions, extensionType)
	}
	sort.Strings(extensions)
	data["extensions"] = strings.Join(extensions, ",")

	return data
}
