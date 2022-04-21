// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genericactuator

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/clock"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	mockgenericactuator "github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator/mock"
	mockextensionscontroller "github.com/gardener/gardener/extensions/pkg/controller/mock"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	extensionswebhookshoot "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockchartrenderer "github.com/gardener/gardener/pkg/chartrenderer/mock"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/chart"
	mockchartutil "github.com/gardener/gardener/pkg/utils/chart/mocks"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

const (
	namespace               = "test"
	cloudProviderConfigName = "cloud-provider-config"
	chartName               = "chartName"
	renderedContent         = "renderedContent"
	providerName            = "provider-test"

	caNameControlPlane         = "ca-" + providerName + "-controlplane"
	caNameControlPlaneExposure = caNameControlPlane + "-exposure"

	seedVersion  = "1.20.0"
	shootVersion = "1.20.0"
)

var (
	vFalse, vTrue = false, true
	pFalse, pTrue = &vFalse, &vTrue

	fakeClock *clock.FakeClock
)

func TestControlPlane(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane Generic Actuator Suite")
}

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVars(
		&secretutils.GenerateRandomString, secretutils.FakeGenerateRandomString,
		&secretutils.GenerateKey, secretutils.FakeGenerateKey,
	))
})

var _ = Describe("Actuator", func() {
	var (
		ctrl              *gomock.Controller
		fakeClient        client.Client
		newSecretsManager newSecretsManagerFunc

		ctx               = context.TODO()
		providerType      = "test"
		webhookServerPort = 443

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

		cpSecretKey    = client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}
		cpConfigMapKey = client.ObjectKey{Namespace: namespace, Name: cloudProviderConfigName}
		cpSecret       = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.SecretNameCloudProvider, Namespace: namespace},
			Data:       map[string][]byte{"foo": []byte("bar")},
		}
		cpConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cloudProviderConfigName, Namespace: namespace},
			Data:       map[string]string{"abc": "xyz"},
		}

		resourceKeyCPShootChart        = client.ObjectKey{Namespace: namespace, Name: ControlPlaneShootChartResourceName}
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

		resourceKeyCPShootCRDsChart        = client.ObjectKey{Namespace: namespace, Name: ControlPlaneShootCRDsChartResourceName}
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

		resourceKeyStorageClassesChart        = client.ObjectKey{Namespace: namespace, Name: StorageClassesChartResourceName}
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

		resourceKeyShootWebhooksNetworkPolicy = client.ObjectKey{Namespace: namespace, Name: "gardener-extension-" + providerName}
		createdNetworkPolicyForShootWebhooks  = constructNetworkPolicy(providerName, namespace, webhookServerPort)
		deletedNetworkPolicyForShootWebhooks  = &networkingv1.NetworkPolicy{
			ObjectMeta: extensionswebhookshoot.GetNetworkPolicyMeta(namespace, providerName).ObjectMeta,
		}

		resourceKeyShootWebhooks  = client.ObjectKey{Namespace: namespace, Name: ShootWebhooksResourceName}
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

		imageVector = imagevector.ImageVector([]*imagevector.ImageSource{})

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			cloudProviderConfigName:                  "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
			caNameControlPlane:                       "bfcf386778c8d3313168d622fc1e2d28d9b6265759e3f505e3cffa5848206dd1",
			"cloud-controller-manager":               "48b5661ff7d535ac5cb2c3e4efc47918cc702ff0ebf84772150310ebc3c943ac",
		}
		checksumsNoConfig = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			caNameControlPlane:                       "bfcf386778c8d3313168d622fc1e2d28d9b6265759e3f505e3cffa5848206dd1",
			"cloud-controller-manager":               "48b5661ff7d535ac5cb2c3e4efc47918cc702ff0ebf84772150310ebc3c943ac",
		}
		exposureChecksums = map[string]string{
			caNameControlPlaneExposure: "3162cd2e6ef1a654fb98ca96af809d7c3341de0e2fa95b64ce8b53accaf45e57",
			"lb-readvertiser":          "81ef59b177361b751734e9d7540331a9110428d5c58f2c5334b45ac6c4ceb39f",
		}

		configChartValues = map[string]interface{}{
			"cloudProviderConfig": `[Global]`,
		}

		controlPlaneChartValues = map[string]interface{}{
			"clusterName": namespace,
		}

		controlPlaneShootChartValues = map[string]interface{}{
			"foo": "bar",
		}

		controlPlaneShootCRDsChartValues = map[string]interface{}{
			"foo": "bar",
		}

		storageClassesChartValues = map[string]interface{}{
			"foo": "bar",
		}

		controlPlaneExposureChartValues = map[string]interface{}{
			"replicas": 1,
		}

		shootAccessSecretsFunc         func(string) []*gutil.ShootAccessSecret
		exposureShootAccessSecretsFunc func(string) []*gutil.ShootAccessSecret

		errNotFound = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
		logger      = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		fakeClient = fakeclient.NewClientBuilder().Build()
		newSecretsManager = func(ctx context.Context, logger logr.Logger, clock clock.Clock, c client.Client, cluster *extensionscontroller.Cluster, identity string, secretConfigs []extensionssecretsmanager.SecretConfigWithOptions) (secretsmanager.Interface, error) {
			// use fake clock and client, pass on the rest
			return extensionssecretsmanager.SecretsManagerForCluster(ctx, logger, fakeClock, fakeClient, cluster, identity, secretConfigs)
		}

		deterministicReader := strings.NewReader(strings.Repeat("-", 10000))
		DeferCleanup(test.WithVar(&rand.Reader, deterministicReader))
		fakeClock = clock.NewFakeClock(time.Unix(1649848746, 0))

		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "control-plane", Namespace: namespace},
			Spec:       extensionsv1alpha1.ControlPlaneSpec{},
		}

		shootAccessSecretsFunc = func(namespace string) []*gutil.ShootAccessSecret {
			return []*gutil.ShootAccessSecret{gutil.NewShootAccessSecret("new-cp", namespace)}
		}
		exposureShootAccessSecretsFunc = func(namespace string) []*gutil.ShootAccessSecret {
			return []*gutil.ShootAccessSecret{gutil.NewShootAccessSecret("new-cp-exposure", namespace)}
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	DescribeTable("#Reconcile",
		func(configName string, checksums map[string]string, webhooks []admissionregistrationv1.MutatingWebhook, withShootCRDsChart bool) {
			// Create mock client
			c := mockclient.NewMockClient(ctrl)

			if len(webhooks) > 0 {
				c.EXPECT().Get(ctx, resourceKeyShootWebhooksNetworkPolicy, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(errNotFound)
				c.EXPECT().Create(ctx, createdNetworkPolicyForShootWebhooks).Return(nil)

				data, _ := marshalWebhooks(webhooks, providerName)
				createdMRSecretForShootWebhooks := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: namespace},
					Data:       map[string][]byte{"mutatingwebhookconfiguration.yaml": data},
					Type:       corev1.SecretTypeOpaque,
				}
				c.EXPECT().Get(ctx, resourceKeyShootWebhooks, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
				c.EXPECT().Create(ctx, createdMRSecretForShootWebhooks).Return(nil)
				c.EXPECT().Get(ctx, resourceKeyShootWebhooks, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
				c.EXPECT().Create(ctx, createdMRForShootWebhooks).Return(nil)
			}

			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			if configName != "" {
				c.EXPECT().Get(ctx, cpConfigMapKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cpConfigMap))
			}

			c.EXPECT().Get(ctx, resourceKeyCPShootChart, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
			c.EXPECT().Create(ctx, createdMRSecretForCPShootChart).Return(nil)
			c.EXPECT().Get(ctx, resourceKeyCPShootChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
			c.EXPECT().Create(ctx, createdMRForCPShootChart).Return(nil)

			if withShootCRDsChart {
				c.EXPECT().Get(ctx, resourceKeyCPShootCRDsChart, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
				c.EXPECT().Create(ctx, createdMRSecretForCPShootCRDsChart).Return(nil)
				c.EXPECT().Get(ctx, resourceKeyCPShootCRDsChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
				c.EXPECT().Create(ctx, createdMRForCPShootCRDsChart).Return(nil)
			}

			c.EXPECT().Get(ctx, resourceKeyStorageClassesChart, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
			c.EXPECT().Create(ctx, createdMRSecretForStorageClassesChart).Return(nil)
			c.EXPECT().Get(ctx, resourceKeyStorageClassesChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
			c.EXPECT().Create(ctx, createdMRForStorageClassesChart).Return(nil)

			// Create mock Gardener clientset and chart applier
			gardenerClientset := mockkubernetes.NewMockInterface(ctrl)
			gardenerClientset.EXPECT().Version().Return(seedVersion)
			chartApplier := mockkubernetes.NewMockChartApplier(ctrl)

			// Create mock chart renderer and factory
			chartRenderer := mockchartrenderer.NewMockInterface(ctrl)
			crf := mockextensionscontroller.NewMockChartRendererFactory(ctrl)
			crf.EXPECT().NewChartRendererForShoot(shootVersion).Return(chartRenderer, nil)

			// Create mock charts
			var configChart chart.Interface
			if configName != "" {
				configChartMock := mockchartutil.NewMockInterface(ctrl)
				configChartMock.EXPECT().Apply(ctx, chartApplier, namespace, nil, "", "", configChartValues).Return(nil)
				configChart = configChartMock
			}
			ccmChart := mockchartutil.NewMockInterface(ctrl)
			ccmChart.EXPECT().Apply(ctx, chartApplier, namespace, imageVector, seedVersion, shootVersion, controlPlaneChartValues).Return(nil)
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
			vp := mockgenericactuator.NewMockValuesProvider(ctrl)
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
			c.EXPECT().Get(ctx, kutil.Key(namespace, shootAccessSecretsFunc(namespace)[0].Secret.Name), gomock.AssignableToTypeOf(&corev1.Secret{}))
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
				Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      shootAccessSecretsFunc(namespace)[0].Secret.Name,
							Namespace: namespace,
							Annotations: map[string]string{
								"serviceaccount.resources.gardener.cloud/name":      shootAccessSecretsFunc(namespace)[0].ServiceAccountName,
								"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
							},
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
							},
						},
						Type: corev1.SecretTypeOpaque,
					}))
				})

			// Create actuator
			a := NewActuator(providerName, getSecretsConfigs, shootAccessSecretsFunc, nil, nil, configChart, ccmChart, ccmShootChart, cpShootCRDsChart, storageClassesChart, nil, vp, crf, imageVector, configName, webhooks, webhookServerPort, logger)
			err := a.(inject.Client).InjectClient(c)
			Expect(err).NotTo(HaveOccurred())
			a.(*actuator).gardenerClientset = gardenerClientset
			a.(*actuator).chartApplier = chartApplier
			a.(*actuator).newSecretsManager = newSecretsManager

			// Call Reconcile method and check the result
			requeue, err := a.Reconcile(ctx, cp, cluster)
			Expect(requeue).To(Equal(false))
			Expect(err).NotTo(HaveOccurred())

			expectSecretsManagedBySecretsManager(fakeClient, "wanted secrets should get created",
				"ca-provider-test-controlplane-b01ab5b3", "ca-provider-test-controlplane-bundle-264279f5",
				"cloud-controller-manager-87d232df",
			)
		},
		Entry("should deploy secrets and apply charts with correct parameters", cloudProviderConfigName, checksums, []admissionregistrationv1.MutatingWebhook{{}}, true),
		Entry("should deploy secrets and apply charts with correct parameters (no config)", "", checksumsNoConfig, []admissionregistrationv1.MutatingWebhook{{}}, true),
		Entry("should deploy secrets and apply charts with correct parameters (no webhook)", cloudProviderConfigName, checksums, nil, true),
		Entry("should deploy secrets and apply charts with correct parameters (no shoot CRDs chart)", cloudProviderConfigName, checksums, []admissionregistrationv1.MutatingWebhook{{}}, false),
	)

	DescribeTable("#Delete",
		func(configName string, webhooks []admissionregistrationv1.MutatingWebhook, withShootCRDsChart bool) {
			// Create mock clients
			client := mockclient.NewMockClient(ctrl)

			client.EXPECT().Delete(ctx, deletedMRForStorageClassesChart).Return(nil)
			client.EXPECT().Delete(ctx, deletedMRSecretForStorageClassesChart).Return(nil)
			var cpShootCRDsChart chart.Interface
			if withShootCRDsChart {
				cpShootCRDsChartMock := mockchartutil.NewMockInterface(ctrl)
				cpShootCRDsChart = cpShootCRDsChartMock
				client.EXPECT().Delete(ctx, deletedMRForCPShootCRDsChart).Return(nil)
				client.EXPECT().Delete(ctx, deletedMRSecretForCPShootCRDsChart).Return(nil)
				client.EXPECT().Get(gomock.Any(), resourceKeyCPShootCRDsChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, deletedMRForCPShootCRDsChart.Name))
			}

			client.EXPECT().Delete(ctx, deletedMRForCPShootChart).Return(nil)
			client.EXPECT().Delete(ctx, deletedMRSecretForCPShootChart).Return(nil)

			client.EXPECT().Get(gomock.Any(), resourceKeyStorageClassesChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, deletedMRForStorageClassesChart.Name))
			client.EXPECT().Get(gomock.Any(), resourceKeyCPShootChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, deletedMRForCPShootChart.Name))

			// Create mock charts
			var configChart chart.Interface
			if configName != "" {
				configChartMock := mockchartutil.NewMockInterface(ctrl)
				configChartMock.EXPECT().Delete(ctx, client, namespace).Return(nil)
				configChart = configChartMock
			}
			ccmChart := mockchartutil.NewMockInterface(ctrl)
			ccmChart.EXPECT().Delete(ctx, client, namespace).Return(nil)

			if len(webhooks) > 0 {
				client.EXPECT().Delete(ctx, deletedNetworkPolicyForShootWebhooks).Return(nil)
				client.EXPECT().Delete(ctx, deletedMRForShootWebhooks).Return(nil)
				client.EXPECT().Delete(ctx, deletedMRSecretForShootWebhooks).Return(nil)
				client.EXPECT().Get(gomock.Any(), resourceKeyShootWebhooks, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, deletedMRForShootWebhooks.Name))
			}

			// Handle shoot access secrets and legacy secret cleanup
			client.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: shootAccessSecretsFunc(namespace)[0].Secret.Name, Namespace: namespace}})

			// Create actuator
			a := NewActuator(providerName, getSecretsConfigs, shootAccessSecretsFunc, nil, nil, configChart, ccmChart, nil, cpShootCRDsChart, nil, nil, nil, nil, nil, configName, webhooks, webhookServerPort, logger)
			Expect(a.(inject.Client).InjectClient(client)).To(Succeed())
			a.(*actuator).newSecretsManager = newSecretsManager

			// Call Delete method and check the result
			Expect(a.Delete(ctx, cp, cluster)).To(Succeed())

			expectSecretsManagedBySecretsManager(fakeClient, "all secrets managed by SecretsManager should get cleaned up")
		},
		Entry("should delete secrets and charts", cloudProviderConfigName, []admissionregistrationv1.MutatingWebhook{{}}, true),
		Entry("should delete secrets and charts (no config)", "", []admissionregistrationv1.MutatingWebhook{{}}, true),
		Entry("should delete secrets and charts (no webhook)", cloudProviderConfigName, nil, true),
		Entry("should delete secrets and charts (no shoot CRDs chart)", cloudProviderConfigName, []admissionregistrationv1.MutatingWebhook{{}}, false),
	)

	DescribeTable("#ReconcileExposure",
		func() {
			// Create mock client
			c := mockclient.NewMockClient(ctrl)

			// Create mock Gardener clientset and chart applier
			gardenerClientset := mockkubernetes.NewMockInterface(ctrl)
			gardenerClientset.EXPECT().Version().Return(seedVersion)
			chartApplier := mockkubernetes.NewMockChartApplier(ctrl)

			// Create mock charts
			cpExposureChart := mockchartutil.NewMockInterface(ctrl)
			cpExposureChart.EXPECT().Apply(ctx, chartApplier, namespace, imageVector, seedVersion, shootVersion, controlPlaneExposureChartValues).Return(nil)

			// Create mock values provider
			vp := mockgenericactuator.NewMockValuesProvider(ctrl)
			vp.EXPECT().GetControlPlaneExposureChartValues(ctx, cpExposure, cluster, gomock.Any(), exposureChecksums).Return(controlPlaneExposureChartValues, nil)

			// Handle shoot access secrets and legacy secret cleanup
			c.EXPECT().Get(ctx, kutil.Key(namespace, exposureShootAccessSecretsFunc(namespace)[0].Secret.Name), gomock.AssignableToTypeOf(&corev1.Secret{}))
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
				Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
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
							},
						},
						Type: corev1.SecretTypeOpaque,
					}))
				})

			// Create actuator
			a := NewActuator(providerName, nil, nil, getSecretsConfigsExposure, exposureShootAccessSecretsFunc, nil, nil, nil, nil, nil, cpExposureChart, vp, nil, imageVector, "", nil, 0, logger)
			Expect(a.(inject.Client).InjectClient(c)).To(Succeed())
			a.(*actuator).gardenerClientset = gardenerClientset
			a.(*actuator).chartApplier = chartApplier
			a.(*actuator).newSecretsManager = newSecretsManager

			// Call Reconcile method and check the result
			requeue, err := a.Reconcile(ctx, cpExposure, cluster)
			Expect(requeue).To(Equal(false))
			Expect(err).NotTo(HaveOccurred())

			expectSecretsManagedBySecretsManager(fakeClient, "wanted secrets should get created",
				"ca-provider-test-controlplane-exposure-708d12fb", "ca-provider-test-controlplane-exposure-bundle-fec10245",
				"lb-readvertiser-29ec35bb",
			)
		},
		Entry("should deploy secrets and apply charts with correct parameters"),
	)

	DescribeTable("#DeleteExposure",
		func() {
			// Create mock clients
			client := mockclient.NewMockClient(ctrl)

			// Create mock charts
			cpExposureChart := mockchartutil.NewMockInterface(ctrl)
			cpExposureChart.EXPECT().Delete(ctx, client, namespace).Return(nil)

			// Handle shoot access secrets and legacy secret cleanup
			client.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: exposureShootAccessSecretsFunc(namespace)[0].Secret.Name, Namespace: namespace}})

			// Create actuator
			a := NewActuator(providerName, nil, nil, getSecretsConfigsExposure, exposureShootAccessSecretsFunc, nil, nil, nil, nil, nil, cpExposureChart, nil, nil, nil, "", nil, 0, logger)
			Expect(a.(inject.Client).InjectClient(client)).To(Succeed())
			a.(*actuator).newSecretsManager = newSecretsManager

			// Call Delete method and check the result
			Expect(a.Delete(ctx, cpExposure, cluster)).To(Succeed())

			expectSecretsManagedBySecretsManager(fakeClient, "all secrets managed by SecretsManager should get cleaned up")
		},
		Entry("should delete secrets and charts"),
	)

	Describe("#ReconcileShootWebhooksForAllNamespaces", func() {
		var (
			labelSelector = client.MatchingLabels{
				v1beta1constants.GardenRole:         v1beta1constants.GardenRoleShoot,
				v1beta1constants.LabelShootProvider: providerType,
			}
			port              = 1234
			networkPolicyName = "gardener-extension-" + providerName
			shootWebhooks     = []admissionregistrationv1.MutatingWebhook{{}}
		)

		It("should behave correctly", func() {
			c := mockclient.NewMockClient(ctrl)

			v1beta1WebhookData, err := marshalWebhooks(shootWebhooks, providerName)
			Expect(err).NotTo(HaveOccurred())

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{}), labelSelector).DoAndReturn(func(_ context.Context, list *corev1.NamespaceList, _ ...client.ListOption) error {
				*list = corev1.NamespaceList{Items: []corev1.Namespace{
					{ObjectMeta: metav1.ObjectMeta{Name: "namespace1"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "namespace2"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "namespace3"}},
				}}
				return nil
			})
			c.EXPECT().Get(ctx, kutil.Key("namespace1", networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}))
			c.EXPECT().Get(ctx, kutil.Key("namespace2", networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(errNotFound)
			c.EXPECT().Get(ctx, kutil.Key("namespace3", networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}))

			createdMRSecretForShootWebhooksNS1 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: "namespace1"},
				Data:       map[string][]byte{"mutatingwebhookconfiguration.yaml": v1beta1WebhookData},
				Type:       corev1.SecretTypeOpaque,
			}
			createdMRForShootWebhooksNS1 := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: "namespace1"},
				Spec:       resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: ShootWebhooksResourceName}}},
			}
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: "namespace1", Name: networkPolicyName}, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(errNotFound)
			c.EXPECT().Create(ctx, constructNetworkPolicy(providerName, "namespace1", port))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: "namespace1", Name: ShootWebhooksResourceName}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
			c.EXPECT().Create(ctx, createdMRSecretForShootWebhooksNS1)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: "namespace1", Name: ShootWebhooksResourceName}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
			c.EXPECT().Create(ctx, createdMRForShootWebhooksNS1)

			c.EXPECT().Get(ctx, kutil.Key("namespace1"), gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, testCluster *extensionsv1alpha1.Cluster) error {
					*testCluster = computeClusterWithShoot("namespace1", gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: shootVersion,
						},
					})
					return nil
				},
			)

			v1WebhookData, err := marshalWebhooks(shootWebhooks, providerName)
			Expect(err).NotTo(HaveOccurred())

			createdMRSecretForShootWebhooksNS3 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: "namespace3"},
				Data:       map[string][]byte{"mutatingwebhookconfiguration.yaml": v1WebhookData},
				Type:       corev1.SecretTypeOpaque,
			}
			createdMRForShootWebhooksNS3 := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: "namespace3"},
				Spec:       resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: ShootWebhooksResourceName}}},
			}
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: "namespace3", Name: networkPolicyName}, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}))
			c.EXPECT().Patch(ctx, constructNetworkPolicy(providerName, "namespace3", port), gomock.Any())
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: "namespace3", Name: ShootWebhooksResourceName}, gomock.AssignableToTypeOf(&corev1.Secret{}))
			c.EXPECT().Update(ctx, createdMRSecretForShootWebhooksNS3)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: "namespace3", Name: ShootWebhooksResourceName}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{}))
			c.EXPECT().Update(ctx, createdMRForShootWebhooksNS3)

			c.EXPECT().Get(ctx, kutil.Key("namespace3"), gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, testCluster *extensionsv1alpha1.Cluster) error {
					*testCluster = computeClusterWithShoot("namespace3", gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: shootVersion,
						},
					})
					return nil
				},
			)

			Expect(ReconcileShootWebhooksForAllNamespaces(ctx, c, providerName, providerType, port, shootWebhooks)).To(Succeed())
		})
	})
})

