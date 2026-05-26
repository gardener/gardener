// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"
	"crypto/rand"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsmockgenericactuator "github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator/mock"
	extensionsmockcontroller "github.com/gardener/gardener/extensions/pkg/controller/mock"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	mockchartrenderer "github.com/gardener/gardener/pkg/chartrenderer/mock"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/chart"
	mockchartutil "github.com/gardener/gardener/pkg/utils/chart/mocks"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	namespace               = "test"
	cloudProviderConfigName = "cloud-provider-config"
	chartName               = "chartName"
	renderedContent         = "renderedContent"
	providerName            = "provider-test"

	caNameControlPlane = "ca-" + providerName + "-controlplane"

	seedVersion  = "1.33.0"
	shootVersion = "1.33.0"
)

func TestControlPlane(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extensions Controller ControlPlane GenericActuator Suite")
}

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVars(
		&secretsutils.GenerateRandomString, secretsutils.FakeGenerateRandomString,
		&secretsutils.GenerateKey, secretsutils.FakeGenerateKey,
	))
})

var _ = Describe("Actuator", func() {
	var (
		ctrl              *gomock.Controller
		fakeClock         *testclock.FakeClock
		fakeClient        client.Client
		newSecretsManager newSecretsManagerFunc

		ctx                    = context.TODO()
		webhookServerNamespace = "extension-foo-12345"

		cp *extensionsv1alpha1.ControlPlane

		cluster = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: shootVersion,
					},
				},
			},
		}

		createdMRSecretForControlPlaneSeedChart *corev1.Secret
		createdMRForControlPlaneSeedChart       *resourcesv1alpha1.ManagedResource

		cpSecret    *corev1.Secret
		cpConfigMap *corev1.ConfigMap

		resourceKeyConfigurationSeedChart        client.ObjectKey
		createdMRSecretForConfigurationSeedChart *corev1.Secret
		createdMRForConfigurationSeedChart       *resourcesv1alpha1.ManagedResource

		resourceKeyCPShootChart        client.ObjectKey
		createdMRSecretForCPShootChart *corev1.Secret
		createdMRForCPShootChart       *resourcesv1alpha1.ManagedResource

		resourceKeyCPShootCRDsChart        client.ObjectKey
		createdMRSecretForCPShootCRDsChart *corev1.Secret
		createdMRForCPShootCRDsChart       *resourcesv1alpha1.ManagedResource

		resourceKeyStorageClassesChart        client.ObjectKey
		createdMRSecretForStorageClassesChart *corev1.Secret
		createdMRForStorageClassesChart       *resourcesv1alpha1.ManagedResource

		resourceKeyShootWebhooks  client.ObjectKey
		createdMRForShootWebhooks *resourcesv1alpha1.ManagedResource

		imageVector = imagevector.ImageVector([]*imagevector.ImageSource{})

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			cloudProviderConfigName:                  "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
			caNameControlPlane:                       "4eaacd526bec01da8f0fbf602077712294caffe81f9fb366bb7d5dea91204246",
			"cloud-controller-manager":               "1dac327f1cd4dd1c108446bb8b414e7a0551f792ad9ff1139b743a0046a1d659",
		}
		checksumsNoConfig = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			caNameControlPlane:                       "4eaacd526bec01da8f0fbf602077712294caffe81f9fb366bb7d5dea91204246",
			"cloud-controller-manager":               "1dac327f1cd4dd1c108446bb8b414e7a0551f792ad9ff1139b743a0046a1d659",
		}

		configChartValues = map[string]any{
			"cloudProviderConfig": `[Global]`,
		}

		controlPlaneChartValues = map[string]any{
			"clusterName": namespace,
		}

		controlPlaneShootChartValues = map[string]any{
			"foo": "bar",
		}

		controlPlaneShootCRDsChartValues = map[string]any{
			"foo": "bar",
		}

		storageClassesChartValues = map[string]any{
			"foo": "bar",
		}

		shootAccessSecretsFunc func(string) []*gardenerutils.AccessSecret

		logger = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		newSecretsManager = func(ctx context.Context, logger logr.Logger, _ clock.Clock, _ client.Client, cluster *extensionscontroller.Cluster, identity string, secretConfigs []extensionssecretsmanager.SecretConfigWithOptions) (secretsmanager.Interface, error) {
			// use fake clock and client, pass on the rest
			return extensionssecretsmanager.SecretsManagerForCluster(ctx, logger, fakeClock, fakeClient, cluster, identity, secretConfigs)
		}

		deterministicReader := strings.NewReader(strings.Repeat("-", 10000))
		fakeClock = testclock.NewFakeClock(time.Unix(1649848746, 0))

		DeferCleanup(test.WithVars(
			&rand.Reader, deterministicReader,
			&secretsutils.Clock, fakeClock,
		))

		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "control-plane", Namespace: namespace},
			Spec:       extensionsv1alpha1.ControlPlaneSpec{},
		}

		shootAccessSecretsFunc = func(namespace string) []*gardenerutils.AccessSecret {
			return []*gardenerutils.AccessSecret{gardenerutils.NewShootAccessSecret("new-cp", namespace)}
		}

		cpSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.SecretNameCloudProvider, Namespace: namespace},
			Data:       map[string][]byte{"foo": []byte("bar")},
		}
		cpConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cloudProviderConfigName, Namespace: namespace},
			Data:       map[string]string{"abc": "xyz"},
		}

		resourceKeyConfigurationSeedChart = client.ObjectKey{Namespace: namespace, Name: ControlPlaneSeedConfigurationChartResourceName}
		createdMRSecretForConfigurationSeedChart = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedConfigurationChartResourceName, Namespace: namespace},
			Data:       map[string][]byte{chartName: []byte(renderedContent)},
			Type:       corev1.SecretTypeOpaque,
		}
		createdMRForConfigurationSeedChart = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedConfigurationChartResourceName, Namespace: namespace},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: ptr.To(v1beta1constants.SeedResourceManagerClass),
				SecretRefs: []corev1.LocalObjectReference{
					{Name: ControlPlaneSeedConfigurationChartResourceName},
				},
			},
		}

		createdMRSecretForControlPlaneSeedChart = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedChartResourceName, Namespace: namespace},
			Data:       map[string][]byte{chartName: []byte(renderedContent)},
			Type:       corev1.SecretTypeOpaque,
		}
		createdMRForControlPlaneSeedChart = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedChartResourceName, Namespace: namespace},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: ptr.To(v1beta1constants.SeedResourceManagerClass),
				SecretRefs: []corev1.LocalObjectReference{
					{Name: ControlPlaneSeedConfigurationChartResourceName},
				},
			},
		}

		resourceKeyCPShootChart = client.ObjectKey{Namespace: namespace, Name: ControlPlaneShootChartResourceName}
		createdMRSecretForCPShootChart = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneShootChartResourceName, Namespace: namespace},
			Data:       map[string][]byte{chartName: []byte(renderedContent)},
			Type:       corev1.SecretTypeOpaque,
		}
		createdMRForCPShootChart = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneShootChartResourceName, Namespace: namespace},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: ControlPlaneShootChartResourceName},
				},
				InjectLabels:              map[string]string{v1beta1constants.ShootNoCleanup: "true"},
				KeepObjects:               ptr.To(false),
				ForceOverwriteAnnotations: ptr.To(false),
			},
		}

		resourceKeyCPShootCRDsChart = client.ObjectKey{Namespace: namespace, Name: ControlPlaneShootCRDsChartResourceName}
		createdMRSecretForCPShootCRDsChart = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneShootCRDsChartResourceName, Namespace: namespace},
			Data:       map[string][]byte{chartName: []byte(renderedContent)},
			Type:       corev1.SecretTypeOpaque,
		}
		createdMRForCPShootCRDsChart = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneShootCRDsChartResourceName, Namespace: namespace},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: ControlPlaneShootCRDsChartResourceName},
				},
				InjectLabels:              map[string]string{v1beta1constants.ShootNoCleanup: "true"},
				KeepObjects:               ptr.To(false),
				ForceOverwriteAnnotations: ptr.To(false),
			},
		}

		resourceKeyStorageClassesChart = client.ObjectKey{Namespace: namespace, Name: StorageClassesChartResourceName}
		createdMRSecretForStorageClassesChart = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: StorageClassesChartResourceName, Namespace: namespace},
			Data:       map[string][]byte{chartName: []byte(renderedContent)},
			Type:       corev1.SecretTypeOpaque,
		}
		createdMRForStorageClassesChart = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: StorageClassesChartResourceName, Namespace: namespace},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: StorageClassesChartResourceName},
				},
				InjectLabels:              map[string]string{v1beta1constants.ShootNoCleanup: "true"},
				KeepObjects:               ptr.To(false),
				ForceOverwriteAnnotations: ptr.To(true),
			},
		}

		resourceKeyShootWebhooks = client.ObjectKey{Namespace: namespace, Name: ShootWebhooksResourceName}
		createdMRForShootWebhooks = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: namespace},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: ShootWebhooksResourceName},
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	DescribeTable("#Reconcile",
		func(configName string, checksums map[string]string, webhookConfig *admissionregistrationv1.MutatingWebhookConfiguration, withShootCRDsChart bool) {
			var atomicWebhookConfig *atomic.Value
			if webhookConfig != nil {
				atomicWebhookConfig = &atomic.Value{}
				atomicWebhookConfig.Store(&extensionswebhook.Configs{MutatingWebhookConfig: webhookConfig})
			}

			c := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				// Inject healthy status for ManagedResources.
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if err := c.Get(ctx, key, obj, opts...); err != nil {
							return err
						}
						if mr, ok := obj.(*resourcesv1alpha1.ManagedResource); ok {
							mr.Status = resourcesv1alpha1.ManagedResourceStatus{
								ObservedGeneration: mr.Generation,
								Conditions: []gardencorev1beta1.Condition{
									{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
									{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
									{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse},
								},
							}
						}
						return nil
					},
				},
				).
				Build()

			gardenerClientset := fakekubernetes.NewClientSetBuilder().WithVersion(seedVersion).Build()

			// Pre-create secrets and configmaps that are expected to be used by the actuator.
			Expect(c.Create(ctx, cpSecret.DeepCopy())).To(Succeed())
			if configName != "" {
				Expect(c.Create(ctx, cpConfigMap.DeepCopy())).To(Succeed())
			}

			// Create mock chart renderer and factory
			chartRenderer := mockchartrenderer.NewMockInterface(ctrl)
			crf := extensionsmockcontroller.NewMockChartRendererFactory(ctrl)
			crf.EXPECT().NewChartRendererForShoot(shootVersion).Return(chartRenderer, nil)

			// Create mock charts
			var configChart chart.Interface
			if configName != "" {
				configChartMock := mockchartutil.NewMockInterface(ctrl)
				configChartMock.EXPECT().Render(chartRenderer, namespace, imageVector, seedVersion, shootVersion, configChartValues).Return(chartName, []byte(renderedContent), nil)
				configChart = configChartMock
			}

			ccmChart := mockchartutil.NewMockInterface(ctrl)
			ccmChart.EXPECT().Render(chartRenderer, namespace, imageVector, seedVersion, shootVersion, controlPlaneChartValues).Return(chartName, []byte(renderedContent), nil)

			ccmShootChart := mockchartutil.NewMockInterface(ctrl)
			ccmShootChart.EXPECT().Render(chartRenderer, metav1.NamespaceSystem, imageVector, shootVersion, shootVersion, controlPlaneShootChartValues).Return(chartName, []byte(renderedContent), nil)

			var cpShootCRDsChart chart.Interface
			if withShootCRDsChart {
				cpShootCRDsChartMock := mockchartutil.NewMockInterface(ctrl)
				cpShootCRDsChartMock.EXPECT().Render(chartRenderer, metav1.NamespaceSystem, imageVector, shootVersion, shootVersion, controlPlaneShootCRDsChartValues).Return(chartName, []byte(renderedContent), nil)
				cpShootCRDsChart = cpShootCRDsChartMock
			}
			storageClassesChart := mockchartutil.NewMockInterface(ctrl)
			storageClassesChart.EXPECT().Render(chartRenderer, metav1.NamespaceSystem, imageVector, shootVersion, shootVersion, storageClassesChartValues).Return(chartName, []byte(renderedContent), nil)

			// Create mock values provider
			vp := extensionsmockgenericactuator.NewMockValuesProvider(ctrl)
			if configName != "" {
				vp.EXPECT().GetConfigChartValues(ctx, cp, cluster).Return(configChartValues, nil)
			}
			vp.EXPECT().GetControlPlaneChartValues(ctx, cp, cluster, gomock.Any(), checksums, false).Return(controlPlaneChartValues, nil)
			vp.EXPECT().GetControlPlaneShootChartValues(ctx, cp, cluster, gomock.Any(), checksums).Return(controlPlaneShootChartValues, nil)
			if withShootCRDsChart {
				vp.EXPECT().GetControlPlaneShootCRDsChartValues(ctx, cp, cluster).Return(controlPlaneShootCRDsChartValues, nil)
			}
			vp.EXPECT().GetStorageClassesChartValues(ctx, cp, cluster).Return(storageClassesChartValues, nil)

			// Create actuator
			a := &actuator{
				providerName:               providerName,
				secretConfigsFunc:          getSecretsConfigs,
				shootAccessSecretsFunc:     shootAccessSecretsFunc,
				configChart:                configChart,
				controlPlaneChart:          ccmChart,
				controlPlaneShootChart:     ccmShootChart,
				controlPlaneShootCRDsChart: cpShootCRDsChart,
				storageClassesChart:        storageClassesChart,
				vp:                         vp,
				chartRendererFactory:       crf,
				imageVector:                imageVector,
				configName:                 configName,
				atomicShootWebhookConfig:   atomicWebhookConfig,
				webhookServerNamespace:     webhookServerNamespace,
				gardenerClientset:          gardenerClientset,
				client:                     c,
				newSecretsManager:          newSecretsManager,
			}

			// Call Reconcile method and check the result
			requeue, err := a.Reconcile(ctx, logger, cp, cluster)
			Expect(requeue).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			expectSecretsManagedBySecretsManager(ctx, fakeClient, "wanted secrets should get created",
				"ca-provider-test-controlplane-05334c48", "ca-provider-test-controlplane-bundle-67817cb3",
				"cloud-controller-manager-c249bd1b",
			)

			if webhookConfig != nil {
				compressedData, err := test.BrotliCompressionForManifests(`apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata: {}
webhooks:
- admissionReviewVersions: null
  clientConfig: {}
  name: ""
  sideEffects: null
`)
				Expect(err).NotTo(HaveOccurred())

				createdMRSecretForShootWebhooks := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: namespace},
					Data:       map[string][]byte{"data.yaml.br": compressedData},
					Type:       corev1.SecretTypeOpaque,
				}

				expectManagedResourceCreated(ctx, c, createdMRSecretForShootWebhooks, createdMRForShootWebhooks)
			}

			expectManagedResourceCreated(ctx, c, createdMRSecretForCPShootChart, createdMRForCPShootChart)

			if withShootCRDsChart {
				expectManagedResourceCreated(ctx, c, createdMRSecretForCPShootCRDsChart, createdMRForCPShootCRDsChart)
			}

			expectManagedResourceCreated(ctx, c, createdMRSecretForStorageClassesChart, createdMRForStorageClassesChart)

			if configName != "" {
				expectManagedResourceCreated(ctx, c, createdMRSecretForConfigurationSeedChart, createdMRForConfigurationSeedChart)
			}

			expectManagedResourceCreated(ctx, c, createdMRSecretForControlPlaneSeedChart, createdMRForControlPlaneSeedChart)
		},
		Entry("should deploy secrets and apply charts with correct parameters", cloudProviderConfigName, checksums, &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}}}, true),
		Entry("should deploy secrets and apply charts with correct parameters (no config)", "", checksumsNoConfig, &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}}}, true),
		Entry("should deploy secrets and apply charts with correct parameters (no webhook)", cloudProviderConfigName, checksums, nil, true),
		Entry("should deploy secrets and apply charts with correct parameters (no shoot CRDs chart)", cloudProviderConfigName, checksums, &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}}}, false),
	)

	DescribeTable("#Delete",
		func(configName string, webhookConfig *admissionregistrationv1.MutatingWebhookConfiguration, withShootCRDsChart bool) {
			var atomicWebhookConfig *atomic.Value
			if webhookConfig != nil {
				atomicWebhookConfig = &atomic.Value{}
				atomicWebhookConfig.Store(&extensionswebhook.Configs{MutatingWebhookConfig: webhookConfig})
			}

			// Create mock Gardener clientset and chart applier
			gardenerClientset := kubernetesmock.NewMockInterface(ctrl)
			gardenerClientset.EXPECT().Version().Return(seedVersion).AnyTimes()

			// Create mock values provider
			vp := extensionsmockgenericactuator.NewMockValuesProvider(ctrl)

			// Create mock chart renderer and factory
			chartRenderer := mockchartrenderer.NewMockInterface(ctrl)
			crf := extensionsmockcontroller.NewMockChartRendererFactory(ctrl)
			crf.EXPECT().NewChartRendererForShoot(shootVersion).Return(chartRenderer, nil)

			// Create mock clients
			c := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			var cpShootCRDsChart chart.Interface
			if withShootCRDsChart {
				cpShootCRDsChartMock := mockchartutil.NewMockInterface(ctrl)
				cpShootCRDsChart = cpShootCRDsChartMock
			}

			// Create mock charts
			var configChart chart.Interface
			if configName != "" {
				configChartMock := mockchartutil.NewMockInterface(ctrl)
				vp.EXPECT().GetConfigChartValues(ctx, cp, cluster).Return(configChartValues, nil)
				configChartMock.EXPECT().Render(chartRenderer, namespace, imageVector, shootVersion, shootVersion, configChartValues).Return(chartName, []byte(renderedContent), nil)
				configChart = configChartMock
			}

			// Delete mock controlplane chart
			ccmChart := mockchartutil.NewMockInterface(ctrl)

			// Create actuator
			a := &actuator{
				providerName:               providerName,
				secretConfigsFunc:          getSecretsConfigs,
				shootAccessSecretsFunc:     shootAccessSecretsFunc,
				configChart:                configChart,
				controlPlaneChart:          ccmChart,
				controlPlaneShootChart:     nil,
				controlPlaneShootCRDsChart: cpShootCRDsChart,
				storageClassesChart:        nil,
				vp:                         vp,
				configName:                 configName,
				atomicShootWebhookConfig:   atomicWebhookConfig,
				webhookServerNamespace:     webhookServerNamespace,
				gardenerClientset:          gardenerClientset,
				client:                     c,
				newSecretsManager:          newSecretsManager,

				chartRendererFactory: crf,
				imageVector:          imageVector,
			}

			// Pre-create resources that should be cleaned up by Delete
			if configName != "" {
				Expect(c.Create(ctx, createdMRSecretForConfigurationSeedChart.DeepCopy())).To(Succeed())
				Expect(c.Create(ctx, createdMRForConfigurationSeedChart.DeepCopy())).To(Succeed())
			}

			Expect(c.Create(ctx, createdMRSecretForStorageClassesChart.DeepCopy())).To(Succeed())
			Expect(c.Create(ctx, createdMRForStorageClassesChart.DeepCopy())).To(Succeed())

			if withShootCRDsChart {
				Expect(c.Create(ctx, createdMRSecretForCPShootCRDsChart.DeepCopy())).To(Succeed())
				Expect(c.Create(ctx, createdMRForCPShootCRDsChart.DeepCopy())).To(Succeed())
			}

			Expect(c.Create(ctx, createdMRSecretForCPShootChart.DeepCopy())).To(Succeed())
			Expect(c.Create(ctx, createdMRForCPShootChart.DeepCopy())).To(Succeed())
			Expect(c.Create(ctx, createdMRSecretForControlPlaneSeedChart.DeepCopy())).To(Succeed())
			Expect(c.Create(ctx, createdMRForControlPlaneSeedChart.DeepCopy())).To(Succeed())

			if webhookConfig != nil {
				Expect(c.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: namespace},
				})).To(Succeed())
				Expect(c.Create(ctx, createdMRForShootWebhooks.DeepCopy())).To(Succeed())
			}
			Expect(c.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: shootAccessSecretsFunc(namespace)[0].Secret.Name, Namespace: namespace},
			})).To(Succeed())

			// Call Delete method and check the result
			Expect(a.Delete(ctx, logger, cp, cluster)).To(Succeed())

			Expect(c.Get(ctx, resourceKeyConfigurationSeedChart, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
			Expect(c.Get(ctx, resourceKeyConfigurationSeedChart, &corev1.Secret{})).To(BeNotFoundError())

			Expect(c.Get(ctx, resourceKeyStorageClassesChart, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
			Expect(c.Get(ctx, resourceKeyStorageClassesChart, &corev1.Secret{})).To(BeNotFoundError())

			if withShootCRDsChart {
				Expect(c.Get(ctx, resourceKeyCPShootCRDsChart, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
				Expect(c.Get(ctx, resourceKeyCPShootCRDsChart, &corev1.Secret{})).To(BeNotFoundError())
			}

			Expect(c.Get(ctx, resourceKeyCPShootChart, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
			Expect(c.Get(ctx, resourceKeyCPShootChart, &corev1.Secret{})).To(BeNotFoundError())

			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ControlPlaneSeedChartResourceName}, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ControlPlaneSeedChartResourceName}, &corev1.Secret{})).To(BeNotFoundError())

			if webhookConfig != nil {
				Expect(c.Get(ctx, resourceKeyShootWebhooks, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
				Expect(c.Get(ctx, resourceKeyShootWebhooks, &corev1.Secret{})).To(BeNotFoundError())
			}

			Expect(c.Get(ctx, client.ObjectKey{Name: shootAccessSecretsFunc(namespace)[0].Secret.Name, Namespace: namespace}, &corev1.Secret{})).To(BeNotFoundError())

			expectSecretsManagedBySecretsManager(ctx, fakeClient, "all secrets managed by SecretsManager should get cleaned up")
		},
		Entry("should delete secrets and charts", cloudProviderConfigName, &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}}}, true),
		Entry("should delete secrets and charts (no config)", "", &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}}}, true),
		Entry("should delete secrets and charts (no webhook)", cloudProviderConfigName, nil, true),
		Entry("should delete secrets and charts (no shoot CRDs chart)", cloudProviderConfigName, &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}}}, false),
	)
})

