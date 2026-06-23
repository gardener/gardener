// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("#GetControllerRegistrationsForInstallations", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client

		installations *gardencorev1beta1.ControllerInstallationList
	)

	registration := func(name string) *gardencorev1beta1.ControllerRegistration {
		return &gardencorev1beta1.ControllerRegistration{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}
	installation := func(registrationName string) gardencorev1beta1.ControllerInstallation {
		return gardencorev1beta1.ControllerInstallation{Spec: gardencorev1beta1.ControllerInstallationSpec{
			RegistrationRef: corev1.ObjectReference{Name: registrationName},
		}}
	}

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		installations = &gardencorev1beta1.ControllerInstallationList{}
	})

	It("should return the distinct ControllerRegistrations referenced by the installations", func() {
		Expect(fakeClient.Create(ctx, registration("reg-a"))).To(Succeed())
		Expect(fakeClient.Create(ctx, registration("reg-b"))).To(Succeed())
		installations.Items = []gardencorev1beta1.ControllerInstallation{installation("reg-a"), installation("reg-b")}

		result, err := GetControllerRegistrationsForInstallations(ctx, fakeClient, installations)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Items).To(HaveLen(2))
		Expect(result.Items[0].Name).To(Equal("reg-a"))
		Expect(result.Items[1].Name).To(Equal("reg-b"))
	})

	It("should deduplicate registrations referenced by multiple installations", func() {
		Expect(fakeClient.Create(ctx, registration("reg-a"))).To(Succeed())
		installations.Items = []gardencorev1beta1.ControllerInstallation{installation("reg-a"), installation("reg-a")}

		result, err := GetControllerRegistrationsForInstallations(ctx, fakeClient, installations)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Items).To(HaveLen(1))
		Expect(result.Items[0].Name).To(Equal("reg-a"))
	})

	It("should skip references to non-existing ControllerRegistrations", func() {
		Expect(fakeClient.Create(ctx, registration("reg-a"))).To(Succeed())
		installations.Items = []gardencorev1beta1.ControllerInstallation{installation("reg-a"), installation("gone")}

		result, err := GetControllerRegistrationsForInstallations(ctx, fakeClient, installations)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Items).To(HaveLen(1))
		Expect(result.Items[0].Name).To(Equal("reg-a"))
	})

	It("should return an empty list when there are no installations", func() {
		result, err := GetControllerRegistrationsForInstallations(ctx, fakeClient, installations)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Items).To(BeEmpty())
	})

	It("should propagate non-NotFound errors", func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
			Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return fmt.Errorf("fake error")
			},
		}).Build()
		installations.Items = []gardencorev1beta1.ControllerInstallation{installation("reg-a")}

		_, err := GetControllerRegistrationsForInstallations(ctx, fakeClient, installations)
		Expect(err).To(MatchError(ContainSubstring("fake error")))
	})
})