func clientGet(result client.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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

func constructNetworkPolicy(providerName, namespace string, webhookPort int) *networkingv1.NetworkPolicy {
	var (
		protocol = corev1.ProtocolTCP
		port     = intstr.FromInt(webhookPort)
	)

	return &networkingv1.NetworkPolicy{
		ObjectMeta: extensionswebhookshoot.GetNetworkPolicyMeta(namespace, providerName).ObjectMeta,
		Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &port,
							Protocol: &protocol,
						},
					},
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.LabelControllerRegistrationName: providerName,
									v1beta1constants.GardenRole:                      v1beta1constants.GardenRoleExtension,
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": "gardener-extension-" + providerName,
								},
							},
						},
					},
				},
			},
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
					v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
				},
			},
		},
	}
}

func computeClusterWithShoot(name string, shootSpec gardencorev1beta1.ShootSpec) extensionsv1alpha1.Cluster {
	shoot := &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Shoot",
		},
		Spec: shootSpec,
	}

	shootJSON, err := json.Marshal(shoot)
	Expect(err).To(Succeed())

	return extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			Shoot: runtime.RawExtension{Raw: shootJSON},
		},
	}
}

func getSecretsConfigs(namespace string) []extensionssecretsmanager.SecretConfigWithOptions {
	return []extensionssecretsmanager.SecretConfigWithOptions{
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:       caNameControlPlane,
				CommonName: caNameControlPlane,
				CertType:   secretutils.CACert,
				Clock:      fakeClock,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		},
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:       "cloud-controller-manager",
				CommonName: "cloud-controller-manager",
				DNSNames:   kutil.DNSNamesForService("cloud-controller-manager", namespace),
				CertType:   secretutils.ServerCert,
				Clock:      fakeClock,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane)},
		},
	}
}

func getSecretsConfigsExposure(namespace string) []extensionssecretsmanager.SecretConfigWithOptions {
	return []extensionssecretsmanager.SecretConfigWithOptions{
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:       caNameControlPlaneExposure,
				CommonName: caNameControlPlaneExposure,
				CertType:   secretutils.CACert,
				Clock:      fakeClock,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		},
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:       "lb-readvertiser",
				CommonName: "lb-readvertiser",
				DNSNames:   kutil.DNSNamesForService("lb-readvertiser", namespace),
				CertType:   secretutils.ServerCert,
				Clock:      fakeClock,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlaneExposure)},
		},
	}
}

var (
	objectIdentifier = Identifier(func(obj interface{}) string {
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
