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

package kubeapiserver

import (
	"context"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	configMapAuditPolicyNamePrefix = "audit-policy-config"
	configMapAuditPolicyDataKey    = "audit-policy.yaml"
)

func (k *kubeAPIServer) emptyConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileConfigMapAuditPolicy(ctx context.Context, configMap *corev1.ConfigMap) error {
	policy := `---
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: None
`

	if k.values.Audit != nil && k.values.Audit.Policy != nil {
		policy = *k.values.Audit.Policy
	}

	configMap.Data = map[string]string{configMapAuditPolicyDataKey: policy}
	utilruntime.Must(kutil.MakeUnique(configMap))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}
