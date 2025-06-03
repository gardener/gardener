// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	corednsconstants "github.com/gardener/gardener/pkg/component/networking/coredns/constants"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/networking/nodelocaldns/constants"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	netutils "github.com/gardener/gardener/pkg/utils/net"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "shoot-core-system"

// Interface is an interface for managing shoot system resources.
type Interface interface {
	component.DeployWaiter
	SetAPIResourceList([]*metav1.APIResourceList)
	SetPodNetworkCIDRs([]net.IPNet)
	SetServiceNetworkCIDRs([]net.IPNet)
	SetNodeNetworkCIDRs([]net.IPNet)
	SetEgressCIDRs([]net.IPNet)
}

// Values is a set of configuration values for the system resources.
type Values struct {
	// APIResourceList is the list of available API resources in the shoot cluster.
	APIResourceList []*metav1.APIResourceList
	// Extensions is the list of the extension types.
	Extensions []string
	// ExternalClusterDomain is the external domain of the cluster.
	ExternalClusterDomain *string
	// IsWorkerless specifies whether the cluster has worker nodes.
	IsWorkerless bool
	// KubernetesVersion is the version of the cluster.
	KubernetesVersion *semver.Version
	// EncryptedResources is the list of resources which are encrypted by the kube-apiserver.
	EncryptedResources []string
	// Object is the shoot object.
	Object *gardencorev1beta1.Shoot
	// PodNetworkCIDRs are the CIDRs of the pod network.
	PodNetworkCIDRs []net.IPNet
	// ProjectName is the name of the project of the cluster.
	ProjectName string
	// ServiceNetworkCIDRs are the CIDRs of the service network.
	ServiceNetworkCIDRs []net.IPNet
	// NodeNetworkCIDRs are the CIDRs of the node network.
	NodeNetworkCIDRs []net.IPNet
	// EgressCIDRs are the egress CIDRs of the cluster, actual presence of this field depends on the implementation of the provider extension.
	EgressCIDRs []net.IPNet
}

// New creates a new instance of DeployWaiter for shoot system resources.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
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

func (s *shootSystem) SetAPIResourceList(list []*metav1.APIResourceList) {
	s.values.APIResourceList = list
}

func (s *shootSystem) SetPodNetworkCIDRs(pods []net.IPNet) {
	s.values.PodNetworkCIDRs = pods
}

func (s *shootSystem) SetServiceNetworkCIDRs(services []net.IPNet) {
	s.values.ServiceNetworkCIDRs = services
}

func (s *shootSystem) SetNodeNetworkCIDRs(nodes []net.IPNet) {
	s.values.NodeNetworkCIDRs = nodes
}

func (s *shootSystem) SetEgressCIDRs(cidrs []net.IPNet) {
	s.values.EgressCIDRs = cidrs
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
	registry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

	if versionutils.ConstraintK8sGreaterEqual133.Check(s.values.KubernetesVersion) {
		networkPolicyDenyAll := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--deny-all",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: "Disables all ingress and egress traffic into/from this namespace.",
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				Egress:      []networkingv1.NetworkPolicyEgressRule{},
				Ingress:     []networkingv1.NetworkPolicyIngressRule{},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				},
			},
		}

		if err := registry.Add(networkPolicyDenyAll); err != nil {
			return nil, err
		}
	}

	if !s.values.IsWorkerless {
		var (
			shootInfoConfigMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.ConfigMapNameShootInfo,
					Namespace: metav1.NamespaceSystem,
				},
				Data: s.shootInfoData(),
			}

			port53      = intstr.FromInt32(53)
			port443     = intstr.FromInt32(kubeapiserverconstants.Port)
			port8053    = intstr.FromInt32(corednsconstants.PortServer)
			port10250   = intstr.FromInt32(10250)
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
									IPBlock: &networkingv1.IPBlock{CIDR: "::/0"},
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
					Egress: []networkingv1.NetworkPolicyEgressRule{{To: []networkingv1.NetworkPolicyPeer{
						{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
						{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}},
					}}},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				},
			}
		)

		if err := registry.Add(
			shootInfoConfigMap,
			networkPolicyAllowToShootAPIServer,
			networkPolicyAllowToDNS,
			networkPolicyAllowToKubelet,
			networkPolicyAllowToPublicNetworks,
		); err != nil {
			return nil, err
		}

		if err := registry.Add(priorityClassResources()...); err != nil {
			return nil, err
		}
	}

	if len(s.values.APIResourceList) > 0 {
		if err := registry.Add(s.readOnlyRBACResources()...); err != nil {
			return nil, err
		}
	}

	return registry.SerializedObjects()
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

