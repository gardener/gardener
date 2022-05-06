// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/local"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mutator struct {
	client client.Client
}

func (m *mutator) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

func (m *mutator) Mutate(ctx context.Context, newObj, oldObj client.Object) error {
	if oldObj != nil {
		// This is basically a hack - ideally, we would like the mutating webhook configuration to only react for CREATE
		// operations. However, currently both "CREATE" and "UPDATE" are hard-coded in the extensions library.
		return nil
	}

	pod, ok := newObj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("unexpected object, got %T wanted *corev1.Pod", newObj)
	}

	service := &corev1.Service{}
	if err := m.client.Get(ctx, client.ObjectKey{Namespace: "gardener-extension-provider-local-coredns", Name: "coredns"}, service); err != nil {
		return err
	}

	metav1.SetMetaDataLabel(&pod.ObjectMeta, local.LabelNetworkPolicyToIstioIngressGateway, v1beta1constants.LabelNetworkPolicyAllowed)
	pod.Spec.DNSPolicy = corev1.DNSNone
	pod.Spec.DNSConfig = &corev1.PodDNSConfig{
		Nameservers: []string{
			service.Spec.ClusterIP,
		},
		Searches: []string{
			fmt.Sprintf("%s.svc.%s", newObj.GetNamespace(), v1beta1.DefaultDomain),
			"svc." + v1beta1.DefaultDomain,
			v1beta1.DefaultDomain,
		},
		Options: []corev1.PodDNSConfigOption{{
			Name:  "ndots",
			Value: pointer.String("5"),
		}},
	}
	return nil
}
