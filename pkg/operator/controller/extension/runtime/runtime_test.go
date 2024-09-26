// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/runtime"
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
		runtimeClientSet *kubernetesfake.ClientSet

		ociRef      string
		ociRegistry *ocifake.Registry

		runtime Interface

		extensionName string
		extension     *operatorv1alpha1.Extension
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		ctrl = gomock.NewController(GinkgoT())

		ociRef = "local-extension-runtime:v1.2.3"
		ociRegistry = ocifake.NewRegistry()

		chartRenderer = mockchartrenderer.NewMockInterface(ctrl)
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		runtimeClientSet = kubernetesfake.NewClientSetBuilder().WithChartRenderer(chartRenderer).WithClient(runtimeClient).Build()

		runtime = New(runtimeClientSet, &record.FakeRecorder{}, "garden", ociRegistry)

		extensionName = "test-extension"
		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionName,
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Deployment: &operatorv1alpha1.Deployment{
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						RuntimeClusterValues: &apiextensionsv1.JSON{Raw: []byte("{}")},
						DeploymentSpec: operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{Ref: &ociRef},
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
		It("should fail when OCI artifact is not found", func() {
			Expect(runtime.Reconcile(ctx, log, extension)).To(MatchError(`failed pulling Helm chart from OCI repository "local-extension-runtime:v1.2.3": not found`))
		})

		It("should succeed reconciling the extension resources", func() {
			extension.Spec.Deployment.ExtensionDeployment.RuntimeClusterValues = &apiextensionsv1.JSON{
				Raw: []byte(`{"foo": "bar"}`),
			}

			ociRegistry.AddArtifact(&gardencorev1.OCIRepository{Ref: &ociRef}, []byte("extension-chart"))

			expectedValues := map[string]any{
				"foo": "bar",
				"gardener": map[string]any{
					"runtimeCluster": map[string]any{
						"enabled":           "true",
						"priorityClassName": "gardener-garden-system-200",
					},
				},
			}

			chartRenderer.EXPECT().RenderArchive([]byte("extension-chart"), extension.Name, "garden", expectedValues).Return(&chartrenderer.RenderedChart{}, nil)

			defer test.WithVar(&retry.Until, func(_ context.Context, _ time.Duration, _ retry.Func) error {
				return nil
			})()

			Expect(runtime.Reconcile(ctx, log, extension)).To(Succeed())
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("extension-%s-garden", extensionName), Namespace: "garden"}, &resourcesv1alpha1.ManagedResource{})).To(Succeed())
		})

		It("should succeed if extension deployment is not defined", func() {
			extension.Spec.Deployment.ExtensionDeployment = nil

			Expect(runtime.Reconcile(ctx, log, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})
	})

	Describe("#Delete", func() {
		It("should succeed if extension was not deployed before", func() {
			Expect(runtime.Delete(ctx, log, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})

		It("should succeed if extension was deployed before", func() {
			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extension-%s-garden", extensionName), Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extension-%s-garden", extensionName), Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: fmt.Sprintf("extension-%s-garden", extensionName)}}}})).To(Succeed())

			Expect(runtime.Delete(ctx, log, extension)).To(Succeed())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())

			secretList := &corev1.SecretList{}
			Expect(runtimeClient.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())
		})
	})
})
