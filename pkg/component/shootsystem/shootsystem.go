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

package shootsystem

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	corednsconstants "github.com/gardener/gardener/pkg/component/coredns/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/nodelocaldns/constants"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
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

	return managedresources.CreateForShoot(ctx, s.client, s.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, data)
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

		port53      = intstr.FromInt(53)
		port443     = intstr.FromInt(kubeapiserverconstants.Port)
		port8053    = intstr.FromInt(corednsconstants.PortServer)
		port10250   = intstr.FromInt(10250)
		protocolUDP = corev1.ProtocolUDP
		protocolTCP = corev1.ProtocolTCP

		networkPolicyAllowToShootAPIServer = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-to-apiserver",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: fmt.Sprintf("Allows traffic to the API server in TCP "+
						"port 443 for pods labeled with '%s=%s'.", v1beta1constants.LabelNetworkPolicyShootToAPIServer,
						v1beta1constants.LabelNetworkPolicyAllowed),
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyShootToAPIServer: v1beta1constants.LabelNetworkPolicyAllowed}},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{Ports: []networkingv1.NetworkPolicyPort{{Port: &port443, Protocol: &protocolTCP}}}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			},
		}
		networkPolicyAllowToDNS = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-to-dns",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: fmt.Sprintf("Allows egress traffic from pods labeled "+
						"with '%s=%s' to DNS running in the '%s' namespace.", v1beta1constants.LabelNetworkPolicyToDNS,
						v1beta1constants.LabelNetworkPolicyAllowed, metav1.NamespaceSystem),
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					{
						To: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      corednsconstants.LabelKey,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{corednsconstants.LabelValue},
								}},
							},
						}},
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: &protocolUDP, Port: &port8053},
							{Protocol: &protocolTCP, Port: &port8053},
						},
					},
					// this allows Pods with 'dnsPolicy: Default' to talk to the node's DNS provider.
					{
						To: []networkingv1.NetworkPolicyPeer{
							{
								IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"},
							},
							{
								PodSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      corednsconstants.LabelKey,
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{nodelocaldnsconstants.LabelValue},
									}},
								},
							},
						},
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: &protocolUDP, Port: &port53},
							{Protocol: &protocolTCP, Port: &port53},
						},
					},
				},
			},
		}
		networkPolicyAllowToKubelet = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-to-kubelet",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: fmt.Sprintf("Allows egress traffic to kubelet in TCP "+
						"port 10250 for pods labeled with '%s=%s'.", v1beta1constants.LabelNetworkPolicyShootToKubelet,
						v1beta1constants.LabelNetworkPolicyAllowed),
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyShootToKubelet: v1beta1constants.LabelNetworkPolicyAllowed}},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{Ports: []networkingv1.NetworkPolicyPort{{Port: &port10250, Protocol: &protocolTCP}}}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			},
		}
		networkPolicyAllowToPublicNetworks = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-to-public-networks",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: fmt.Sprintf("Allows egress traffic to all networks for "+
						"pods labeled with '%s=%s'.", v1beta1constants.LabelNetworkPolicyToPublicNetworks,
						v1beta1constants.LabelNetworkPolicyAllowed),
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed}},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}}}}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			},
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

	if err := addPriorityClasses(registry); err != nil {
		return nil, err
	}

	return registry.AddAllAndSerialize(
		shootInfoConfigMap,
		networkPolicyAllowToShootAPIServer,
		networkPolicyAllowToDNS,
		networkPolicyAllowToKubelet,
		networkPolicyAllowToPublicNetworks,
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
		"endpointslicemirroring-controller",
		"ephemeral-volume-controller",
		"storage-version-garbage-collector",
		"node-controller",
		"route-controller",
		"service-controller",
	}

	if versionutils.ConstraintK8sGreaterEqual126.Check(s.values.Shoot.KubernetesVersion) {
		kubeControllerManagerServiceAccountNames = append(kubeControllerManagerServiceAccountNames,
			"resource-claim-controller")
	}

	if versionutils.ConstraintK8sGreaterEqual128.Check(s.values.Shoot.KubernetesVersion) {
		kubeControllerManagerServiceAccountNames = append(kubeControllerManagerServiceAccountNames,
			"legacy-service-account-token-cleaner",
			"validatingadmissionpolicy-status-controller")
	}

	return append(kubeControllerManagerServiceAccountNames, "default")
}

// remember to update docs/development/priority-classes.md when making changes here
var gardenletManagedPriorityClasses = []struct {
	name        string
	value       int32
	description string
}{
	{v1beta1constants.PriorityClassNameShootSystem900, 999999900, "PriorityClass for Shoot system components"},
	{v1beta1constants.PriorityClassNameShootSystem800, 999999800, "PriorityClass for Shoot system components"},
	{v1beta1constants.PriorityClassNameShootSystem700, 999999700, "PriorityClass for Shoot system components"},
	{v1beta1constants.PriorityClassNameShootSystem600, 999999600, "PriorityClass for Shoot system components"},
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

	extensions := make([]string, 0, len(s.values.Shoot.Components.Extensions.Extension.Extensions()))
	for extensionType := range s.values.Shoot.Components.Extensions.Extension.Extensions() {
		extensions = append(extensions, extensionType)
	}
	slices.Sort(extensions)
	data["extensions"] = strings.Join(extensions, ",")

	return data
}
