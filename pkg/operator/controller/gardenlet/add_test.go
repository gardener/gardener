// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/gardenlet"
)

var _ = Describe("Add", func() {
	Describe("#OperatorResponsiblePredicate", func() {
		var (
			ctx = context.Background()

			fakeClient client.Client
			predicate  predicate.Predicate
			gardenlet  *seedmanagementv1alpha1.Gardenlet

			test = func(object client.Object, matcher gomegatypes.GomegaMatcher) {
				ExpectWithOffset(1, predicate.Create(event.CreateEvent{Object: object})).To(matcher)
				ExpectWithOffset(1, predicate.Update(event.UpdateEvent{ObjectNew: object})).To(matcher)
				ExpectWithOffset(1, predicate.Delete(event.DeleteEvent{Object: object})).To(matcher)
				ExpectWithOffset(1, predicate.Generic(event.GenericEvent{Object: object})).To(matcher)
			}
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.VirtualScheme).Build()
			predicate = (&Reconciler{VirtualClient: fakeClient}).OperatorResponsiblePredicate(ctx)
			gardenlet = &seedmanagementv1alpha1.Gardenlet{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		})

		It("should return true when the seed object does not exist", func() {
			test(gardenlet, BeTrue())
		})

		When("seed object exists", func() {
			BeforeEach(func() {
				Expect(fakeClient.Create(ctx, &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: gardenlet.Name}})).To(Succeed())
			})

			It("should return false because there is no force-redeploy annotation and no kubeconfig secret ref", func() {
				test(gardenlet, BeFalse())
			})

			It("should return true because there is a kubeconfig secret ref", func() {
				gardenlet.Spec.KubeconfigSecretRef = &corev1.LocalObjectReference{Name: "kubeconfig"}
				test(gardenlet, BeTrue())
			})

			It("should return true because there is the force-redeploy operation annotation", func() {
				metav1.SetMetaDataAnnotation(&gardenlet.ObjectMeta, "gardener.cloud/operation", "force-redeploy")
				test(gardenlet, BeTrue())
			})

			It("should return false because there is an unhandled operation annotation", func() {
				metav1.SetMetaDataAnnotation(&gardenlet.ObjectMeta, "gardener.cloud/operation", "foo")
				test(gardenlet, BeFalse())
			})
		})
	})
})
