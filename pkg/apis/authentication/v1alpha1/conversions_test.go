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

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/authentication"
	. "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
)

var _ = Describe("conversion", func() {
	var (
		expirationSeconds   int64 = 1337
		kubeconfig                = []byte("kubeconfig")
		expirationTimestamp       = metav1.Now()
	)

	Describe("#Convert_v1alpha1_AdminKubeconfigRequest_To_authentication_KubeconfigRequest", func() {
		It("should properly convert", func() {
			in := &AdminKubeconfigRequest{
				Spec:   AdminKubeconfigRequestSpec{ExpirationSeconds: &expirationSeconds},
				Status: AdminKubeconfigRequestStatus{Kubeconfig: kubeconfig, ExpirationTimestamp: expirationTimestamp},
			}
			out := &authentication.KubeconfigRequest{}

			Expect(Convert_v1alpha1_AdminKubeconfigRequest_To_authentication_KubeconfigRequest(in, out, nil)).To(Succeed())

			Expect(out.Spec).To(Equal(authentication.KubeconfigRequestSpec{ExpirationSeconds: expirationSeconds}))
			Expect(out.Status).To(Equal(authentication.KubeconfigRequestStatus{Kubeconfig: kubeconfig, ExpirationTimestamp: expirationTimestamp}))
		})
	})

	Describe("#Convert_authentication_KubeconfigRequest_To_v1alpha1_AdminKubeconfigRequest", func() {
		It("should properly convert", func() {
			in := &authentication.KubeconfigRequest{
				Spec:   authentication.KubeconfigRequestSpec{ExpirationSeconds: expirationSeconds},
				Status: authentication.KubeconfigRequestStatus{Kubeconfig: kubeconfig, ExpirationTimestamp: expirationTimestamp},
			}
			out := &AdminKubeconfigRequest{}

			Expect(Convert_authentication_KubeconfigRequest_To_v1alpha1_AdminKubeconfigRequest(in, out, nil)).To(Succeed())

			Expect(out.Spec).To(Equal(AdminKubeconfigRequestSpec{ExpirationSeconds: &expirationSeconds}))
			Expect(out.Status).To(Equal(AdminKubeconfigRequestStatus{Kubeconfig: kubeconfig, ExpirationTimestamp: expirationTimestamp}))
		})
	})

	Describe("#Convert_v1alpha1_ViewerKubeconfigRequest_To_authentication_KubeconfigRequest", func() {
		It("should properly convert", func() {
			in := &ViewerKubeconfigRequest{
				Spec:   ViewerKubeconfigRequestSpec{ExpirationSeconds: &expirationSeconds},
				Status: ViewerKubeconfigRequestStatus{Kubeconfig: kubeconfig, ExpirationTimestamp: expirationTimestamp},
			}
			out := &authentication.KubeconfigRequest{}

			Expect(Convert_v1alpha1_ViewerKubeconfigRequest_To_authentication_KubeconfigRequest(in, out, nil)).To(Succeed())

			Expect(out.Spec).To(Equal(authentication.KubeconfigRequestSpec{ExpirationSeconds: expirationSeconds}))
			Expect(out.Status).To(Equal(authentication.KubeconfigRequestStatus{Kubeconfig: kubeconfig, ExpirationTimestamp: expirationTimestamp}))
		})
	})

	Describe("#Convert_authentication_KubeconfigRequest_To_v1alpha1_ViewerKubeconfigRequest", func() {
		It("should properly convert", func() {
			in := &authentication.KubeconfigRequest{
				Spec:   authentication.KubeconfigRequestSpec{ExpirationSeconds: expirationSeconds},
				Status: authentication.KubeconfigRequestStatus{Kubeconfig: kubeconfig, ExpirationTimestamp: expirationTimestamp},
			}
			out := &ViewerKubeconfigRequest{}

			Expect(Convert_authentication_KubeconfigRequest_To_v1alpha1_ViewerKubeconfigRequest(in, out, nil)).To(Succeed())

			Expect(out.Spec).To(Equal(ViewerKubeconfigRequestSpec{ExpirationSeconds: &expirationSeconds}))
			Expect(out.Status).To(Equal(ViewerKubeconfigRequestStatus{Kubeconfig: kubeconfig, ExpirationTimestamp: expirationTimestamp}))
		})
	})
})
