// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("Cleanup", func() {
	Describe("#CleanWebhooks", func() {
		var (
			ctx         = context.Background()
			shootClient client.Client
			botanist    *Botanist

			policy, policyBinding, systemBinding, noCleanupBinding client.Object
		)

		BeforeEach(func() {
			shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()

			s := &shootpkg.Shoot{}
			s.SetInfo(&gardencorev1beta1.Shoot{})

			botanist = &Botanist{Operation: &operation.Operation{
				Logger:         logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON),
				ShootClientSet: fake.NewClientSetBuilder().WithClient(shootClient).Build(),
				Shoot:          s,
			}}

			policy = &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "policy"}}
			policyBinding = &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "binding"}}
			systemBinding = &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{
				Name:   "system-binding",
				Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent},
			}}
			noCleanupBinding = &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{
				Name:   "no-cleanup-binding",
				Labels: map[string]string{v1beta1constants.ShootNoCleanup: "true"},
			}}

			for _, obj := range []client.Object{policy, policyBinding, systemBinding, noCleanupBinding} {
				Expect(shootClient.Create(ctx, obj)).To(Succeed())
			}
		})

		It("should delete ValidatingAdmissionPolicyBindings but keep protected ones and the policies", func() {
			Expect(botanist.CleanWebhooks(ctx)).To(Succeed())

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(policy), &admissionregistrationv1.ValidatingAdmissionPolicy{})).To(Succeed())
			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(policyBinding), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}))).To(BeTrue())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemBinding), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupBinding), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{})).To(Succeed())
		})
	})
})
