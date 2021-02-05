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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubeapiserver/deployment"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("controlplane", func() {
	Describe("#ValidateAuditPolicyApiGroupVersionKind", func() {
		var (
			kind = "Policy"
		)

		It("should return false without error because of version incompatibility", func() {
			incompatibilityMatrix := map[string][]schema.GroupVersionKind{
				"1.10.0": {
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.11.0": {
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
			}

			for shootVersion, gvks := range incompatibilityMatrix {
				for _, gvk := range gvks {
					ok, err := deployment.IsValidAuditPolicyVersion(shootVersion, &gvk)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeFalse())
				}
			}
		})

		It("should return true without error because of version compatibility", func() {
			compatibilityMatrix := map[string][]schema.GroupVersionKind{
				"1.10.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
				},
				"1.11.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
				},
				"1.12.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.13.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.14.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.15.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
			}

			for shootVersion, gvks := range compatibilityMatrix {
				for _, gvk := range gvks {
					ok, err := deployment.IsValidAuditPolicyVersion(shootVersion, &gvk)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeTrue())
				}
			}
		})

		It("should return false with error because of not valid semver version", func() {
			shootVersion := "1.ab.0"
			gvk := auditv1.SchemeGroupVersion.WithKind(kind)

			ok, err := deployment.IsValidAuditPolicyVersion(shootVersion, &gvk)
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})
})

const (
	// static checksums for empty audit policy deployed by this resource
	// If these policy changes, these constants need to be updated with the updated SHA256
	checksumEmptyAuditPolicyV1      = "42555d954ef4948505782cb5400463ee2503e444b39188e672f9d7e323118d16"
	checksumEmptyAuditPolicyV1beta1 = "e8c1cfde9c46933ddc6dc406863a66ecf4983bcf2d20c47131a7b9c2fcc7a740"
)

var (
	v1AuditPolicy = `apiVersion: audit.k8s.io/v1
kind: Policy
metadata:
  creationTimestamp: null
rules:
- level: None
`

	v1beta1AuditPolicy = `apiVersion: audit.k8s.io/v1beta1
kind: Policy
metadata:
  creationTimestamp: null
rules:
- level: None
`
	cmDataAuditPolicyV1 = map[string]string{
		"audit-policy.yaml": `---
` + v1AuditPolicy,
	}

	cmDataAuditPolicyV1Beta1 = map[string]string{
		"audit-policy.yaml": `---
` + v1beta1AuditPolicy,
	}

	// this is a custom v1beta1 audit policy provided by a config map in the Garden cluster
	// please note, that the config map data key is "policy" and not "audit-policy.yaml"
	// like in the audit config map deployed to the Seed cluster
	dataAuditPolicyV1Beta1UserProvided = map[string]string{
		"policy": v1beta1AuditPolicy,
	}
)

func expectAuditPolicyConfigMap(ctx context.Context, apiServerConfig *gardencorev1beta1.KubeAPIServerConfig, shootKubernetesVersion string) string {
	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "audit-policy-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	var (
		auditMap          map[string]string
		shaAuditConfigMap string
	)

	if apiServerConfig != nil &&
		apiServerConfig.AuditConfig != nil &&
		apiServerConfig.AuditConfig.AuditPolicy != nil &&
		apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		mockGardenClient.EXPECT().Get(ctx, kutil.Key(defaultGardenNamespace, apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, cm *corev1.ConfigMap) error {
			// contains an empty audit policy in version audit.k8s.io/v1beta1 valid for all supported K8s versions
			// there are dedicated unit tests for other Audit policy versions that are not supported across all K8s versions
			*cm = corev1.ConfigMap{Data: dataAuditPolicyV1Beta1UserProvided}
			return nil
		})
		// seed audit config map will be created with key "audit-policy.yaml"
		auditMap = cmDataAuditPolicyV1Beta1
		shaAuditConfigMap = checksumEmptyAuditPolicyV1beta1
	} else if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, ">=", "1.12"); ok {
		shaAuditConfigMap = checksumEmptyAuditPolicyV1
		auditMap = cmDataAuditPolicyV1
	} else {
		shaAuditConfigMap = checksumEmptyAuditPolicyV1beta1
		auditMap = cmDataAuditPolicyV1Beta1
	}

	expectedDefaultAdmissionConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "audit-policy-config",
			Namespace: defaultSeedNamespace,
		},
		Data: auditMap,
	}
	mockSeedClient.EXPECT().Create(ctx, expectedDefaultAdmissionConfig).Times(1)
	return shaAuditConfigMap
}
