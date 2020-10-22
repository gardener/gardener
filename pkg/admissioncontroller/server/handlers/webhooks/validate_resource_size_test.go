// Copyright (c) 2020 SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package webhooks_test

import (
	"net/http"
	"net/http/httptest"

	apisconfig "github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	. "github.com/gardener/gardener/pkg/admissioncontroller/server/handlers/webhooks"
	core "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("ValidateResourceSizeHandler", func() {
	logger.Logger = logger.NewNopLogger()

	var (
		projectsSizeLimit, _ = resource.ParseQuantity("0M")
		secretSizeLimit, _   = resource.ParseQuantity("1Mi")
		// size of shoot w/o spec
		shootsv1beta1SizeLimit, _ = resource.ParseQuantity("0.354k")
		// size of shoot w/o spec -1 byte
		shootsv1alpha1SizeLimit, _ = resource.ParseQuantity("0.354k")

		unrestrictedUserName                = "unrestrictedUser"
		unrestrictedGroupName               = "unrestrictedGroup"
		unrestrictedServiceAccountName      = "unrestrictedServiceAccount"
		unrestrictedServiceAccountNamespace = "unrestricted"

		config = &apisconfig.ResourceAdmissionConfiguration{
			UnrestrictedSubjects: []rbacv1.Subject{
				{
					Kind: rbacv1.GroupKind,
					Name: unrestrictedGroupName,
				},
				{
					Kind: rbacv1.UserKind,
					Name: unrestrictedUserName,
				},
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      unrestrictedServiceAccountName,
					Namespace: unrestrictedServiceAccountNamespace,
				},
			},
			Limits: []apisconfig.ResourceLimit{
				{
					APIGroups:   []string{"*"},
					APIVersions: []string{"*"},
					Resources:   []string{"projects"},
					Size:        projectsSizeLimit,
				},
				{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"secrets"},
					Size:        secretSizeLimit,
				},
				{
					APIGroups:   []string{"core.gardener.cloud"},
					APIVersions: []string{"v1beta1"},
					Resources:   []string{"shoots", "plants"},
					Size:        shootsv1beta1SizeLimit,
				},
				{
					APIGroups:   []string{"core.gardener.cloud"},
					APIVersions: []string{"v1alpha1"},
					Resources:   []string{"shoots"},
					Size:        shootsv1alpha1SizeLimit,
				},
			},
		}

		empty = func() runtime.Object {
			return nil
		}

		shootv1beta1 = func() runtime.Object {
			return &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Shoot",
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				},
			}
		}

		shootv1alpha1 = func() runtime.Object {
			return &gardencorev1alpha1.Shoot{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Shoot",
					APIVersion: gardencorev1alpha1.SchemeGroupVersion.String(),
				},
			}
		}

		project = func() runtime.Object {
			return &gardencorev1beta1.Project{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Project",
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				},
			}
		}

		secret = func() runtime.Object {
			return &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
			}
		}

		configMap = func() runtime.Object {
			return &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
			}
		}

		unrestrictedUser = func() authenticationv1.UserInfo {
			return authenticationv1.UserInfo{
				Username: unrestrictedUserName,
				Groups:   []string{"test"},
			}
		}

		unrestrictedGroup = func() authenticationv1.UserInfo {
			return authenticationv1.UserInfo{
				Username: "restricted",
				Groups:   []string{unrestrictedGroupName},
			}
		}

		unrestrictedServiceAccount = func() authenticationv1.UserInfo {
			return authenticationv1.UserInfo{
				Username: serviceaccount.MakeUsername(unrestrictedServiceAccountNamespace, unrestrictedServiceAccountName),
				Groups:   serviceaccount.MakeGroupNames(unrestrictedGroupName),
			}
		}

		restrictedUser = func() authenticationv1.UserInfo {
			return authenticationv1.UserInfo{
				Username: "restricted",
				Groups:   []string{"test"},
			}
		}
	)

	DescribeTable("Quotas test",
		func(objFn func() runtime.Object, subresource string, userFn func() authenticationv1.UserInfo, expectedAllowed bool, expectedMsg string) {
			scheme := runtime.NewScheme()
			core.Install(scheme)
			utilruntime.Must(kubernetesscheme.AddToScheme(scheme))

			response := httptest.NewRecorder()
			validator := NewValidateResourceSizeHandler(config)

			request := createHTTPRequest(objFn(), scheme, userFn(), admissionv1beta1.Update)
			if subresource != "" {
				request = createHTTPRequestForSubresource(subresource)
			}

			validator.ServeHTTP(response, request)

			admissionReview := &admissionv1beta1.AdmissionReview{}
			Expect(decodeAdmissionResponse(response, admissionReview)).To(Succeed())

			Expect(response).Should(HaveHTTPStatus(http.StatusOK))
			Expect(admissionReview.Response).To(Not(BeNil()))
			Expect(admissionReview.Response.Allowed).To(Equal(expectedAllowed))
			if expectedMsg != "" {
				Expect(admissionReview.Response.Result).To(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Message": ContainSubstring(expectedMsg),
					})))
			}
		},
		Entry("request is empty", empty, noSubResource, restrictedUser, false, "missing admission request"),
		Entry("should pass because size is in range for v1beta1 shoot", shootv1beta1, noSubResource, restrictedUser, true, emptyMessage),
		Entry("should fail because size is not in range for v1alpha1 shoot", shootv1alpha1, noSubResource, restrictedUser, false, "resource size exceeded"),
		Entry("should pass because request is for status subresource of v1alpha1 shoot", shootv1alpha1, "status", restrictedUser, true, emptyMessage),
		Entry("should pass because size is in range for secret", secret, noSubResource, restrictedUser, true, emptyMessage),
		Entry("should pass because no limits configured for configMaps", configMap, noSubResource, restrictedUser, true, emptyMessage),
		Entry("should fail because size is not in range for project", project, noSubResource, restrictedUser, false, "resource size exceeded"),
		Entry("should pass because of unrestricted user", project, noSubResource, unrestrictedUser, true, emptyMessage),
		Entry("should pass because of unrestricted group", project, noSubResource, unrestrictedGroup, true, emptyMessage),
		Entry("should pass because of unrestricted service account", project, noSubResource, unrestrictedServiceAccount, true, emptyMessage),
	)
})
