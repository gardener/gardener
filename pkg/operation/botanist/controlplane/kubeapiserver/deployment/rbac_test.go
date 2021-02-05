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
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func expectRBAC(ctx context.Context, valuesProvider KubeAPIServerValuesProvider) {
	if !valuesProvider.IsKonnectivityTunnelEnabled() || valuesProvider.IsSNIEnabled() {
		return
	}

	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "konnectivity-server"), gomock.AssignableToTypeOf(&rbacv1.Role{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))
	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "konnectivity-server"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "konnectivity-server",
			Namespace: defaultSeedNamespace,
			Labels: map[string]string{
				"gardener.cloud/role": "controlplane",
				"app":                 "kubernetes",
				"role":                "apiserver",
			},
		},
		Rules: []rbacv1.PolicyRule{{
			Verbs:         []string{"get", "list", "watch"},
			APIGroups:     []string{appsv1.SchemeGroupVersion.Group},
			Resources:     []string{"deployments"},
			ResourceNames: []string{"kube-apiserver"},
		},
		},
	}

	expectedRolebinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "konnectivity-server",
			Namespace: defaultSeedNamespace,
			Labels: map[string]string{
				"gardener.cloud/role": "controlplane",
				"app":                 "kubernetes",
				"role":                "apiserver",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "konnectivity-server",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      "kube-apiserver",
			Namespace: defaultSeedNamespace,
		}},
	}
	mockSeedClient.EXPECT().Create(ctx, expectedRole).Times(1)
	mockSeedClient.EXPECT().Create(ctx, expectedRolebinding).Times(1)
}