func getSecretsConfigs(namespace string) []extensionssecretsmanager.SecretConfigWithOptions {
	return []extensionssecretsmanager.SecretConfigWithOptions{
		{
			Config: &secretsutils.CertificateSecretConfig{
				Name:       caNameControlPlane,
				CommonName: caNameControlPlane,
				CertType:   secretsutils.CACert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		},
		{
			Config: &secretsutils.CertificateSecretConfig{
				Name:       "cloud-controller-manager",
				CommonName: "cloud-controller-manager",
				DNSNames:   kubernetesutils.DNSNamesForService("cloud-controller-manager", namespace),
				CertType:   secretsutils.ServerCert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane)},
		},
	}
}

var (
	objectIdentifier = Identifier(func(obj any) string {
		switch o := obj.(type) {
		case corev1.Secret:
			return o.GetName()
		}
		return obj.(client.Object).GetName()
	})
	alwaysMatch = And()
)

func consistOfObjects(names ...string) gomegatypes.GomegaMatcher {
	elements := make(Elements, len(names))
	for _, name := range names {
		elements[name] = alwaysMatch
	}

	return MatchAllElements(objectIdentifier, elements)
}

func expectSecretsManagedBySecretsManager(ctx context.Context, c client.Reader, description string, secretNames ...string) {
	secretList := &corev1.SecretList{}
	ExpectWithOffset(1, c.List(ctx, secretList, client.MatchingLabels{"managed-by": "secrets-manager"})).To(Succeed())
	ExpectWithOffset(1, secretList.Items).To(consistOfObjects(secretNames...), description)
}

func expectManagedResourceCreated(ctx context.Context, c client.Client, s *corev1.Secret, mr *resourcesv1alpha1.ManagedResource) {
	utilruntime.Must(kubernetesutils.MakeUnique(s))

	mrSecret := &corev1.Secret{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: s.Name}, mrSecret)).To(Succeed())
	Expect(mrSecret.Data).To(Equal(s.Data))
	Expect(mrSecret.Type).To(Equal(corev1.SecretTypeOpaque))

	managedResource := &resourcesv1alpha1.ManagedResource{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: mr.Namespace, Name: mr.Name}, managedResource)).To(Succeed())
	Expect(managedResource.Spec.Class).To(Equal(mr.Spec.Class))
	Expect(managedResource.Spec.SecretRefs).To(ConsistOf(corev1.LocalObjectReference{Name: s.Name}))
	Expect(managedResource.Annotations).To(HaveKeyWithValue(references.AnnotationKey(references.KindSecret, s.Name), s.Name))
}
