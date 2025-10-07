// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespace_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/webhook/validation/namespace"
)

var _ = Describe("Handler", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		handler    *Handler
		namespace  *corev1.Namespace
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		handler = &Handler{RuntimeClient: fakeClient}
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		}
	})

	Describe("#ValidateCreate", func() {
		It("should do nothing", func() {
			warning, err := handler.ValidateCreate(ctx, namespace)
			Expect(warning).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ValidateUpdate", func() {
		It("should do nothing", func() {
			warning, err := handler.ValidateUpdate(ctx, namespace, namespace)
			Expect(warning).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ValidateDelete", func() {
		It("should allow deletion because no Garden exists", func() {
			warning, err := handler.ValidateDelete(ctx, namespace)
			Expect(warning).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent deletion because a Garden exists", func() {
			Expect(fakeClient.Create(ctx, &operatorv1alpha1.Garden{ObjectMeta: metav1.ObjectMeta{Name: "foo"}})).To(Succeed())

			warning, err := handler.ValidateDelete(ctx, namespace)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("is forbidden while a Garden resource exists - delete it first before deleting the namespace")))
		})
	})
})
