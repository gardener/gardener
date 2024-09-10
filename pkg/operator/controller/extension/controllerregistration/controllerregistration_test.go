// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/controllerregistration"
)

var _ = Describe("ControllerRegistration", func() {
	var (
		ctx                    context.Context
		log                    logr.Logger
		virtualClient          client.Client
		controllerRegistration Interface

		extensionName string
		ociRef        string
		extensionKind string

		extension *operatorv1alpha1.Extension
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		virtualClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.VirtualScheme).Build()
		controllerRegistration = New(&record.FakeRecorder{})

		extensionName = "test-extension"
		ociRef = "test-extension:v1.2.3"
		extensionKind = "worker"

		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionName,
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{Kind: extensionKind},
				},
				Deployment: &operatorv1alpha1.Deployment{
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						DeploymentSpec: operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{
									Ref: ptr.To(ociRef),
								},
							},
						},
					},
				},
			},
		}
	})

	Describe("#Reconcile", func() {
		It("should create the expected ControllerRegistration and ControllerInstallation resources", func() {
			Expect(controllerRegistration.Reconcile(ctx, log, virtualClient, extension)).To(Succeed())

			var controllerDeploymentList gardencorev1.ControllerDeploymentList
			Expect(virtualClient.List(ctx, &controllerDeploymentList)).To(Succeed())
			Expect(controllerDeploymentList.Items).To(ConsistOf(gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            extensionName,
					ResourceVersion: "1",
				},
				Helm: &gardencorev1.HelmControllerDeployment{
					OCIRepository: &gardencorev1.OCIRepository{
						Ref: ptr.To(ociRef),
					},
				},
			}))

			var controllerRegistrationList gardencorev1beta1.ControllerRegistrationList
			Expect(virtualClient.List(ctx, &controllerRegistrationList)).To(Succeed())
			Expect(controllerRegistrationList.Items).To(ConsistOf(gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:            extensionName,
					ResourceVersion: "1",
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionKind},
					},
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: extensionName},
						},
					},
				},
			}))
		})

		It("should succeed if extension deployment is not defined", func() {
			extension.Spec.Deployment = nil

			Expect(controllerRegistration.Reconcile(ctx, log, virtualClient, extension)).To(Succeed())

			var controllerDeploymentList gardencorev1.ControllerDeploymentList
			Expect(virtualClient.List(ctx, &controllerDeploymentList)).To(Succeed())
			Expect(controllerDeploymentList.Items).To(BeEmpty())

			var controllerRegistrationList gardencorev1beta1.ControllerRegistrationList
			Expect(virtualClient.List(ctx, &controllerRegistrationList)).To(Succeed())
			Expect(controllerRegistrationList.Items).To(BeEmpty())
		})

		It("should delete the extension", func() {
			Expect(controllerRegistration.Reconcile(ctx, log, virtualClient, extension)).To(Succeed())

			var controllerDeploymentList gardencorev1.ControllerDeploymentList
			Expect(virtualClient.List(ctx, &controllerDeploymentList)).To(Succeed())
			Expect(controllerDeploymentList.Items).To(HaveLen(1))

			var controllerRegistrationList gardencorev1beta1.ControllerRegistrationList
			Expect(virtualClient.List(ctx, &controllerRegistrationList)).To(Succeed())
			Expect(controllerRegistrationList.Items).To(HaveLen(1))

			extension.Spec.Deployment = nil

			Expect(controllerRegistration.Reconcile(ctx, log, virtualClient, extension)).To(Succeed())

			Expect(virtualClient.List(ctx, &controllerDeploymentList)).To(Succeed())
			Expect(controllerDeploymentList.Items).To(BeEmpty())

			Expect(virtualClient.List(ctx, &controllerRegistrationList)).To(Succeed())
			Expect(controllerRegistrationList.Items).To(BeEmpty())
		})
	})

	Describe("#Delete", func() {
		It("should succeed if extension was not deployed before", func() {
			Expect(controllerRegistration.Delete(ctx, log, virtualClient, extension)).To(Succeed())

			var controllerDeploymentList gardencorev1.ControllerDeploymentList
			Expect(virtualClient.List(ctx, &controllerDeploymentList)).To(Succeed())
			Expect(controllerDeploymentList.Items).To(BeEmpty())

			var controllerRegistrationList gardencorev1beta1.ControllerRegistrationList
			Expect(virtualClient.List(ctx, &controllerRegistrationList)).To(Succeed())
			Expect(controllerRegistrationList.Items).To(BeEmpty())
		})

		It("should succeed if extension was deployed before", func() {
			Expect(controllerRegistration.Reconcile(ctx, log, virtualClient, extension)).To(Succeed())

			var controllerDeploymentList gardencorev1.ControllerDeploymentList
			Expect(virtualClient.List(ctx, &controllerDeploymentList)).To(Succeed())
			Expect(controllerDeploymentList.Items).To(HaveLen(1))

			var controllerRegistrationList gardencorev1beta1.ControllerRegistrationList
			Expect(virtualClient.List(ctx, &controllerRegistrationList)).To(Succeed())
			Expect(controllerRegistrationList.Items).To(HaveLen(1))

			Expect(controllerRegistration.Delete(ctx, log, virtualClient, extension)).To(Succeed())

			Expect(virtualClient.List(ctx, &controllerDeploymentList)).To(Succeed())
			Expect(controllerDeploymentList.Items).To(BeEmpty())

			Expect(virtualClient.List(ctx, &controllerRegistrationList)).To(Succeed())
			Expect(controllerRegistrationList.Items).To(BeEmpty())
		})
	})
})
