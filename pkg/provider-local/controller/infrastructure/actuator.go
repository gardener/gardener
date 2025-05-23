// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/provider-local/local"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type actuator struct {
	client client.Client
}

// NewActuator creates a new Actuator that updates the status of the handled Infrastructure resources.
func NewActuator(mgr manager.Manager) infrastructure.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	networkPolicyAllowMachinePods := emptyNetworkPolicy("allow-machine-pods", infrastructure.Namespace)
	networkPolicyAllowMachinePods.Spec = networkingv1.NetworkPolicySpec{
		Ingress: []networkingv1.NetworkPolicyIngressRule{{
			From: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "machine"}},
				},
			}},
		},
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "machine"}},
				},
				{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "registry"}},
					PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "registry"}},
				},
			},
		}},
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "machine"},
		},
		PolicyTypes: []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
			networkingv1.PolicyTypeEgress,
		},
	}

	if cluster.Shoot.Spec.Networking == nil || cluster.Shoot.Spec.Networking.Nodes == nil {
		return fmt.Errorf("shoot specification does not contain node network CIDR required for VPN tunnel")
	}

	objects := []client.Object{
		networkPolicyAllowMachinePods,
	}

	for _, ipFamily := range cluster.Shoot.Spec.Networking.IPFamilies {
		ipPoolObj, err := ipPool(infrastructure.Namespace, string(ipFamily), *cluster.Shoot.Spec.Networking.Nodes)
		if err != nil {
			return err
		}
		objects = append(objects, ipPoolObj)
	}

	for _, obj := range objects {
		if err := a.client.Patch(ctx, obj, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return err
		}
	}

	patch := client.MergeFrom(infrastructure.DeepCopy())
	infrastructure.Status.Networking = &extensionsv1alpha1.InfrastructureStatusNetworking{}
	if nodes := cluster.Shoot.Spec.Networking.Nodes; nodes != nil {
		infrastructure.Status.Networking.Nodes = []string{*nodes}
		// The egress CIDRs of local nodes are hard to define and depends on the traffic destination.
		// Traffic to the seed cluster will have nodes IPs as source IPs (i.e., the machine pod IPs).
		// Traffic to other containers in the kind network or the outside world will be NATed.
		// For now, we only report the nodes CIDR here to test the feature that propagates it back to the shoot status.
		infrastructure.Status.EgressCIDRs = []string{*nodes}
	}
	if pods := cluster.Shoot.Spec.Networking.Pods; pods != nil {
		infrastructure.Status.Networking.Pods = []string{*pods}
	}
	if services := cluster.Shoot.Spec.Networking.Services; services != nil {
		infrastructure.Status.Networking.Services = []string{*services}
	}
	return a.client.Status().Patch(ctx, infrastructure, patch)
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, _ *extensionscontroller.Cluster) error {
	return kubernetesutils.DeleteObjects(ctx, a.client,
		emptyNetworkPolicy("allow-machine-pods", infrastructure.Namespace),
		emptyNetworkPolicy("allow-to-istio-ingress-gateway", infrastructure.Namespace),
		emptyNetworkPolicy("allow-to-provider-local-coredns", infrastructure.Namespace),
		&metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{APIVersion: "crd.projectcalico.org/v1", Kind: "IPPool"}, ObjectMeta: metav1.ObjectMeta{Name: IPPoolName(infrastructure.Namespace, string(gardencorev1beta1.IPFamilyIPv4))}},
		&metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{APIVersion: "crd.projectcalico.org/v1", Kind: "IPPool"}, ObjectMeta: metav1.ObjectMeta{Name: IPPoolName(infrastructure.Namespace, string(gardencorev1beta1.IPFamilyIPv6))}},
	)
}

func (a *actuator) Migrate(ctx context.Context, log logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, infrastructure, cluster)
}

func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, infrastructure, cluster)
}

func (a *actuator) Restore(ctx context.Context, log logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, log, infrastructure, cluster)
}

func emptyNetworkPolicy(name, namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// IPPoolName returns the name of the crd.projectcalico.org/v1.IPPool resource for the given shoot namespace.
func IPPoolName(shootNamespace, ipFamily string) string {
	return "shoot-machine-pods-" + shootNamespace + "-" + strings.ToLower(ipFamily)
}

func ipPool(shootNamespace, ipFamily, nodeCIDR string) (client.Object, error) {
	return kubernetes.NewManifestReader([]byte(`apiVersion: crd.projectcalico.org/v1
kind: IPPool
metadata:
  name: ` + IPPoolName(shootNamespace, ipFamily) + `
spec:
  cidr: ` + nodeCIDR + `
  ipipMode: Always
  natOutgoing: true
  nodeSelector: "!all()" # Without this, calico defaults nodeSelector to "all()" and can randomly pick this pool for
                         # IPAM for pods even if the pod does not explicitly request an IP from this pool via the
                         # cni.projectcalico.org/IPv{4,6}Pools annotation.
                         # See https://github.com/projectcalico/calico/issues/7299#issuecomment-1446834103
`)).Read()
}
