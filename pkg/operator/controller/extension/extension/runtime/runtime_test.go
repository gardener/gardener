// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package runtime_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"helm.sh/helm/v4/pkg/chart/common"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
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
	. "github.com/gardener/gardener/pkg/operator/controller/extension/extension/runtime"
	gardeneroperator "github.com/gardener/gardener/pkg/utils/gardener/operator"
	ocifake "github.com/gardener/gardener/pkg/utils/oci/fake"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Runtime", func() {
	var (
		ctx  context.Context
		log  logr.Logger
		ctrl *gomock.Controller

		chartRenderer    *mockchartrenderer.MockInterface
		runtimeClient    client.Client
		runtimeClientSet *fakekubernetes.ClientSet

		virtualClient    client.Client
		virtualClientSet *fakekubernetes.ClientSet

		ociRef      string
		ociRegistry *ocifake.Registry

		runtime Interface

		extensionName string
		extension     *operatorv1alpha1.Extension

		gardenNamespace *corev1.Namespace
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		ctrl = gomock.NewController(GinkgoT())

		ociRef = "local-extension-runtime:v1.2.3"
		ociRegistry = ocifake.NewRegistry()

		chartRenderer = mockchartrenderer.NewMockInterface(ctrl)
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		runtimeClientSet = fakekubernetes.NewClientSetBuilder().WithChartRenderer(chartRenderer).WithClient(runtimeClient).Build()

		virtualClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		virtualClientSet = fakekubernetes.NewClientSetBuilder().
			WithClient(virtualClient).
			WithRESTConfig(&rest.Config{
				Host:            "https://virtual-garden-kube-apiserver.garden.svc.cluster.local",
				TLSClientConfig: rest.TLSClientConfig{CAData: []byte("virtual-garden-ca")},
			}).
			Build()

		runtime = New(runtimeClientSet, &events.FakeRecorder{}, "garden", ociRegistry)

		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden",
				Annotations: map[string]string{
					"high-availability-config.resources.gardener.cloud/zones": "eu-west-1a,eu-west-1b",
				},
			},
		}
		Expect(runtimeClient.Create(ctx, gardenNamespace)).To(Succeed())
		extensionName = "test-extension"
		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionName,
				Annotations: map[string]string{
					"security.gardener.cloud/pod-security-enforce": "baseline",
				},
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

		priorityClass := &schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: "gardener-garden-system-200"}}
		Expect(runtimeClient.Create(ctx, priorityClass)).To(Succeed())

		kubeSystemNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}}
		Expect(runtimeClient.Create(ctx, kubeSystemNamespace)).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.IgnoreNotFound(runtimeClient.Delete(ctx, gardenNamespace))).To(Succeed())
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		It("should fail when OCI artifact is not found", func() {
			_, err := runtime.Reconcile(ctx, log, virtualClientSet, extension)
			Expect(err).To(MatchError(`failed pulling Helm chart from OCI repository "local-extension-runtime:v1.2.3": not found`))
		})

		It("should succeed reconciling the extension resources", func() {
			extension.Spec.Deployment.ExtensionDeployment.RuntimeClusterValues = &apiextensionsv1.JSON{
				Raw: []byte(`{"foo": "bar"}`),
			}

			ociRegistry.AddArtifact(&gardencorev1.OCIRepository{Ref: &ociRef}, []byte("extension-chart"))

			expectedValues := map[string]any{
				"foo": "bar",
				"gardener": map[string]any{
					"clusterTypes": map[string]any{
						"gardenRuntimeCluster":   true,
						"selfHostedShootCluster": false,
					},
					"runtimeCluster": map[string]any{
						"enabled":           "true",
						"priorityClassName": "gardener-garden-system-200",
					},
				},
			}

			chartRenderer.EXPECT().RenderArchive([]byte("extension-chart"), extension.Name, "runtime-extension-test-extension", expectedValues).Return(&chartrenderer.RenderedChart{}, nil)

			defer test.WithVar(&retry.Until, func(_ context.Context, _ time.Duration, _ retry.Func) error {
				return nil
			})()

			_, err := runtime.Reconcile(ctx, log, virtualClientSet, extension)
			Expect(err).NotTo(HaveOccurred())
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: "garden-extension-test-extension-23b70", Namespace: "garden"}, &resourcesv1alpha1.ManagedResource{})).To(Succeed())
			namespace := &corev1.Namespace{}
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: "runtime-extension-test-extension"}, namespace)).To(Succeed())
			Expect(namespace.Labels).To(HaveKeyWithValue("gardener.cloud/role", "extension"))
			Expect(namespace.Labels).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"))
			Expect(namespace.Labels).To(HaveKeyWithValue("networking.gardener.cloud/access-target-apiserver", "allowed"))
			Expect(namespace.Labels).To(HaveKeyWithValue("pod-security.kubernetes.io/enforce", "baseline"))
			Expect(namespace.Annotations).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "eu-west-1a,eu-west-1b"))
		})

		It("should set selfHostedShootCluster=true in clusterTypes when runtime cluster is a self-hosted shoot", func() {
			kubeSystemNamespace := &corev1.Namespace{}
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: metav1.NamespaceSystem}, kubeSystemNamespace)).To(Succeed())
			patch := client.MergeFrom(kubeSystemNamespace.DeepCopy())
			metav1.SetMetaDataLabel(&kubeSystemNamespace.ObjectMeta, "gardener.cloud/role", "shoot")
			Expect(runtimeClient.Patch(ctx, kubeSystemNamespace, patch)).To(Succeed())

			ociRegistry.AddArtifact(&gardencorev1.OCIRepository{Ref: &ociRef}, []byte("extension-chart"))

			expectedValues := map[string]any{
				"gardener": map[string]any{
					"clusterTypes": map[string]any{
						"gardenRuntimeCluster":   true,
						"selfHostedShootCluster": true,
					},
					"runtimeCluster": map[string]any{
						"enabled":           "true",
						"priorityClassName": "gardener-garden-system-200",
					},
				},
			}

			chartRenderer.EXPECT().RenderArchive([]byte("extension-chart"), extension.Name, "runtime-extension-test-extension", expectedValues).Return(&chartrenderer.RenderedChart{}, nil)

			defer test.WithVar(&retry.Until, func(_ context.Context, _ time.Duration, _ retry.Func) error {
				return nil
			})()

			_, err := runtime.Reconcile(ctx, log, virtualClientSet, extension)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if extension deployment is not defined", func() {
			extension.Spec.Deployment.ExtensionDeployment = nil

			_, err := runtime.Reconcile(ctx, log, virtualClientSet, extension)
			Expect(err).NotTo(HaveOccurred())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})

		Context("garden kubeconfig injection", func() {
			var (
				namespace          string
				gardenAccessSecret *corev1.Secret
			)

			// deploymentManifest returns a rendered chart containing a single Deployment, optionally carrying the
			// inject-garden-kubeconfig label with the given value (empty string means the label is absent).
			deploymentManifest := func(injectLabelValue string) *chartrenderer.RenderedChart {
				labels := ""
				if injectLabelValue != "" {
					labels = fmt.Sprintf("\n  labels:\n    extensions.gardener.cloud/inject-garden-kubeconfig: %q", injectLabelValue)
				}
				content := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: extension-deployment
  namespace: %s%s
spec:
  template:
    spec:
      containers:
      - name: extension
        image: extension:v1.2.3
`, namespace, labels)

				manifestName := extension.Name + "/templates/deployment.yaml"
				_, manifests, err := releaseutil.SortManifests(
					map[string]string{manifestName: content},
					common.DefaultVersionSet,
					releaseutil.InstallOrder,
				)
				Expect(err).NotTo(HaveOccurred())

				return &chartrenderer.RenderedChart{
					ChartName: extension.Name,
					Manifests: manifests,
				}
			}

			BeforeEach(func() {
				namespace = fmt.Sprintf("runtime-extension-%s", extensionName)
				gardenAccessSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "garden-access-extension", Namespace: namespace}}

				extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = new(true)
				ociRegistry.AddArtifact(&gardencorev1.OCIRepository{Ref: &ociRef}, []byte("extension-chart"))

				DeferCleanup(test.WithVar(&retry.Until, func(_ context.Context, _ time.Duration, _ retry.Func) error {
					return nil
				}))
			})

			It("should inject the garden kubeconfig", func() {
				chartRenderer.EXPECT().RenderArchive([]byte("extension-chart"), extension.Name, namespace, gomock.Any()).Return(deploymentManifest("true"), nil)

				_, err := runtime.Reconcile(ctx, log, virtualClientSet, extension)
				Expect(err).NotTo(HaveOccurred())

				By("Verify the garden access secret was created in the extension namespace")
				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(gardenAccessSecret), gardenAccessSecret)).To(Succeed())
				Expect(gardenAccessSecret.Labels).To(HaveKeyWithValue("resources.gardener.cloud/class", "garden"))

				By("Verify the generic garden kubeconfig secret was created in the extension namespace")
				secretList := &corev1.SecretList{}
				Expect(runtimeClient.List(ctx, secretList, client.InNamespace(namespace))).To(Succeed())
				var genericKubeconfigSecret *corev1.Secret
				for i, s := range secretList.Items {
					if strings.HasPrefix(s.Name, "generic-garden-kubeconfig-") {
						genericKubeconfigSecret = &secretList.Items[i]
						break
					}
				}
				Expect(genericKubeconfigSecret).NotTo(BeNil())
				Expect(genericKubeconfigSecret.Data).To(HaveKey("kubeconfig"))
				Expect(string(genericKubeconfigSecret.Data["kubeconfig"])).To(ContainSubstring("https://virtual-garden-kube-apiserver.garden.svc.cluster.local"))

				By("Verify the rendered Deployment got the garden kubeconfig injected")
				mrSecretData := managedResourceSecretData(ctx, runtimeClient, extensionName)
				Expect(mrSecretData).To(HaveLen(1))
				var manifest string
				for _, v := range mrSecretData {
					manifest = string(v)
				}
				Expect(manifest).To(ContainSubstring("GARDEN_KUBECONFIG"))
				Expect(manifest).To(ContainSubstring("/var/run/secrets/gardener.cloud/garden/generic-kubeconfig"))
				Expect(manifest).To(ContainSubstring(genericKubeconfigSecret.Name))
				Expect(manifest).To(ContainSubstring("garden-access-extension"))
			})

			It("should not inject and should delete the access secret when the workload opts out", func() {
				Expect(runtimeClient.Create(ctx, gardenAccessSecret.DeepCopy())).To(Succeed())
				chartRenderer.EXPECT().RenderArchive([]byte("extension-chart"), extension.Name, namespace, gomock.Any()).Return(deploymentManifest("false"), nil)

				_, err := runtime.Reconcile(ctx, log, virtualClientSet, extension)
				Expect(err).NotTo(HaveOccurred())

				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(gardenAccessSecret), gardenAccessSecret)).To(BeNotFoundError())

				mrSecretData := managedResourceSecretData(ctx, runtimeClient, extensionName)
				var manifest string
				for _, v := range mrSecretData {
					manifest = string(v)
				}
				Expect(manifest).NotTo(ContainSubstring("GARDEN_KUBECONFIG"))
			})

			It("should not inject when InjectGardenKubeconfig is disabled", func() {
				extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = new(false)
				chartRenderer.EXPECT().RenderArchive([]byte("extension-chart"), extension.Name, namespace, gomock.Any()).Return(deploymentManifest("true"), nil)

				_, err := runtime.Reconcile(ctx, log, virtualClientSet, extension)
				Expect(err).NotTo(HaveOccurred())

				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(gardenAccessSecret), gardenAccessSecret)).To(BeNotFoundError())

				mrSecretData := managedResourceSecretData(ctx, runtimeClient, extensionName)
				var manifest string
				for _, v := range mrSecretData {
					manifest = string(v)
				}
				Expect(manifest).NotTo(ContainSubstring("GARDEN_KUBECONFIG"))
			})

			It("should not inject when InjectGardenKubeconfig when the clientSet is nil", func() {
				extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = new(true)
				chartRenderer.EXPECT().RenderArchive([]byte("extension-chart"), extension.Name, namespace, gomock.Any()).Return(deploymentManifest("true"), nil)

				_, err := runtime.Reconcile(ctx, log, nil, extension)
				Expect(err).NotTo(HaveOccurred())

				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(gardenAccessSecret), gardenAccessSecret)).To(BeNotFoundError())

				mrSecretData := managedResourceSecretData(ctx, runtimeClient, extensionName)
				var manifest string
				for _, v := range mrSecretData {
					manifest = string(v)
				}
				Expect(manifest).NotTo(ContainSubstring("GARDEN_KUBECONFIG"))
			})
		})
	})

	Describe("#Delete", func() {
		It("should succeed if extension was not deployed before", func() {
			_, err := runtime.Delete(ctx, log, extension)
			Expect(err).NotTo(HaveOccurred())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())
		})

		It("should succeed if extension was deployed before", func() {
			namespace := fmt.Sprintf("runtime-extension-%s", extensionName)
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			Expect(runtimeClient.Create(ctx, ns)).To(Succeed())
			Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "garden-extension-test-extension-23b70", Namespace: "garden"}})).To(Succeed())
			Expect(runtimeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "garden-extension-test-extension-23b70", Namespace: "garden"}, Spec: resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "garden-extension-test-extension-23b70"}}}})).To(Succeed())

			_, err := runtime.Delete(ctx, log, extension)
			Expect(err).NotTo(HaveOccurred())

			mrList := &resourcesv1alpha1.ManagedResourceList{}
			Expect(runtimeClient.List(ctx, mrList)).To(Succeed())
			Expect(mrList.Items).To(BeEmpty())

			secretList := &corev1.SecretList{}
			Expect(runtimeClient.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())

			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(ns), ns)).To(BeNotFoundError())
		})
	})
})

// managedResourceSecretData returns the data of the secret referenced by the runtime extension's ManagedResource.
func managedResourceSecretData(ctx context.Context, c client.Client, extensionName string) map[string][]byte {
	GinkgoHelper()

	mr := &resourcesv1alpha1.ManagedResource{}
	Expect(c.Get(ctx, client.ObjectKey{Name: gardeneroperator.ExtensionRuntimeManagedResourceName(extensionName), Namespace: "garden"}, mr)).To(Succeed())
	Expect(mr.Spec.SecretRefs).NotTo(BeEmpty())

	data := map[string][]byte{}
	for _, ref := range mr.Spec.SecretRefs {
		secret := &corev1.Secret{}
		Expect(c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: "garden"}, secret)).To(Succeed())
		for k, v := range secret.Data {
			data[k] = v
		}
	}
	return data
}
