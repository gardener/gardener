// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mapper_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/mapper"
)

var _ = Describe("Mapper", func() {
	Describe("#MapControllerInstallationToExtension", func() {
		var (
			ctx context.Context
			log logr.Logger

			fakeClient client.Client

			mapperFunc handler.MapFunc
		)

		BeforeEach(func() {
			ctx = context.Background()
			log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

			fakeClient = fake.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

			mapperFunc = MapControllerInstallationToExtension(fakeClient, log)
		})

		Context("without controller installation", func() {
			It("should not return any request", func() {
				requests := mapperFunc(ctx, nil)
				Expect(requests).To(BeEmpty())
			})
		})

		Context("with controller installation", func() {
			var (
				extensionName          string
				controllerInstallation *gardencorev1beta1.ControllerInstallation
				extension              *operatorv1alpha1.Extension
			)

			BeforeEach(func() {
				extensionName = "provider-local"

				controllerInstallation = &gardencorev1beta1.ControllerInstallation{
					ObjectMeta: metav1.ObjectMeta{
						Name: extensionName + "-123",
					},
					Spec: gardencorev1beta1.ControllerInstallationSpec{
						RegistrationRef: corev1.ObjectReference{
							Name: extensionName,
						},
					},
				}

				extension = &operatorv1alpha1.Extension{
					ObjectMeta: metav1.ObjectMeta{
						Name: extensionName,
					},
				}

				Expect(fakeClient.Create(ctx, extension)).To(Succeed())
			})

			It("should return expected extension request", func() {
				requests := mapperFunc(ctx, controllerInstallation)
				Expect(requests).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: extensionName}}))
			})

			It("should not return any request if no related extension is found", func() {
				controllerInstallation.Spec.RegistrationRef.Name = controllerInstallation.Spec.RegistrationRef.Name + "-foo"
				requests := mapperFunc(ctx, controllerInstallation)
				Expect(requests).To(BeEmpty())
			})
		})
	})
})
