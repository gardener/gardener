// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
