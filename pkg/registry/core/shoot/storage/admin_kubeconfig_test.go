/*
Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	authenticationv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
)

var _ = Describe("Admin Kubeconfig", func() {
	kubeconfigTests(
		NewAdminKubeconfigREST,
		func() runtime.Object {
			return &authenticationv1alpha1.AdminKubeconfigRequest{
				Spec: authenticationv1alpha1.AdminKubeconfigRequestSpec{
					ExpirationSeconds: ptr.To(int64(time.Minute.Seconds() * 11)),
				},
			}
		},
		func(obj runtime.Object, expirationSeconds *int64) {
			akc := obj.(*authenticationv1alpha1.AdminKubeconfigRequest)
			akc.Spec.ExpirationSeconds = expirationSeconds
		},
		func(obj runtime.Object) metav1.Time {
			akc := obj.(*authenticationv1alpha1.AdminKubeconfigRequest)
			return akc.Status.ExpirationTimestamp
		},
		func(obj runtime.Object) []byte {
			akc := obj.(*authenticationv1alpha1.AdminKubeconfigRequest)
			return akc.Status.Kubeconfig
		},
		ConsistOf("system:masters"),
	)
})
