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
	"encoding/json"
	"testing"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	mockgenericactuator "github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator/mock"
	mockextensionscontroller "github.com/gardener/gardener/extensions/pkg/controller/mock"
	extensionswebhookshoot "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockchartrenderer "github.com/gardener/gardener/pkg/chartrenderer/mock"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/chart"
	mockchartutil "github.com/gardener/gardener/pkg/utils/chart/mocks"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	mocksecretsutil "github.com/gardener/gardener/pkg/utils/secrets/mock"

	"github.com/Masterminds/semver"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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
)

const (
	namespace               = "test"
	cloudProviderConfigName = "cloud-provider-config"
	chartName               = "chartName"
	renderedContent         = "renderedContent"

	seedVersion     = "1.15.0"
	shootVersion115 = "1.15.0"
	shootVersion116 = "1.16.0"
)

var (
	vFalse, vTrue = false, true
	pFalse, pTrue = &vFalse, &vTrue
)

func TestControlplane(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controlplane Generic Actuator Suite")
}

var _ = Describe("Actuator", func() {
	var (
		ctrl                                   *gomock.Controller
		shootK8sVersion115, shootK8sVersion116 *semver.Version

		ctx               = context.TODO()
		providerName      = "provider-test"
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
						Version: shootVersion116,
					},
				},
			},
		}

		deployedSecrets = map[string]*corev1.Secret{
			"cloud-controller-manager": {
				ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager", Namespace: namespace},
				Data:       map[string][]byte{"a": []byte("b")},
			},
		}
		deployedExposureSecrets = map[string]*corev1.Secret{
			"lb-readvertiser": {
				ObjectMeta: metav1.ObjectMeta{Name: "lb-readvertiser", Namespace: namespace},
				Data:       map[string][]byte{"a": []byte("b")},
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
			"cloud-controller-manager":               "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
		}
		checksumsNoConfig = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			"cloud-controller-manager":               "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
		}
		exposureChecksums = map[string]string{
			"lb-readvertiser": "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
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

		errNotFound = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
		logger      = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		var err error
		shootK8sVersion115, err = semver.NewVersion(shootVersion115)
		Expect(err).To(Not(HaveOccurred()))
		shootK8sVersion116, err = semver.NewVersion(shootVersion116)
		Expect(err).To(Not(HaveOccurred()))

		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "control-plane", Namespace: namespace},
			Spec:       extensionsv1alpha1.ControlPlaneSpec{},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	DescribeTable("#Reconcile",
		func(configName string, checksums map[string]string, webhooks []admissionregistrationv1.MutatingWebhook, withShootCRDsChart bool) {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)

			if len(webhooks) > 0 {
				client.EXPECT().Get(ctx, resourceKeyShootWebhooksNetworkPolicy, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(errNotFound)
				client.EXPECT().Create(ctx, createdNetworkPolicyForShootWebhooks).Return(nil)

				data, _ := marshalWebhooks(webhooks, providerName, shootK8sVersion116)
				createdMRSecretForShootWebhooks := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: ShootWebhooksResourceName, Namespace: namespace},
					Data:       map[string][]byte{"mutatingwebhookconfiguration.yaml": data},
					Type:       corev1.SecretTypeOpaque,
				}
				client.EXPECT().Get(ctx, resourceKeyShootWebhooks, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
				client.EXPECT().Create(ctx, createdMRSecretForShootWebhooks).Return(nil)
				client.EXPECT().Get(ctx, resourceKeyShootWebhooks, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
				client.EXPECT().Create(ctx, createdMRForShootWebhooks).Return(nil)
			}

			client.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			if configName != "" {
				client.EXPECT().Get(ctx, cpConfigMapKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cpConfigMap))
			}

			client.EXPECT().Get(ctx, resourceKeyCPShootChart, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
			client.EXPECT().Create(ctx, createdMRSecretForCPShootChart).Return(nil)
			client.EXPECT().Get(ctx, resourceKeyCPShootChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
			client.EXPECT().Create(ctx, createdMRForCPShootChart).Return(nil)

			if withShootCRDsChart {
				client.EXPECT().Get(ctx, resourceKeyCPShootCRDsChart, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
				client.EXPECT().Create(ctx, createdMRSecretForCPShootCRDsChart).Return(nil)
				client.EXPECT().Get(ctx, resourceKeyCPShootCRDsChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
				client.EXPECT().Create(ctx, createdMRForCPShootCRDsChart).Return(nil)
			}

			client.EXPECT().Get(ctx, resourceKeyStorageClassesChart, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
			client.EXPECT().Create(ctx, createdMRSecretForStorageClassesChart).Return(nil)
			client.EXPECT().Get(ctx, resourceKeyStorageClassesChart, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(errNotFound)
			client.EXPECT().Create(ctx, createdMRForStorageClassesChart).Return(nil)

			// Create mock Gardener clientset and chart applier
			gardenerClientset := mockkubernetes.NewMockInterface(ctrl)
			gardenerClientset.EXPECT().Version().Return(seedVersion)
			chartApplier := mockkubernetes.NewMockChartApplier(ctrl)

			// Create mock chart renderer and factory
			chartRenderer := mockchartrenderer.NewMockInterface(ctrl)
			crf := mockextensionscontroller.NewMockChartRendererFactory(ctrl)
			crf.EXPECT().NewChartRendererForShoot(shootVersion116).Return(chartRenderer, nil)

			// Create mock secrets and charts
			secrets := mocksecretsutil.NewMockInterface(ctrl)
			secrets.EXPECT().Deploy(ctx, gomock.Any(), gardenerClientset, namespace).Return(deployedSecrets, nil)
			var configChart chart.Interface
			if configName != "" {
				configChartMock := mockchartutil.NewMockInterface(ctrl)
				configChartMock.EXPECT().Apply(ctx, chartApplier, namespace, nil, "", "", configChartValues).Return(nil)
				configChart = configChartMock
			}
			ccmChart := mockchartutil.NewMockInterface(ctrl)
			ccmChart.EXPECT().Apply(ctx, chartApplier, namespace, imageVector, seedVersion, shootVersion116, controlPlaneChartValues).Return(nil)
			ccmShootChart := mockchartutil.NewMockInterface(ctrl)
			ccmShootChart.EXPECT().Render(chartRenderer, metav1.NamespaceSystem, imageVector, shootVersion116, shootVersion116, controlPlaneShootChartValues).Return(chartName, []byte(renderedContent), nil)
			var cpShootCRDsChart chart.Interface
			if withShootCRDsChart {
				cpShootCRDsChartMock := mockchartutil.NewMockInterface(ctrl)
				cpShootCRDsChartMock.EXPECT().Render(chartRenderer, metav1.NamespaceSystem, imageVector, shootVersion116, shootVersion116, controlPlaneShootCRDsChartValues).Return(chartName, []byte(renderedContent), nil)
				cpShootCRDsChart = cpShootCRDsChartMock
			}
			storageClassesChart := mockchartutil.NewMockInterface(ctrl)
			storageClassesChart.EXPECT().Render(chartRenderer, metav1.NamespaceSystem, imageVector, shootVersion116, shootVersion116, storageClassesChartValues).Return(chartName, []byte(renderedContent), nil)

			// Create mock values provider
			vp := mockgenericactuator.NewMockValuesProvider(ctrl)
			if configName != "" {
				vp.EXPECT().GetConfigChartValues(ctx, cp, cluster).Return(configChartValues, nil)
			}
			vp.EXPECT().GetControlPlaneChartValues(ctx, cp, cluster, checksums, false).Return(controlPlaneChartValues, nil)
			vp.EXPECT().GetControlPlaneShootChartValues(ctx, cp, cluster, checksums).Return(controlPlaneShootChartValues, nil)
			if withShootCRDsChart {
				vp.EXPECT().GetControlPlaneShootCRDsChartValues(ctx, cp, cluster).Return(controlPlaneShootCRDsChartValues, nil)
			}
			vp.EXPECT().GetStorageClassesChartValues(ctx, cp, cluster).Return(storageClassesChartValues, nil)

			// Create actuator
			a := NewActuator(providerName, secrets, nil, configChart, ccmChart, ccmShootChart, cpShootCRDsChart, storageClassesChart, nil, vp, crf, imageVector, configName, webhooks, webhookServerPort, logger)
			err := a.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())
			a.(*actuator).gardenerClientset = gardenerClientset
			a.(*actuator).chartApplier = chartApplier

			// Call Reconcile method and check the result
			requeue, err := a.Reconcile(ctx, cp, cluster)
			Expect(requeue).To(Equal(false))
			Expect(err).NotTo(HaveOccurred())
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

			// Create mock secrets and charts
			secrets := mocksecretsutil.NewMockInterface(ctrl)
			secrets.EXPECT().Delete(ctx, gomock.Any(), namespace).Return(nil)
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

			// Create actuator
			a := NewActuator(providerName, secrets, nil, configChart, ccmChart, nil, cpShootCRDsChart, nil, nil, nil, nil, nil, configName, webhooks, webhookServerPort, logger)
			err := a.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call Delete method and check the result
			err = a.Delete(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("should delete secrets and charts", cloudProviderConfigName, []admissionregistrationv1.MutatingWebhook{{}}, true),
		Entry("should delete secrets and charts (no config)", "", []admissionregistrationv1.MutatingWebhook{{}}, true),
		Entry("should delete secrets and charts (no webhook)", cloudProviderConfigName, nil, true),
		Entry("should delete secrets and charts (no shoot CRDs chart)", cloudProviderConfigName, []admissionregistrationv1.MutatingWebhook{{}}, false),
	)

	DescribeTable("#ReconcileExposure",
		func() {
			// Create mock Gardener clientset and chart applier
			gardenerClientset := mockkubernetes.NewMockInterface(ctrl)
			gardenerClientset.EXPECT().Version().Return(seedVersion)
			chartApplier := mockkubernetes.NewMockChartApplier(ctrl)

			// Create mock secrets and charts
			exposureSecrets := mocksecretsutil.NewMockInterface(ctrl)
			exposureSecrets.EXPECT().Deploy(ctx, gomock.Any(), gardenerClientset, namespace).Return(deployedExposureSecrets, nil)
			cpExposureChart := mockchartutil.NewMockInterface(ctrl)
			cpExposureChart.EXPECT().Apply(ctx, chartApplier, namespace, imageVector, seedVersion, shootVersion116, controlPlaneExposureChartValues).Return(nil)

			// Create mock values provider
			vp := mockgenericactuator.NewMockValuesProvider(ctrl)
			vp.EXPECT().GetControlPlaneExposureChartValues(ctx, cpExposure, cluster, exposureChecksums).Return(controlPlaneExposureChartValues, nil)

			// Create actuator
			a := NewActuator(providerName, nil, exposureSecrets, nil, nil, nil, nil, nil, cpExposureChart, vp, nil, imageVector, "", nil, 0, logger)
			a.(*actuator).gardenerClientset = gardenerClientset
			a.(*actuator).chartApplier = chartApplier

			// Call Reconcile method and check the result
			requeue, err := a.Reconcile(ctx, cpExposure, cluster)
			Expect(requeue).To(Equal(false))
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("should deploy secrets and apply charts with correct parameters"),
	)

	DescribeTable("#DeleteExposure",
		func() {
			// Create mock clients
			client := mockclient.NewMockClient(ctrl)

			// Create mock secrets and charts
			exposureSecrets := mocksecretsutil.NewMockInterface(ctrl)
			exposureSecrets.EXPECT().Delete(ctx, gomock.Any(), namespace).Return(nil)

			cpExposureChart := mockchartutil.NewMockInterface(ctrl)
			cpExposureChart.EXPECT().Delete(ctx, client, namespace).Return(nil)

			// Create actuator
			a := NewActuator(providerName, nil, exposureSecrets, nil, nil, nil, nil, nil, cpExposureChart, nil, nil, nil, "", nil, 0, logger)
			err := a.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call Delete method and check the result
			err = a.Delete(ctx, cpExposure, cluster)
			Expect(err).NotTo(HaveOccurred())
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

			v1beta1WebhookData, err := marshalWebhooks(shootWebhooks, providerName, shootK8sVersion115)
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
							Version: shootVersion115,
						},
					})
					return nil
				},
			)

			v1WebhookData, err := marshalWebhooks(shootWebhooks, providerName, shootK8sVersion116)
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
							Version: shootVersion116,
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
