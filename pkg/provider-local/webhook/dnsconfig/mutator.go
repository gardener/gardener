// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsconfig

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

type mutator struct {
	client client.Client
}

func (m *mutator) Mutate(ctx context.Context, newObj, oldObj client.Object) error {
	var (
		podMeta *metav1.ObjectMeta
		podSpec *corev1.PodSpec
	)

	if newObj.GetDeletionTimestamp() != nil {
		return nil
	}

	switch obj := newObj.(type) {
	case *corev1.Pod:
		if oldObj != nil {
			// This is basically a hack - ideally, we would like the mutating webhook configuration to only react for CREATE
			// operations. However, currently both "CREATE" and "UPDATE" are hard-coded in the extensions library.
			return nil
		}

		if newObj.GetLabels()["app"] == "dependency-watchdog-prober" {
			// We don't want to react for DWD pods but only for DWD deployments, so exit early here.
			return nil
		}

		podMeta = &obj.ObjectMeta
		podSpec = &obj.Spec

		if newObj.GetLabels()["app"] == "machine" && newObj.GetLabels()["machine-provider"] == "local" {
			metav1.SetMetaDataLabel(podMeta, "networking.resources.gardener.cloud/to-all-istio-ingresses-istio-ingressgateway-tcp-9443", v1beta1constants.LabelNetworkPolicyAllowed)
		}

	case *appsv1.Deployment:
		podMeta = &obj.Spec.Template.ObjectMeta
		podSpec = &obj.Spec.Template.Spec

	default:
		return fmt.Errorf("unexpected object, got %T wanted *appsv1.Deployment or *corev1.Pod", newObj)
	}

	service := &corev1.Service{}
	if err := m.client.Get(ctx, client.ObjectKey{Namespace: "gardener-extension-provider-local-coredns", Name: "coredns"}, service); err != nil {
		return err
	}

	metav1.SetMetaDataLabel(podMeta, local.LabelNetworkPolicyToIstioIngressGateway, v1beta1constants.LabelNetworkPolicyAllowed)
	injectDNSConfig(podSpec, newObj.GetNamespace(), service.Spec.ClusterIPs)
	return nil
}

// injectDNSConfig changes the `.spec.dnsPolicy` and `.spec.dnsConfig` in the provided `podSpec`. Basically, we
// configure the same options and search domains as the Kubernetes default behaviour. The only difference is that we use
// the gardener-extension-provider-local-coredns instead of the cluster's default DNS server. This is because this
// extension coredns can resolve the local domains (local.gardener.cloud). It otherwise forwards the traffic to the
// cluster's default DNS server.
func injectDNSConfig(podSpec *corev1.PodSpec, namespace string, coreDNSClusterIPs []string) {
	podSpec.DNSPolicy = corev1.DNSNone
	podSpec.DNSConfig = &corev1.PodDNSConfig{
		Nameservers: coreDNSClusterIPs,
		Searches: []string{
			fmt.Sprintf("%s.svc.%s", namespace, v1beta1.DefaultDomain),
			"svc." + v1beta1.DefaultDomain,
			v1beta1.DefaultDomain,
		},
		Options: []corev1.PodDNSConfigOption{{
			Name:  "ndots",
			Value: ptr.To("5"),
		}},
	}
}
