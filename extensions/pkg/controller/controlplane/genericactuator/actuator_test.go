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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

const (
	namespace               = "test"
	cloudProviderConfigName = "cloud-provider-config"
	chartName               = "chartName"
	renderedContent         = "renderedContent"
	providerName            = "provider-test"

	caNameControlPlane         = "ca-" + providerName + "-controlplane"
	caNameControlPlaneExposure = caNameControlPlane + "-exposure"

	seedVersion  = "1.28.0"
	shootVersion = "1.28.0"
)

var (
	vFalse, vTrue = false, true
	pFalse, pTrue = &vFalse, &vTrue

	fakeClock *testclock.FakeClock
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
		fakeClient        client.Client
		newSecretsManager newSecretsManagerFunc

		ctx                    = context.TODO()
		webhookServerNamespace = "extension-foo-12345"

		cp         *extensionsv1alpha1.ControlPlane
		cpExposure = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "control-plane-exposure", Namespace: namespace},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				Purpose: getPurposeExposure(),
			},
		}

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

		cpSecretKey    client.ObjectKey
		cpConfigMapKey client.ObjectKey
		cpSecret       *corev1.Secret
		cpConfigMap    *corev1.ConfigMap

		resourceKeyCPShootChart        client.ObjectKey
		createdMRSecretForCPShootChart *corev1.Secret
		createdMRForCPShootChart       *resourcesv1alpha1.ManagedResource
		deletedMRSecretForCPShootChart *corev1.Secret
		deletedMRForCPShootChart       *resourcesv1alpha1.ManagedResource

		resourceKeyCPShootCRDsChart        client.ObjectKey
		createdMRSecretForCPShootCRDsChart *corev1.Secret
		createdMRForCPShootCRDsChart       *resourcesv1alpha1.ManagedResource
		deletedMRSecretForCPShootCRDsChart *corev1.Secret
		deletedMRForCPShootCRDsChart       *resourcesv1alpha1.ManagedResource

		resourceKeyStorageClassesChart        client.ObjectKey
		createdMRSecretForStorageClassesChart *corev1.Secret
		createdMRForStorageClassesChart       *resourcesv1alpha1.ManagedResource
		deletedMRSecretForStorageClassesChart *corev1.Secret
		deletedMRForStorageClassesChart       *resourcesv1alpha1.ManagedResource

		resourceKeyShootWebhooks        client.ObjectKey
		createdMRForShootWebhooks       *resourcesv1alpha1.ManagedResource
		deletedMRForShootWebhooks       *resourcesv1alpha1.ManagedResource
		deletedMRSecretForShootWebhooks *corev1.Secret

		imageVector = imagevector.ImageVector([]*imagevector.ImageSource{})

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			cloudProviderConfigName:                  "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
			caNameControlPlane:                       "3e6425b85bb75f33df7c16387b6999eb0f2d3c3e0a81afb4739626c69a79887b",
			"cloud-controller-manager":               "47448ddc1d8c7b02d20125b4f0914acf8402ee9d763d5bdd48634fcbf8d75b1d",
		}
		checksumsNoConfig = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			caNameControlPlane:                       "3e6425b85bb75f33df7c16387b6999eb0f2d3c3e0a81afb4739626c69a79887b",
			"cloud-controller-manager":               "47448ddc1d8c7b02d20125b4f0914acf8402ee9d763d5bdd48634fcbf8d75b1d",
		}
		exposureChecksums = map[string]string{
			caNameControlPlaneExposure: "98637da60735fb6f44615e032c30b3f5fc12d0af5df057fa2741aa97554db9a3",
			"lb-readvertiser":          "c9157ced4dfc92686c2bf62e2f1a9f0d12f6d9ac1d835b877d9651b126b56e67",
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

		controlPlaneExposureChartValues = map[string]any{
			"replicas": 1,
		}

		shootAccessSecretsFunc         func(string) []*gardenerutils.AccessSecret
		exposureShootAccessSecretsFunc func(string) []*gardenerutils.AccessSecret

		errNotFound = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
		logger      = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		fakeClient = fakeclient.NewClientBuilder().Build()
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
		exposureShootAccessSecretsFunc = func(namespace string) []*gardenerutils.AccessSecret {
			return []*gardenerutils.AccessSecret{gardenerutils.NewShootAccessSecret("new-cp-exposure", namespace)}
		}

		cpSecretKey = client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}
		cpConfigMapKey = client.ObjectKey{Namespace: namespace, Name: cloudProviderConfigName}
		cpSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.SecretNameCloudProvider, Namespace: namespace},
			Data:       map[string][]byte{"foo": []byte("bar")},
		}
		cpConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cloudProviderConfigName, Namespace: namespace},
			Data:       map[string]string{"abc": "xyz"},
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
				KeepObjects:               pFalse,
				ForceOverwriteAnnotations: pFalse,
			},
		}
		deletedMRSecretForCPShootChart = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneShootChartResourceName, Namespace: namespace},
		}
		deletedMRForCPShootChart = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneShootChartResourceName, Namespace: namespace},
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
				KeepObjects:               pFalse,
				ForceOverwriteAnnotations: pFalse,
			},
		}
		deletedMRSecretForCPShootCRDsChart = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneShootCRDsChartResourceName, Namespace: namespace},
		}
		deletedMRForCPShootCRDsChart = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneShootCRDsChartResourceName, Namespace: namespace},
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
				KeepObjects:               pFalse,
				ForceOverwriteAnnotations: pTrue,
			},
		}
		deletedMRSecretForStorageClassesChart = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: StorageClassesChartResourceName, Namespace: namespace},
		}
		deletedMRForStorageClassesChart = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: StorageClassesChartResourceName, Namespace: namespace},
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
		deletedMRForShootWebhooks = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: namespace},
		}
		deletedMRSecretForShootWebhooks = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: namespace},
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

			// Create mock client
			c := mockclient.NewMockClient(ctrl)

			if webhookConfig != nil {
				compressedData, err := test.BrotliCompressionForManifests(`apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  creationTimestamp: null
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

				utilruntime.Must(kubernetesutils.MakeUnique(createdMRSecretForShootWebhooks))
				c.EXPECT().Get(ctx, client.ObjectKeyFromObject(createdMRSecretForShootWebhooks), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
				c.EXPECT().Create(ctx, createdMRSecretForShootWebhooks).Return(nil)
				c.EXPECT().Get(ctx, resourceKeyShootWebhooks, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
				createdMRForShootWebhooks.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: createdMRSecretForShootWebhooks.Name}}
				utilruntime.Must(references.InjectAnnotations(createdMRForShootWebhooks))
				c.EXPECT().Create(ctx, createdMRForShootWebhooks).Return(nil)
			}

			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			if configName != "" {
				c.EXPECT().Get(ctx, cpConfigMapKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cpConfigMap))
			}

			utilruntime.Must(kubernetesutils.MakeUnique(createdMRSecretForCPShootChart))
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(createdMRSecretForCPShootChart), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
			c.EXPECT().Create(ctx, createdMRSecretForCPShootChart).Return(nil)
			c.EXPECT().Get(ctx, resourceKeyCPShootChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
			createdMRForCPShootChart.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: createdMRSecretForCPShootChart.Name}}
			utilruntime.Must(references.InjectAnnotations(createdMRForCPShootChart))
			c.EXPECT().Create(ctx, createdMRForCPShootChart).Return(nil)

			if withShootCRDsChart {
				utilruntime.Must(kubernetesutils.MakeUnique(createdMRSecretForCPShootCRDsChart))
				c.EXPECT().Get(ctx, client.ObjectKeyFromObject(createdMRSecretForCPShootCRDsChart), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
				c.EXPECT().Create(ctx, createdMRSecretForCPShootCRDsChart).Return(nil)
				c.EXPECT().Get(ctx, resourceKeyCPShootCRDsChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
				createdMRForCPShootCRDsChart.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: createdMRSecretForCPShootCRDsChart.Name}}
				utilruntime.Must(references.InjectAnnotations(createdMRForCPShootCRDsChart))
				c.EXPECT().Create(ctx, createdMRForCPShootCRDsChart).Return(nil)
			}

			utilruntime.Must(kubernetesutils.MakeUnique(createdMRSecretForStorageClassesChart))
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(createdMRSecretForStorageClassesChart), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
			c.EXPECT().Create(ctx, createdMRSecretForStorageClassesChart).Return(nil)
			c.EXPECT().Get(ctx, resourceKeyStorageClassesChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
			createdMRForStorageClassesChart.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: createdMRSecretForStorageClassesChart.Name}}
			utilruntime.Must(references.InjectAnnotations(createdMRForStorageClassesChart))
			c.EXPECT().Create(ctx, createdMRForStorageClassesChart).Return(nil)

			// Create mock Gardener clientset and chart applier
			gardenerClientset := kubernetesmock.NewMockInterface(ctrl)
			gardenerClientset.EXPECT().Version().Return(seedVersion).AnyTimes()

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

				createdMRSecretForConfigurationSeedChart := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedConfigurationChartResourceName, Namespace: namespace},
					Data:       map[string][]byte{chartName: []byte(renderedContent)},
					Type:       corev1.SecretTypeOpaque,
				}
				createdMRForConfigurationSeedChart := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedConfigurationChartResourceName, Namespace: namespace},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: ptr.To(v1beta1constants.SeedResourceManagerClass),
						SecretRefs: []corev1.LocalObjectReference{
							{Name: ControlPlaneSeedConfigurationChartResourceName},
						},
					},
				}
				setupManagedResourceCreation(ctx, c, createdMRSecretForConfigurationSeedChart, createdMRForConfigurationSeedChart)
			}

			ccmChart := mockchartutil.NewMockInterface(ctrl)
			ccmChart.EXPECT().Render(chartRenderer, namespace, imageVector, seedVersion, shootVersion, controlPlaneChartValues).Return(chartName, []byte(renderedContent), nil)
			setupManagedResourceCreation(ctx, c, createdMRSecretForControlPlaneSeedChart, createdMRForControlPlaneSeedChart)

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

			// Handle shoot access secrets and legacy secret cleanup
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: shootAccessSecretsFunc(namespace)[0].Secret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				Do(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
					obj.SetResourceVersion("0")
				})

			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
				Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(Equal(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      shootAccessSecretsFunc(namespace)[0].Secret.Name,
							Namespace: namespace,
							Annotations: map[string]string{
								"serviceaccount.resources.gardener.cloud/name":      shootAccessSecretsFunc(namespace)[0].ServiceAccountName,
								"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
							},
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
								"resources.gardener.cloud/class":   "shoot",
							},
							ResourceVersion: "0",
						},
						Type: corev1.SecretTypeOpaque,
					}))
				})

			// Create actuator
			a := &actuator{
				providerName:                   providerName,
				secretConfigsFunc:              getSecretsConfigs,
				shootAccessSecretsFunc:         shootAccessSecretsFunc,
				exposureSecretConfigsFunc:      nil,
				exposureShootAccessSecretsFunc: nil,
				configChart:                    configChart,
				controlPlaneChart:              ccmChart,
				controlPlaneShootChart:         ccmShootChart,
				controlPlaneShootCRDsChart:     cpShootCRDsChart,
				storageClassesChart:            storageClassesChart,
				controlPlaneExposureChart:      nil,
				vp:                             vp,
				chartRendererFactory:           crf,
				imageVector:                    imageVector,
				configName:                     configName,
				atomicShootWebhookConfig:       atomicWebhookConfig,
				webhookServerNamespace:         webhookServerNamespace,
				gardenerClientset:              gardenerClientset,
				client:                         c,
				newSecretsManager:              newSecretsManager,
			}
			// Call Reconcile method and check the result
			requeue, err := a.Reconcile(ctx, logger, cp, cluster)
			Expect(requeue).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			expectSecretsManagedBySecretsManager(fakeClient, "wanted secrets should get created",
				"ca-provider-test-controlplane-05334c48", "ca-provider-test-controlplane-bundle-bdc12448",
				"cloud-controller-manager-bc446deb",
			)
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
			c := mockclient.NewMockClient(ctrl)

			c.EXPECT().Get(gomock.Any(), resourceKeyStorageClassesChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{}))
			c.EXPECT().Delete(ctx, deletedMRForStorageClassesChart).Return(nil)
			c.EXPECT().Delete(ctx, deletedMRSecretForStorageClassesChart).Return(nil)
			var cpShootCRDsChart chart.Interface
			if withShootCRDsChart {
				cpShootCRDsChartMock := mockchartutil.NewMockInterface(ctrl)
				cpShootCRDsChart = cpShootCRDsChartMock
				c.EXPECT().Get(gomock.Any(), resourceKeyCPShootCRDsChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{}))
				c.EXPECT().Delete(ctx, deletedMRForCPShootCRDsChart).Return(nil)
				c.EXPECT().Delete(ctx, deletedMRSecretForCPShootCRDsChart).Return(nil)
				c.EXPECT().Get(gomock.Any(), resourceKeyCPShootCRDsChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, deletedMRForCPShootCRDsChart.Name))
			}

			c.EXPECT().Get(gomock.Any(), resourceKeyCPShootChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{}))
			c.EXPECT().Delete(ctx, deletedMRForCPShootChart).Return(nil)
			c.EXPECT().Delete(ctx, deletedMRSecretForCPShootChart).Return(nil)

			c.EXPECT().Get(gomock.Any(), resourceKeyStorageClassesChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, deletedMRForStorageClassesChart.Name))
			c.EXPECT().Get(gomock.Any(), resourceKeyCPShootChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, deletedMRForCPShootChart.Name))

			// Create mock charts
			var configChart chart.Interface
			if configName != "" {
				configChartMock := mockchartutil.NewMockInterface(ctrl)
				vp.EXPECT().GetConfigChartValues(ctx, cp, cluster).Return(configChartValues, nil)
				configChartMock.EXPECT().Render(chartRenderer, namespace, imageVector, shootVersion, shootVersion, configChartValues).Return(chartName, []byte(renderedContent), nil)
				configChart = configChartMock

				createdMRSecretForConfigurationSeedChart := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedConfigurationChartResourceName, Namespace: namespace},
					Data:       map[string][]byte{chartName: []byte(renderedContent)},
					Type:       corev1.SecretTypeOpaque,
				}
				createdMRForConfigurationSeedChart := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedConfigurationChartResourceName, Namespace: namespace},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: ptr.To(v1beta1constants.SeedResourceManagerClass),
						SecretRefs: []corev1.LocalObjectReference{
							{Name: ControlPlaneSeedConfigurationChartResourceName},
						},
					},
				}
				setupManagedResourceCreation(ctx, c, createdMRSecretForConfigurationSeedChart, createdMRForConfigurationSeedChart)
				c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(createdMRForConfigurationSeedChart), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource, _ ...client.PatchOption) error {
						*obj = ptr.Deref(createdMRForConfigurationSeedChart.DeepCopy(), resourcesv1alpha1.ManagedResource{})
						return nil
					})
				c.EXPECT().Delete(ctx, createdMRForConfigurationSeedChart).Return(nil)
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: createdMRSecretForConfigurationSeedChart.Name, Namespace: createdMRSecretForConfigurationSeedChart.Namespace}}).Return(nil)
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedConfigurationChartResourceName, Namespace: createdMRSecretForConfigurationSeedChart.Namespace}}).Return(nil)
			}

			// Delete mock controlplane chart
			ccmChart := mockchartutil.NewMockInterface(ctrl)
			deletedMRForControlPlaneChart := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedChartResourceName, Namespace: namespace},
			}
			deletedMRSecretForControlPlaneChart := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: ControlPlaneSeedChartResourceName, Namespace: namespace},
			}
			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: ControlPlaneSeedChartResourceName}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{}))
			c.EXPECT().Delete(ctx, deletedMRForControlPlaneChart).Return(nil)
			c.EXPECT().Delete(ctx, deletedMRSecretForControlPlaneChart).Return(nil)

			if webhookConfig != nil {
				c.EXPECT().Get(gomock.Any(), resourceKeyShootWebhooks, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{}))
				c.EXPECT().Delete(ctx, deletedMRForShootWebhooks).Return(nil)
				c.EXPECT().Delete(ctx, deletedMRSecretForShootWebhooks).Return(nil)
				c.EXPECT().Get(gomock.Any(), resourceKeyShootWebhooks, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, deletedMRForShootWebhooks.Name))
			}

			// Handle shoot access secrets and legacy secret cleanup
			c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: shootAccessSecretsFunc(namespace)[0].Secret.Name, Namespace: namespace}})

			// Create actuator
			a := &actuator{
				providerName:                   providerName,
				secretConfigsFunc:              getSecretsConfigs,
				shootAccessSecretsFunc:         shootAccessSecretsFunc,
				exposureSecretConfigsFunc:      nil,
				exposureShootAccessSecretsFunc: nil,
				configChart:                    configChart,
				controlPlaneChart:              ccmChart,
				controlPlaneShootChart:         nil,
				controlPlaneShootCRDsChart:     cpShootCRDsChart,
				storageClassesChart:            nil,
				controlPlaneExposureChart:      nil,
				vp:                             vp,
				chartRendererFactory:           crf,
				imageVector:                    imageVector,
				configName:                     configName,
				atomicShootWebhookConfig:       atomicWebhookConfig,
				webhookServerNamespace:         webhookServerNamespace,
				gardenerClientset:              gardenerClientset,
				client:                         c,
				newSecretsManager:              newSecretsManager,
			}

			// Call Delete method and check the result
			Expect(a.Delete(ctx, logger, cp, cluster)).To(Succeed())

			expectSecretsManagedBySecretsManager(fakeClient, "all secrets managed by SecretsManager should get cleaned up")
		},
		Entry("should delete secrets and charts", cloudProviderConfigName, &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}}}, true),
		Entry("should delete secrets and charts (no config)", "", &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}}}, true),
		Entry("should delete secrets and charts (no webhook)", cloudProviderConfigName, nil, true),
		Entry("should delete secrets and charts (no shoot CRDs chart)", cloudProviderConfigName, &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}}}, false),
	)

	DescribeTable("#ReconcileExposure",
		func() {
			// Create mock client
			c := mockclient.NewMockClient(ctrl)

			// Create mock Gardener clientset and chart applier
			gardenerClientset := kubernetesmock.NewMockInterface(ctrl)
			gardenerClientset.EXPECT().Version().Return(seedVersion)
			chartApplier := kubernetesmock.NewMockChartApplier(ctrl)
			gardenerClientset.EXPECT().ChartApplier().Return(chartApplier)

			// Create mock charts
			cpExposureChart := mockchartutil.NewMockInterface(ctrl)
			cpExposureChart.EXPECT().Apply(ctx, chartApplier, namespace, imageVector, seedVersion, shootVersion, controlPlaneExposureChartValues).Return(nil)

			// Create mock values provider
			vp := extensionsmockgenericactuator.NewMockValuesProvider(ctrl)
			vp.EXPECT().GetControlPlaneExposureChartValues(ctx, cpExposure, cluster, gomock.Any(), exposureChecksums).Return(controlPlaneExposureChartValues, nil)

			// Handle shoot access secrets and legacy secret cleanup
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: exposureShootAccessSecretsFunc(namespace)[0].Secret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				Do(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
					obj.SetResourceVersion("0")
				})
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
				Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      exposureShootAccessSecretsFunc(namespace)[0].Secret.Name,
							Namespace: namespace,
							Annotations: map[string]string{
								"serviceaccount.resources.gardener.cloud/name":      exposureShootAccessSecretsFunc(namespace)[0].ServiceAccountName,
								"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
							},
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
								"resources.gardener.cloud/class":   "shoot",
							},
							ResourceVersion: "0",
						},
						Type: corev1.SecretTypeOpaque,
					}))
				})

			// Create actuator
			a := &actuator{
				providerName:                   providerName,
				secretConfigsFunc:              nil,
				shootAccessSecretsFunc:         nil,
				exposureSecretConfigsFunc:      getSecretsConfigsExposure,
				exposureShootAccessSecretsFunc: exposureShootAccessSecretsFunc,
				configChart:                    nil,
				controlPlaneChart:              nil,
				controlPlaneShootChart:         nil,
				controlPlaneShootCRDsChart:     nil,
				storageClassesChart:            nil,
				controlPlaneExposureChart:      cpExposureChart,
				vp:                             vp,
				chartRendererFactory:           nil,
				imageVector:                    imageVector,
				configName:                     "",
				atomicShootWebhookConfig:       nil,
				webhookServerNamespace:         "",
				gardenerClientset:              gardenerClientset,
				client:                         c,
				newSecretsManager:              newSecretsManager,
			}

			// Call Reconcile method and check the result
			requeue, err := a.Reconcile(ctx, logger, cpExposure, cluster)
			Expect(requeue).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			expectSecretsManagedBySecretsManager(fakeClient, "wanted secrets should get created",
				"ca-provider-test-controlplane-exposure-3dcf5fed", "ca-provider-test-controlplane-exposure-bundle-20af429f",
				"lb-readvertiser-aa3c2451",
			)
		},
		Entry("should deploy secrets and apply charts with correct parameters"),
	)

	DescribeTable("#DeleteExposure",
		func() {
			// Create mock clients
			c := mockclient.NewMockClient(ctrl)

			// Create mock Gardener clientset and chart applier
			gardenerClientset := kubernetesmock.NewMockInterface(ctrl)

			// Create mock charts
			cpExposureChart := mockchartutil.NewMockInterface(ctrl)
			cpExposureChart.EXPECT().Delete(ctx, c, namespace).Return(nil)

			// Handle shoot access secrets and legacy secret cleanup
			c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: exposureShootAccessSecretsFunc(namespace)[0].Secret.Name, Namespace: namespace}})

			// Create actuator
			a := &actuator{
				providerName:                   providerName,
				secretConfigsFunc:              nil,
				shootAccessSecretsFunc:         nil,
				exposureSecretConfigsFunc:      getSecretsConfigsExposure,
				exposureShootAccessSecretsFunc: exposureShootAccessSecretsFunc,
				configChart:                    nil,
				controlPlaneChart:              nil,
				controlPlaneShootChart:         nil,
				controlPlaneShootCRDsChart:     nil,
				storageClassesChart:            nil,
				controlPlaneExposureChart:      cpExposureChart,
				vp:                             nil,
				chartRendererFactory:           nil,
				imageVector:                    nil,
				configName:                     "",
				atomicShootWebhookConfig:       nil,
				webhookServerNamespace:         "",
				gardenerClientset:              gardenerClientset,
				client:                         c,
				newSecretsManager:              newSecretsManager,
			}

			// Call Delete method and check the result
			Expect(a.Delete(ctx, logger, cpExposure, cluster)).To(Succeed())

			expectSecretsManagedBySecretsManager(fakeClient, "all secrets managed by SecretsManager should get cleaned up")
		},
		Entry("should delete secrets and charts"),
	)
})

func clientGet(result client.Object) any {
	return func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		case *corev1.ConfigMap:
			*obj.(*corev1.ConfigMap) = *result.(*corev1.ConfigMap)
		}
		return nil
	}
}

func getPurposeExposure() *extensionsv1alpha1.Purpose {
	purpose := new(extensionsv1alpha1.Purpose)
	*purpose = extensionsv1alpha1.Exposure
	return purpose
}

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

func getSecretsConfigsExposure(namespace string) []extensionssecretsmanager.SecretConfigWithOptions {
	return []extensionssecretsmanager.SecretConfigWithOptions{
		{
			Config: &secretsutils.CertificateSecretConfig{
				Name:       caNameControlPlaneExposure,
				CommonName: caNameControlPlaneExposure,
				CertType:   secretsutils.CACert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		},
		{
			Config: &secretsutils.CertificateSecretConfig{
				Name:       "lb-readvertiser",
				CommonName: "lb-readvertiser",
				DNSNames:   kubernetesutils.DNSNamesForService("lb-readvertiser", namespace),
				CertType:   secretsutils.ServerCert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlaneExposure)},
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

func expectSecretsManagedBySecretsManager(c client.Reader, description string, secretNames ...string) {
	secretList := &corev1.SecretList{}
	ExpectWithOffset(1, c.List(context.Background(), secretList, client.MatchingLabels{"managed-by": "secrets-manager"})).To(Succeed())
	ExpectWithOffset(1, secretList.Items).To(consistOfObjects(secretNames...), description)
}

func setupManagedResourceCreation(ctx context.Context, c *mockclient.MockClient, s *corev1.Secret, r *resourcesv1alpha1.ManagedResource) {
	errNotFound := &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
	utilruntime.Must(kubernetesutils.MakeUnique(s))
	c.EXPECT().Get(ctx, client.ObjectKeyFromObject(s), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
	c.EXPECT().Create(ctx, s).Return(nil)
	c.EXPECT().Get(ctx, client.ObjectKeyFromObject(r), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
	r.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: s.Name}}
	utilruntime.Must(references.InjectAnnotations(r))
	c.EXPECT().Create(ctx, r).Return(nil)
}
