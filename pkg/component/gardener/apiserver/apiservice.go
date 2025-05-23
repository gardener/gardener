// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

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
