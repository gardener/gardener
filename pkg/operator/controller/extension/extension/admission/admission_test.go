// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admission_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	mockchartrenderer "github.com/gardener/gardener/pkg/chartrenderer/mock"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/extension/admission"
	ocifake "github.com/gardener/gardener/pkg/utils/oci/fake"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Deployment", func() {
	var (
		ctx  context.Context
		log  logr.Logger
		ctrl *gomock.Controller

		chartRenderer    *mockchartrenderer.MockInterface
		runtimeClient    client.Client
		runtimeClientSet *fakekubernetes.ClientSet
		virtualClient    client.Client
		virtualClientSet *fakekubernetes.ClientSet

		ociRefRuntime     string
		ociRefApplication string
		ociRegistry       *ocifake.Registry

		admission Interface

		extensionName string
		extension     *operatorv1alpha1.Extension
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		ctrl = gomock.NewController(GinkgoT())

		ociRefRuntime = "local-extension-runtime:v1.2.3"
		ociRefApplication = "local-extension-virtual:v1.2.3"
		ociRegistry = ocifake.NewRegistry()

		chartRenderer = mockchartrenderer.NewMockInterface(ctrl)
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		runtimeClientSet = fakekubernetes.NewClientSetBuilder().WithChartRenderer(chartRenderer).WithClient(runtimeClient).Build()
		virtualClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.VirtualScheme).Build()
		virtualClientSet = fakekubernetes.NewClientSetBuilder().WithChartRenderer(chartRenderer).WithClient(virtualClient).Build()

		admission = New(runtimeClientSet, &record.FakeRecorder{}, "garden", ociRegistry)

		extensionName = "test-extension"
		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionName,
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Deployment: &operatorv1alpha1.Deployment{
					AdmissionDeployment: &operatorv1alpha1.AdmissionDeploymentSpec{
						RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{Ref: &ociRefRuntime},
							},
						},
						VirtualCluster: &operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{Ref: &ociRefApplication},
							},
						},
					},
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		var (
			genericKubeconfigSecretName string
		)

		BeforeEach(func() {
			genericKubeconfigSecretName = "access-test"
		})

		It("should fail when virtual OCI artifact is not found", func() {
			Expect(admission.Reconcile(ctx, log, virtualClientSet, genericKubeconfigSecretName, extension)).To(MatchError(`failed pulling Helm chart from OCI repository "local-extension-virtual:v1.2.3": not found`))
		})

		It("should fail when runtime OCI artifact is not found", func() {
			ociRegistry.AddArtifact(&gardencorev1.OCIRepository{Ref: &ociRefApplication}, []byte(""))
			chartRenderer.EXPECT().RenderArchive(gomock.Any(), extension.Name, fmt.Sprintf("extension-%s", extension.Name), gomock.Any()).Return(&chartrenderer.RenderedChart{}, nil)

			defer test.WithVar(&retry.Until, func(_ context.Context, _ time.Duration, _ retry.Func) error {
				return nil
			})()

			Expect(admission.Reconcile(ctx, log, virtualClientSet, genericKubeconfigSecretName, extension)).To(MatchError(`failed pulling Helm chart from OCI repository "local-extension-runtime:v1.2.3": not found`))
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}, &resourcesv1alpha1.ManagedResource{})).To(Succeed())
		})

		It("should succeed reconciling the admission resources", func() {
			extension.Spec.Deployment.AdmissionDeployment.Values = &apiextensionsv1.JSON{
				Raw: []byte(`{"foo": "bar"}`),
			}

			ociRegistry.AddArtifact(&gardencorev1.OCIRepository{Ref: &ociRefApplication}, []byte("virtual-chart"))
			ociRegistry.AddArtifact(&gardencorev1.OCIRepository{Ref: &ociRefRuntime}, []byte("runtime-chart"))

			expectedVirtualValues := map[string]any{
				"foo": "bar",
				"gardener": map[string]any{
					"virtualCluster": map[string]any{
						"enabled": true,
						"serviceAccount": map[string]any{
							"name":      "extension-admission-" + extensionName,
							"namespace": "kube-system",
						},
					},
				},
			}
			expectedRuntimeValues := map[string]any{
				"foo": "bar",
				"gardener": map[string]any{
					"runtimeCluster": map[string]any{
						"priorityClassName": "gardener-garden-system-400",
					},
					"virtualCluster": map[string]any{
						"enabled":   true,
						"namespace": "extension-" + extensionName,
					},
				},
			}

			chartRenderer.EXPECT().RenderArchive([]byte("virtual-chart"), extension.Name, fmt.Sprintf("extension-%s", extension.Name), expectedVirtualValues).Return(&chartrenderer.RenderedChart{}, nil)
			chartRenderer.EXPECT().RenderArchive([]byte("runtime-chart"), extension.Name, "garden", expectedRuntimeValues).Return(&chartrenderer.RenderedChart{}, nil)

			defer test.WithVar(&retry.Until, func(_ context.Context, _ time.Duration, _ retry.Func) error {
				return nil
			})()

			Expect(admission.Reconcile(ctx, log, virtualClientSet, genericKubeconfigSecretName, extension)).To(Succeed())
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}, &resourcesv1alpha1.ManagedResource{})).To(Succeed())
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: "extension-admission-runtime-" + extensionName, Namespace: "garden"}, &resourcesv1alpha1.ManagedResource{})).To(Succeed())
		})

		It("should succeed if admission deployment is not defined", func() {
			extension.Spec.Deployment.AdmissionDeployment = nil

			Expect(admission.Reconcile(ctx, log, virtualClientSet, genericKubeconfigSecretName, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})

		It("should delete the admission deployment", func() {
			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "extension-admission-virtual-" + extensionName}}}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-runtime-" + extensionName, Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-runtime-" + extensionName, Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "extension-admission-runtime-" + extensionName}}}})).To(Succeed())

			extension.Spec.Deployment.AdmissionDeployment = nil

			Expect(admission.Reconcile(ctx, log, virtualClientSet, genericKubeconfigSecretName, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())

			secretList := &corev1.SecretList{}
			Expect(runtimeClient.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())
		})

		It("should only delete the managed resource for the virtual cluster", func() {
			ociRegistry.AddArtifact(&gardencorev1.OCIRepository{Ref: &ociRefRuntime}, []byte("runtime-chart"))
			chartRenderer.EXPECT().RenderArchive([]byte("runtime-chart"), extension.Name, "garden", gomock.Any()).Return(&chartrenderer.RenderedChart{}, nil)
			defer test.WithVar(&retry.Until, func(_ context.Context, _ time.Duration, _ retry.Func) error {
				return nil
			})()

			runtimeManagedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-runtime-" + extensionName, Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "extension-admission-runtime-" + extensionName}}}}

			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "extension-admission-virtual-" + extensionName}}}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-runtime-" + extensionName, Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, runtimeManagedResource)).To(Succeed())

			extension.Spec.Deployment.AdmissionDeployment.VirtualCluster = nil

			Expect(admission.Reconcile(ctx, log, virtualClientSet, genericKubeconfigSecretName, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"ObjectMeta": MatchFields(IgnoreExtras, Fields{
					"Name":      Equal(runtimeManagedResource.Name),
					"Namespace": Equal(runtimeManagedResource.Namespace),
				}),
			})))
		})

		It("should only delete the managed resource for the runtime cluster", func() {
			ociRegistry.AddArtifact(&gardencorev1.OCIRepository{Ref: &ociRefApplication}, []byte("virtual-chart"))
			chartRenderer.EXPECT().RenderArchive([]byte("virtual-chart"), extension.Name, fmt.Sprintf("extension-%s", extension.Name), gomock.Any()).Return(&chartrenderer.RenderedChart{}, nil)
			defer test.WithVar(&retry.Until, func(_ context.Context, _ time.Duration, _ retry.Func) error {
				return nil
			})()

			virtualManagedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "extension-admission-virtual-" + extensionName}}}}

			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, virtualManagedResource)).To(Succeed())
			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-runtime-" + extensionName, Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-runtime-" + extensionName, Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "extension-admission-runtime-" + extensionName}}}})).To(Succeed())

			extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster = nil

			Expect(admission.Reconcile(ctx, log, virtualClientSet, genericKubeconfigSecretName, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"ObjectMeta": MatchFields(IgnoreExtras, Fields{
					"Name":      Equal(virtualManagedResource.Name),
					"Namespace": Equal(virtualManagedResource.Namespace),
				}),
			})))
		})
	})

	Describe("#Delete", func() {
		It("should succeed if extension was not deployed before", func() {
			Expect(admission.Delete(ctx, log, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})

		It("should succeed if extension was deployed before", func() {
			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shoot-access-extension-admission-" + extensionName, Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-virtual-" + extensionName, Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "extension-admission-virtual-" + extensionName}}}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-runtime-" + extensionName, Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "extension-admission-runtime-" + extensionName, Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "extension-admission-runtime-" + extensionName}}}})).To(Succeed())

			Expect(admission.Delete(ctx, log, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())

			secretList := &corev1.SecretList{}
			Expect(runtimeClient.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())
		})
	})
})
