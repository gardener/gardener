// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenerapiserver

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	"github.com/gardener/gardener/pkg/utils/secrets"
)

func (g *gardenerAPIServer) apiService(secretCAGardener *corev1.Secret, group, version string) *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s.%s", version, group),
			Labels: GetLabels(),
		},
		Spec: apiregistrationv1.APIServiceSpec{
			CABundle: secretCAGardener.Data[secrets.DataKeyCertificateBundle],
			Service: &apiregistrationv1.ServiceReference{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
			},
			Group:                group,
			Version:              version,
			GroupPriorityMinimum: 10000,
			VersionPriority:      20,
		},
	}
}
