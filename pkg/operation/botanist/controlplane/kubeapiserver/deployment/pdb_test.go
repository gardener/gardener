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

package deployment_test

import (
	"context"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func expectPodDisruptionBudget(ctx context.Context) {
	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	oneMaxUnavailable := intstr.FromInt(1)
	expectedPdb := &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver",
			Namespace: defaultSeedNamespace,
			Labels: map[string]string{
				"app":  "kubernetes",
				"role": "apiserver",
			},
		},
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &oneMaxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"gardener.cloud/role":     "controlplane",
					"garden.sapcloud.io/role": "controlplane",
					"app":                     "kubernetes",
					"role":                    "apiserver",
				},
			},
		},
	}
	mockSeedClient.EXPECT().Create(ctx, expectedPdb).Times(1)
}
