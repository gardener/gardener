// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenercustommetrics

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"
)

func (gcmx *gardenerCustomMetrics) apiService() *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "v1beta2.custom.metrics.k8s.io",
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Name:      serviceName,
				Namespace: gcmx.namespace,
				Port:      ptr.To[int32](443),
			},
			Group:                "custom.metrics.k8s.io",
			Version:              "v1beta2",
			GroupPriorityMinimum: 100,
			VersionPriority:      200,
			// The following enables MITM attack between seed kube-apiserver and GCMx. Not ideal, but it's on par with
			// the metrics-server setup. For more information, see https://github.com/kubernetes-sigs/metrics-server/issues/544
			InsecureSkipTLSVerify: true,
		},
	}
}
