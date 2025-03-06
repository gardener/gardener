// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/extension/controllerregistration"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerRegistration", func() {
	var (
		ctx                    context.Context
		log                    logr.Logger
		c                      client.Client
		controllerRegistration Interface

		gardenNamespace string
		extensionName   string
		ociRef          string
		extensionKind   string
		extension       *operatorv1alpha1.Extension

		scheme    *runtime.Scheme
		consistOf func(...client.Object) types.GomegaMatcher
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(resourcesv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(gardencoreinstall.AddToScheme(scheme)).To(Succeed())
		Expect(kubernetesscheme.AddToScheme(scheme)).To(Succeed())

		ctx = context.Background()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		c = fakeclient.NewClientBuilder().WithScheme(scheme).Build()

		gardenNamespace = "garden-test"
		extensionName = "test-extension"
		ociRef = "test-extension:v1.2.3"
		extensionKind = "worker"

		controllerRegistration = New(c, &record.FakeRecorder{}, gardenNamespace)

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
						InjectGardenKubeconfig: ptr.To(true),
					},
				},
			},
		}

		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)
	})

	Describe("#Reconcile", func() {
		It("should create the expected ControllerRegistration and ControllerInstallation resources", func() {
			Expect(controllerRegistration.Reconcile(ctx, log, extension)).To(Succeed())

			managedResource := &resourcesv1alpha1.ManagedResource{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: "extension-registration-" + extensionName}, managedResource)).To(Succeed())

			Expect(managedResource).To(consistOf(
				&gardencorev1.ControllerDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: extensionName,
					},
					Helm: &gardencorev1.HelmControllerDeployment{
						OCIRepository: &gardencorev1.OCIRepository{
							Ref: ptr.To(ociRef),
						},
					},
					InjectGardenKubeconfig: ptr.To(true),
				},
				&gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{
						Name: extensionName,
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
				},
			))
		})

		It("should succeed if extension deployment is not defined", func() {
			extension.Spec.Deployment = nil

			Expect(controllerRegistration.Reconcile(ctx, log, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(c.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})

		It("should delete the extension", func() {
			Expect(controllerRegistration.Reconcile(ctx, log, extension)).To(Succeed())

			managedResource := &resourcesv1alpha1.ManagedResource{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: "extension-registration-" + extensionName}, managedResource)).To(Succeed())

			extension.Spec.Deployment = nil

			Expect(controllerRegistration.Reconcile(ctx, log, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(c.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})
	})

	Describe("#Delete", func() {
		It("should succeed if extension was not deployed before", func() {
			Expect(controllerRegistration.Delete(ctx, log, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(c.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})

		It("should succeed if extension was deployed before", func() {
			Expect(controllerRegistration.Reconcile(ctx, log, extension)).To(Succeed())

			managedResource := &resourcesv1alpha1.ManagedResource{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: "extension-registration-" + extensionName}, managedResource)).To(Succeed())

			Expect(controllerRegistration.Delete(ctx, log, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(c.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})
	})
})