func priorityClassResources() []client.Object {
	out := make([]client.Object, 0, len(gardenletManagedPriorityClasses))

	for _, class := range gardenletManagedPriorityClasses {
		out = append(out, &schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: class.name,
			},
			Description:   class.description,
			GlobalDefault: false,
			Value:         class.value,
		})
	}

	return out
}

func (s *shootSystem) shootInfoData() map[string]string {
	data := map[string]string{
		"extensions":        strings.Join(s.values.Extensions, ","),
		"projectName":       s.values.ProjectName,
		"shootName":         s.values.Object.Name,
		"provider":          s.values.Object.Spec.Provider.Type,
		"region":            s.values.Object.Spec.Region,
		"kubernetesVersion": s.values.Object.Spec.Kubernetes.Version,
		"podNetwork":        s.values.PodNetworkCIDRs[0].String(),
		"serviceNetwork":    s.values.ServiceNetworkCIDRs[0].String(),
		"maintenanceBegin":  s.values.Object.Spec.Maintenance.TimeWindow.Begin,
		"maintenanceEnd":    s.values.Object.Spec.Maintenance.TimeWindow.End,
	}

	if domain := s.values.ExternalClusterDomain; domain != nil {
		data["domain"] = *domain
	}

	if len(s.values.NodeNetworkCIDRs) > 0 {
		data["nodeNetwork"] = s.values.NodeNetworkCIDRs[0].String()
	}

	addNetworkToMap("podNetworks", s.values.PodNetworkCIDRs, data)
	addNetworkToMap("serviceNetworks", s.values.ServiceNetworkCIDRs, data)
	addNetworkToMap("nodeNetworks", s.values.NodeNetworkCIDRs, data)
	addNetworkToMap("egressCIDRs", s.values.EgressCIDRs, data)

	return data
}

func (s *shootSystem) readOnlyRBACResources() []client.Object {
	allowedSubResources := map[string]map[string][]string{
		corev1.GroupName: {
			"pods": {"log"},
		},
	}

	apiGroupToReadableResourcesNames := make(map[string][]string, len(s.values.APIResourceList))
	for _, api := range s.values.APIResourceList {
		apiGroup := strings.Split(api.GroupVersion, "/")[0]
		if apiGroup == corev1.SchemeGroupVersion.Version {
			apiGroup = corev1.GroupName
		}

		for _, resource := range api.APIResources {
			// We don't want to include privileges for reading encrypted resources.
			if s.isEncryptedResource(resource.Name, apiGroup) {
				continue
			}

			// We don't want to include privileges for resources which are not readable.
			if !slices.ContainsFunc(resource.Verbs, func(verb string) bool {
				return verb == "get" || verb == "list"
			}) {
				continue
			}

			apiGroupToReadableResourcesNames[apiGroup] = append(apiGroupToReadableResourcesNames[apiGroup], resource.Name)

			if resourceToSubResources, ok := allowedSubResources[apiGroup]; ok {
				for _, subResource := range resourceToSubResources[resource.Name] {
					apiGroupToReadableResourcesNames[apiGroup] = append(apiGroupToReadableResourcesNames[apiGroup], resource.Name+"/"+subResource)
				}
			}
		}

		// Sort keys to get a stable order of the RBAC rules when iterating.
		slices.Sort(apiGroupToReadableResourcesNames[apiGroup])
	}

	// Sort keys to get a stable order of the RBAC rules when iterating.
	allAPIGroups := make([]string, 0, len(apiGroupToReadableResourcesNames))
	for key := range apiGroupToReadableResourcesNames {
		allAPIGroups = append(allAPIGroups, key)
	}
	slices.Sort(allAPIGroups)

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener.cloud:system:read-only",
		},
		Rules: make([]rbacv1.PolicyRule, 0, len(allAPIGroups)),
	}

	for _, apiGroup := range allAPIGroups {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{apiGroup},
			Resources: apiGroupToReadableResourcesNames[apiGroup],
			Verbs:     []string{"get", "list", "watch"},
		})
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "gardener.cloud:system:read-only",
			Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{{
			Kind: rbacv1.GroupKind,
			Name: v1beta1constants.ShootGroupViewers,
		}},
	}

	return []client.Object{clusterRole, clusterRoleBinding}
}

func (s *shootSystem) isEncryptedResource(resource, group string) bool {
	resourceName := fmt.Sprintf("%s.%s", resource, group)

	if group == corev1.SchemeGroupVersion.Group {
		resourceName = resource
	}

	return slices.Contains(s.values.EncryptedResources, resourceName)
}

func addNetworkToMap(name string, cidrs []net.IPNet, data map[string]string) {
	networks := netutils.JoinByComma(cidrs)
	if networks != "" {
		data[name] = networks
	}
}
