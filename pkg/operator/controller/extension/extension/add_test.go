// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/extension"
)

var _ = Describe("Add", func() {
	var (
		ctx                    = context.Background()
		log                    logr.Logger
		fakeClient             client.Client
		extension1, extension2 *operatorv1alpha1.Extension
	)

	BeforeEach(func() {
		log = logr.Discard()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		extension1 = &operatorv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Name: "extension1"}}
		extension2 = &operatorv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Name: "extension2"}}

		Expect(fakeClient.Create(ctx, extension1)).To(Succeed())
		Expect(fakeClient.Create(ctx, extension2)).To(Succeed())
	})

	Describe("#MapToAllExtensions", func() {
		It("should map to all extensions", func() {
			Expect((&Reconciler{RuntimeClientSet: fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()}).MapToAllExtensions(log)(ctx, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "extension1"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "extension2"}},
			))
		})
	})
})
