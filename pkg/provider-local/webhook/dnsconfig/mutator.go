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

package dnsconfig

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

type mutator struct {
	client client.Client
}

func (m *mutator) InjectClient(c client.Client) error {
	m.client = c
	return nil
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

		if newObj.GetLabels()["app"] == "dependency-watchdog-probe" {
			// We don't want to react for DWD pods but only for DWD deployments, so exit early here.
			return nil
		}

		podMeta = &obj.ObjectMeta
		podSpec = &obj.Spec

	case *appsv1.Deployment:
		podMeta = &obj.Spec.Template.ObjectMeta
		podSpec = &obj.Spec.Template.Spec

	case *appsv1.StatefulSet:
		podMeta = &obj.Spec.Template.ObjectMeta
		podSpec = &obj.Spec.Template.Spec

	default:
		return fmt.Errorf("unexpected object, got %T wanted *appsv1.Deployment, *appsv1.StatefulSet or *corev1.Pod", newObj)
	}

	service := &corev1.Service{}
	if err := m.client.Get(ctx, client.ObjectKey{Namespace: "gardener-extension-provider-local-coredns", Name: "coredns"}, service); err != nil {
		return err
	}

	metav1.SetMetaDataLabel(podMeta, local.LabelNetworkPolicyToIstioIngressGateway, v1beta1constants.LabelNetworkPolicyAllowed)
	injectDNSConfig(podSpec, newObj.GetNamespace(), service.Spec.ClusterIP)
	return nil
}

// injectDNSConfig changes the `.spec.dnsPolicy` and `.spec.dnsConfig` in the provided `podSpec`. Bascially, we
// configure the same options and search domains as the Kubernetes default behaviour. The only difference is that we use
// the gardener-extension-provider-local-coredns instead of the cluster's default DNS server. This is because this
// extension coredns can resolve the local domains (local.gardener.cloud). It otherwise forwards the traffic to the
// cluster's default DNS server.
func injectDNSConfig(podSpec *corev1.PodSpec, namespace, coreDNSClusterIP string) {
	podSpec.DNSPolicy = corev1.DNSNone
	podSpec.DNSConfig = &corev1.PodDNSConfig{
		Nameservers: []string{
			coreDNSClusterIP,
		},
		Searches: []string{
			fmt.Sprintf("%s.svc.%s", namespace, v1beta1.DefaultDomain),
			"svc." + v1beta1.DefaultDomain,
			v1beta1.DefaultDomain,
		},
		Options: []corev1.PodDNSConfigOption{{
			Name:  "ndots",
			Value: pointer.String("5"),
		}},
	}
}
