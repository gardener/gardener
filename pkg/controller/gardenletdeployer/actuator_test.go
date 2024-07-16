// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenletdeployer

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	mockclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/mock"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockgardenletdepoyer "github.com/gardener/gardener/pkg/controller/gardenletdeployer/mock"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockrecord "github.com/gardener/gardener/third_party/mock/client-go/tools/record"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

const (
	name                   = "test"
	namespace              = "garden"
	seedName               = "test-seed"
	secretBindingName      = "test-secret-binding"
	credentialsBindingName = "test-credentials-binding"
	secretName             = "test-secret"
	backupSecretName       = "test-backup-secret"
)

var _ = Describe("Interface", func() {
	var (
		ctrl *gomock.Controller

		gardenClient      *mockclient.MockClient
		gardenAPIReader   *mockclient.MockReader
		seedClient        *mockclient.MockClient
		shootClientSet    *kubernetesmock.MockInterface
		shootClientMap    *mockclientmap.MockClientMap
		vh                *mockgardenletdepoyer.MockValuesHelper
		shootClient       *mockclient.MockClient
		shootChartApplier *kubernetesmock.MockChartApplier
		recorder          *mockrecord.MockEventRecorder

		log      logr.Logger
		actuator Interface

		ctx context.Context

		managedSeed        *seedmanagementv1alpha1.ManagedSeed
		shoot              *gardencorev1beta1.Shoot
		secretBinding      *gardencorev1beta1.SecretBinding
		credentialsBinding *securityv1alpha1.CredentialsBinding
		secret             *corev1.Secret

		seedTemplate *gardencorev1beta1.SeedTemplate
		gardenlet    *seedmanagementv1alpha1.GardenletConfig

		gardenNamespace     *corev1.Namespace
		backupSecret        *corev1.Secret
		seed                *gardencorev1beta1.Seed
		gardenletDeployment *appsv1.Deployment

		mergedDeployment      *seedmanagementv1alpha1.GardenletDeployment
		mergedGardenletConfig *gardenletv1alpha1.GardenletConfiguration
		gardenletChartValues  map[string]any
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = mockclient.NewMockClient(ctrl)
		gardenAPIReader = mockclient.NewMockReader(ctrl)
		seedClient = mockclient.NewMockClient(ctrl)
		shootClient = mockclient.NewMockClient(ctrl)
		shootClientSet = kubernetesmock.NewMockInterface(ctrl)
		shootClientMap = mockclientmap.NewMockClientMap(ctrl)
		vh = mockgardenletdepoyer.NewMockValuesHelper(ctrl)
		shootChartApplier = kubernetesmock.NewMockChartApplier(ctrl)
		recorder = mockrecord.NewMockEventRecorder(ctrl)

		shootClientSet.EXPECT().Client().Return(shootClient).AnyTimes()
		shootClientSet.EXPECT().ChartApplier().Return(shootChartApplier).AnyTimes()

		log = logr.Discard()
		actuator = &Actuator{
			GardenConfig:         &rest.Config{},
			GardenAPIReader:      gardenAPIReader,
			GardenClient:         gardenClient,
			SeedClient:           seedClient,
			ShootClientMap:       shootClientMap,
			Clock:                clock.RealClock{},
			ValuesHelper:         vh,
			Recorder:             recorder,
			GardenNamespaceShoot: namespace,
		}

		ctx = context.TODO()

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Shoot: &seedmanagementv1alpha1.Shoot{
					Name: name,
				},
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					EnableStaticTokenKubeconfig: ptr.To(true),
				},
				SecretBindingName: ptr.To(secretBindingName),
				SeedName:          ptr.To(seedName),
			},
			Status: gardencorev1beta1.ShootStatus{
				LastOperation: &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
				},
				ObservedGeneration: 1,
				TechnicalID:        "shoot--" + namespace + "--" + name,
			},
		}
		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretBindingName,
				Namespace: namespace,
			},
			SecretRef: corev1.SecretReference{
				Name:      secretName,
				Namespace: namespace,
			},
		}
		credentialsBinding = &securityv1alpha1.CredentialsBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretBindingName,
				Namespace: namespace,
			},
			CredentialsRef: corev1.ObjectReference{
				Name:      secretName,
				Namespace: namespace,
			},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		seedTemplate = &gardencorev1beta1.SeedTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"foo": "bar",
				},
				Annotations: map[string]string{
					"bar": "baz",
				},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Backup: &gardencorev1beta1.SeedBackup{
					SecretRef: corev1.SecretReference{
						Name:      backupSecretName,
						Namespace: namespace,
					},
				},
				Settings: &gardencorev1beta1.SeedSettings{
					VerticalPodAutoscaler: &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{
						Enabled: true,
					},
				},
				Ingress: &gardencorev1beta1.Ingress{},
			},
		}
		gardenlet = &seedmanagementv1alpha1.GardenletConfig{
			Deployment: &seedmanagementv1alpha1.GardenletDeployment{
				ReplicaCount:         ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](1),
				Image: &seedmanagementv1alpha1.Image{
					PullPolicy: ptr.To(corev1.PullIfNotPresent),
				},
				VPA: ptr.To(true),
			},
			Config: runtime.RawExtension{
				Object: &gardenletv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					SeedConfig: &gardenletv1alpha1.SeedConfig{
						SeedTemplate: *seedTemplate,
					},
				},
			},
			Bootstrap:       ptr.To(seedmanagementv1alpha1.BootstrapToken),
			MergeWithParent: ptr.To(true),
		}

		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1beta1constants.GardenNamespace,
			},
		}
		backupSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupSecretName,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
				},
			},
			Data: secret.Data,
			Type: corev1.SecretTypeOpaque,
		}
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: utils.MergeStringMaps(seedTemplate.Labels, map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
				}),
				Annotations: seedTemplate.Annotations,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
				},
			},
			Spec: seedTemplate.Spec,
		}
		gardenletDeployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DeploymentNameGardenlet,
				Namespace: v1beta1constants.GardenNamespace,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		expectGetShoot = func() {
			gardenAPIReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = *shoot
					return nil
				},
			)
		}

		expectCreateGardenNamespace = func() {
			shootClient.EXPECT().Get(ctx, client.ObjectKey{Name: v1beta1constants.GardenNamespace}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Namespace, _ ...client.GetOption) error {
					return apierrors.NewNotFound(corev1.Resource("namespace"), v1beta1constants.GardenNamespace)
				},
			)
			shootClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(
				func(_ context.Context, ns *corev1.Namespace, _ ...client.CreateOption) error {
					Expect(ns.Name).To(Equal(v1beta1constants.GardenNamespace))
					return nil
				},
			)
		}

		expectDeleteGardenNamespace = func() {
			shootClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(
				func(_ context.Context, ns *corev1.Namespace, _ ...client.DeleteOption) error {
					Expect(ns.Name).To(Equal(v1beta1constants.GardenNamespace))
					return nil
				},
			)
		}

		expectGetGardenNamespace = func(exists bool) {
			shootClient.EXPECT().Get(ctx, client.ObjectKey{Name: v1beta1constants.GardenNamespace}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, ns *corev1.Namespace, _ ...client.GetOption) error {
					if exists {
						*ns = *gardenNamespace
						return nil
					}
					return apierrors.NewNotFound(corev1.Resource("namespace"), v1beta1constants.GardenNamespace)
				},
			)
		}

		expectCheckSeedSpec = func() {
			// Check if the shoot namespace in the seed contains a vpa-admission-controller deployment
			seedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "vpa-admission-controller"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *appsv1.Deployment, _ ...client.GetOption) error {
					return apierrors.NewNotFound(appsv1.Resource("deployment"), "vpa-admission-controller")
				},
			)
		}

		expectCreateSeedSecrets = func() {
			// Get shoot secret
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretBindingName}, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBinding{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, sb *gardencorev1beta1.SecretBinding, _ ...client.GetOption) error {
					*sb = *secretBinding
					return nil
				},
			)
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
					*s = *secret
					return nil
				},
			)

			// Create backup secret
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: backupSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret, _ ...client.GetOption) error {
					return apierrors.NewNotFound(corev1.Resource("secret"), backupSecretName)
				},
			)
			gardenClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, s *corev1.Secret, _ ...client.CreateOption) error {
					Expect(s).To(Equal(backupSecret))
					return nil
				},
			)
		}

		expectCreateSeedSecretsShootWithCredsBinding = func() {
			// Get shoot secret
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: credentialsBindingName}, gomock.AssignableToTypeOf(&securityv1alpha1.CredentialsBinding{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, cb *securityv1alpha1.CredentialsBinding, _ ...client.GetOption) error {
					*cb = *credentialsBinding
					return nil
				},
			)
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
					*s = *secret
					return nil
				},
			)

			// Create backup secret
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: backupSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret, _ ...client.GetOption) error {
					return apierrors.NewNotFound(corev1.Resource("secret"), backupSecretName)
				},
			)
			gardenClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, s *corev1.Secret, _ ...client.CreateOption) error {
					Expect(s).To(Equal(backupSecret))
					return nil
				},
			)
		}

		expectDeleteSeedSecrets = func() {
			// Delete backup secret
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: backupSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
					*s = *backupSecret
					return nil
				},
			)
			gardenClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, s *corev1.Secret, _ ...client.DeleteOption) error {
					Expect(s.Name).To(Equal(backupSecretName))
					Expect(s.Namespace).To(Equal(namespace))
					return nil
				},
			)
		}

		expectGetSeedSecrets = func(exist bool) {
			// Get backup secret
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: backupSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
					if exist {
						*s = *backupSecret
						return nil
					}
					return apierrors.NewNotFound(corev1.Resource("secret"), backupSecretName)
				},
			)
		}

		expectDeleteSeed = func() {
			gardenClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Seed, _ ...client.DeleteOption) error {
					Expect(s.Name).To(Equal(name))
					return nil
				},
			)
		}

		expectGetSeed = func(exists bool) {
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Name: name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed, _ ...client.GetOption) error {
					if exists {
						*s = *seed
						return nil
					}
					return apierrors.NewNotFound(gardencorev1beta1.Resource("seed"), name)
				},
			)
		}

		expectMergeWithParent = func() {
			mergedDeployment = managedSeed.Spec.Gardenlet.Deployment.DeepCopy()
			mergedDeployment.Image = &seedmanagementv1alpha1.Image{
				Repository: ptr.To("repository"),
				Tag:        ptr.To("tag"),
				PullPolicy: ptr.To(corev1.PullIfNotPresent),
			}

			mergedGardenletConfig = managedSeed.Spec.Gardenlet.Config.Object.(*gardenletv1alpha1.GardenletConfiguration).DeepCopy()
			mergedGardenletConfig.GardenClientConnection = &gardenletv1alpha1.GardenClientConnection{
				ClientConnectionConfiguration: v1alpha1.ClientConnectionConfiguration{
					Kubeconfig: "kubeconfig",
				},
			}

			vh.EXPECT().MergeGardenletDeployment(managedSeed.Spec.Gardenlet.Deployment, shoot).Return(mergedDeployment, nil)
			vh.EXPECT().MergeGardenletConfiguration(managedSeed.Spec.Gardenlet.Config.Object).Return(mergedGardenletConfig, nil)
		}

		expectDeleteKubeconfigSecret = func() {
			shootClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet-kubeconfig"}})
		}

		expectPrepareGardenClientConnection = func(withAlreadyBootstrappedCheck bool) {
			if withAlreadyBootstrappedCheck {
				// Check if kubeconfig secret exists
				shootClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet-kubeconfig"}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret, _ ...client.GetOption) error {
						return apierrors.NewNotFound(corev1.Resource("secret"), "gardenlet-kubeconfig")
					},
				)
			}

			// Create bootstrap token secret
			gardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "bootstrap-token-a82f8a"}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret, _ ...client.GetOption) error {
					return apierrors.NewNotFound(corev1.Resource("secret"), "bootstrap-token-a82f8a")
				},
			).Times(3)
			gardenClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, s *corev1.Secret, _ ...client.CreateOption) error {
					Expect(s.Name).To(Equal("bootstrap-token-a82f8a"))
					Expect(s.Namespace).To(Equal(metav1.NamespaceSystem))
					Expect(s.Type).To(Equal(corev1.SecretTypeBootstrapToken))
					Expect(s.Data).To(HaveKeyWithValue("token-id", []byte("a82f8a")))
					Expect(s.Data).To(HaveKey("token-secret"))
					Expect(s.Data).To(HaveKeyWithValue("usage-bootstrap-signing", []byte("true")))
					Expect(s.Data).To(HaveKeyWithValue("usage-bootstrap-authentication", []byte("true")))
					return nil
				},
			)
		}

		expectGetGardenletChartValues = func(withBootstrap bool) {
			gardenletChartValues = map[string]any{"foo": "bar"}

			vh.EXPECT().GetGardenletChartValues(mergedDeployment, gomock.AssignableToTypeOf(&gardenletv1alpha1.GardenletConfiguration{}), gomock.AssignableToTypeOf("")).DoAndReturn(
				func(_ *seedmanagementv1alpha1.GardenletDeployment, gc *gardenletv1alpha1.GardenletConfiguration, _ string) (map[string]any, error) {
					if withBootstrap {
						Expect(gc.GardenClientConnection.Kubeconfig).To(Equal(""))
						Expect(gc.GardenClientConnection.KubeconfigSecret).To(Equal(&corev1.SecretReference{
							Name:      "gardenlet-kubeconfig",
							Namespace: v1beta1constants.GardenNamespace,
						}))
						Expect(gc.GardenClientConnection.BootstrapKubeconfig).To(Equal(&corev1.SecretReference{
							Name:      "gardenlet-kubeconfig-bootstrap",
							Namespace: v1beta1constants.GardenNamespace,
						}))
					} else {
						Expect(gc.GardenClientConnection.Kubeconfig).To(Equal("kubeconfig"))
						Expect(gc.GardenClientConnection.KubeconfigSecret).To(BeNil())
						Expect(gc.GardenClientConnection.BootstrapKubeconfig).To(BeNil())
					}
					Expect(gc.SeedConfig.SeedTemplate).To(Equal(gardencorev1beta1.SeedTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Name:        name,
							Labels:      seedTemplate.Labels,
							Annotations: seedTemplate.Annotations,
						},
						Spec: seedTemplate.Spec,
					}))

					return gardenletChartValues, nil
				},
			)
		}

		expectApplyGardenletChart = func() {
			shootChartApplier.EXPECT().ApplyFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(gardenletChartValues)).Return(nil)
		}

		expectDeleteGardenletChart = func() {
			shootChartApplier.EXPECT().DeleteFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(gardenletChartValues)).Return(nil)
		}

		expectGetGardenletDeployment = func(exists bool) {
			shootClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: v1beta1constants.DeploymentNameGardenlet}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, d *appsv1.Deployment, _ ...client.GetOption) error {
					if exists {
						*d = *gardenletDeployment
						return nil
					}
					return apierrors.NewNotFound(appsv1.Resource("deployment"), v1beta1constants.DeploymentNameGardenlet)
				},
			)
		}
	)

	Describe("#Reconcile", func() {
		BeforeEach(func() {
			shootClientMap.EXPECT().GetClient(ctx, keys.ForShoot(shoot)).Return(shootClientSet, nil).AnyTimes()
			gardenClient.EXPECT().Scheme().Return(kubernetes.GardenScheme).AnyTimes()
		})

		It("should wait if the Shoot is still reconciling", func() {
			shoot.ObjectMeta.Generation = 2
			expectGetShoot()
			recorder.EXPECT().Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Waiting for shoot \""+client.ObjectKeyFromObject(shoot).String()+"\" to be reconciled")

			status, wait, err := actuator.Reconcile(ctx, log, managedSeed)
			Expect(err).ToNot(HaveOccurred())
			Expect(status.Conditions).To(ContainCondition(
				OfType(seedmanagementv1alpha1.ManagedSeedShootReconciled),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason(gardencorev1beta1.EventReconciling),
			))
			Expect(wait).To(BeTrue())
		})

		Context("gardenlet", func() {
			BeforeEach(func() {
				managedSeed.Spec.Gardenlet = gardenlet
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap and shoot using credentialsbindings name)", func() {
				shoot.Spec.CredentialsBindingName = ptr.To(credentialsBindingName)
				shoot.Spec.SecretBindingName = nil
				expectGetShoot()
				expectGetSeed(false)
				expectCheckSeedSpec()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Ensuring garden namespace in shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectCreateGardenNamespace()
				recorder.EXPECT().Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling seed secrets")
				expectCreateSeedSecretsShootWithCredsBinding()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet into shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectMergeWithParent()
				expectPrepareGardenClientConnection(true)
				expectGetGardenletChartValues(true)
				expectApplyGardenletChart()

				status, wait, err := actuator.Reconcile(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(And(
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedShootReconciled),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
				))
				Expect(wait).To(BeFalse())
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap)", func() {
				expectGetShoot()
				expectGetSeed(false)
				expectCheckSeedSpec()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Ensuring garden namespace in shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectCreateGardenNamespace()
				recorder.EXPECT().Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling seed secrets")
				expectCreateSeedSecrets()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet into shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectMergeWithParent()
				expectPrepareGardenClientConnection(true)
				expectGetGardenletChartValues(true)
				expectApplyGardenletChart()

				status, wait, err := actuator.Reconcile(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(And(
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedShootReconciled),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
				))
				Expect(wait).To(BeFalse())
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap and non-expired gardenlet client cert)", func() {
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: time.Now().Add(time.Hour)}

				expectGetShoot()
				expectGetSeed(true)
				expectCheckSeedSpec()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Ensuring garden namespace in shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectCreateGardenNamespace()
				recorder.EXPECT().Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling seed secrets")
				expectCreateSeedSecrets()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet into shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectMergeWithParent()
				expectPrepareGardenClientConnection(true)
				expectGetGardenletChartValues(true)
				expectApplyGardenletChart()

				status, wait, err := actuator.Reconcile(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(And(
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedShootReconciled),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
				))
				Expect(wait).To(BeFalse())
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap and expired gardenlet client cert)", func() {
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: time.Now().Add(-time.Hour)}

				expectGetShoot()
				expectDeleteKubeconfigSecret()
				expectGetSeed(true)
				expectCheckSeedSpec()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Ensuring garden namespace in shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectCreateGardenNamespace()
				recorder.EXPECT().Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling seed secrets")
				expectCreateSeedSecrets()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet into shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectMergeWithParent()
				expectPrepareGardenClientConnection(false)
				expectGetGardenletChartValues(true)
				expectApplyGardenletChart()

				status, wait, err := actuator.Reconcile(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(And(
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedShootReconciled),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
				))
				Expect(wait).To(BeFalse())
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap, non-expired gardenlet client cert, and renew-kubeconfig annotation)", func() {
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: time.Now().Add(time.Hour)}
				managedSeed.Annotations = map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRenewKubeconfig,
				}

				expectGetShoot()
				recorder.EXPECT().Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Renewing gardenlet kubeconfig secret due to operation annotation")
				expectDeleteKubeconfigSecret()
				gardenClient.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"annotations":null}}`))
					return nil
				})
				expectGetSeed(true)
				expectCheckSeedSpec()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Ensuring garden namespace in shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectCreateGardenNamespace()
				recorder.EXPECT().Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling seed secrets")
				expectCreateSeedSecrets()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet into shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectMergeWithParent()
				expectPrepareGardenClientConnection(false)
				expectGetGardenletChartValues(true)
				expectApplyGardenletChart()

				status, wait, err := actuator.Reconcile(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(And(
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedShootReconciled),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
				))
				Expect(wait).To(BeFalse())
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (without bootstrap)", func() {
				managedSeed.Spec.Gardenlet.Bootstrap = ptr.To(seedmanagementv1alpha1.BootstrapNone)

				expectGetShoot()
				expectGetSeed(false)
				expectCheckSeedSpec()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Ensuring garden namespace in shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectCreateGardenNamespace()
				recorder.EXPECT().Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling seed secrets")
				expectCreateSeedSecrets()
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet into shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectMergeWithParent()
				expectGetGardenletChartValues(false)
				expectApplyGardenletChart()

				status, wait, err := actuator.Reconcile(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(And(
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedShootReconciled),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
					ContainCondition(
						OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
						WithStatus(gardencorev1beta1.ConditionTrue),
						WithReason(gardencorev1beta1.EventReconciled),
					),
				))
				Expect(wait).To(BeFalse())
			})
		})
	})

	Describe("#Delete", func() {
		BeforeEach(func() {
			shootClientMap.EXPECT().GetClient(ctx, keys.ForShoot(shoot)).Return(shootClientSet, nil)
		})

		Context("gardenlet", func() {
			BeforeEach(func() {
				managedSeed.Spec.Gardenlet = gardenlet
			})

			It("should delete the seed if it still exists", func() {
				expectGetShoot()
				expectGetSeed(true)
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting seed %s", name)
				expectDeleteSeed()

				status, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleting),
				))
				Expect(wait).To(BeFalse())
				Expect(removeFinalizer).To(BeFalse())
			})

			It("should delete gardenlet if it still exists", func() {
				expectGetShoot()
				expectGetSeed(false)
				expectGetGardenletDeployment(true)
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting gardenlet from shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectMergeWithParent()
				expectPrepareGardenClientConnection(true)
				expectGetGardenletChartValues(true)
				expectDeleteGardenletChart()

				status, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleting),
				))
				Expect(wait).To(BeTrue())
				Expect(removeFinalizer).To(BeFalse())
			})

			It("should delete the seed secrets if they still exist", func() {
				expectGetShoot()
				expectGetSeed(false)
				expectGetGardenletDeployment(false)
				expectGetSeedSecrets(true)
				recorder.EXPECT().Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting seed secrets")
				expectDeleteSeedSecrets()

				status, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleting),
				),
				)
				Expect(wait).To(BeTrue())
				Expect(removeFinalizer).To(BeFalse())
			})

			It("should delete the garden namespace if it still exists, and set wait to true", func() {
				expectGetShoot()
				expectGetSeed(false)
				expectGetGardenletDeployment(false)
				expectGetSeedSecrets(false)
				expectGetGardenNamespace(true)
				recorder.EXPECT().Eventf(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting garden namespace from shoot %q", client.ObjectKeyFromObject(shoot).String())
				expectDeleteGardenNamespace()

				status, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleting),
				))
				Expect(wait).To(BeTrue())
				Expect(removeFinalizer).To(BeFalse())
			})

			It("should do nothing if neither the seed, nor gardenlet, nor the seed secrets, nor the garden namespace exist, and set removeFinalizer to true", func() {
				expectGetShoot()
				expectGetSeed(false)
				expectGetGardenletDeployment(false)
				expectGetSeedSecrets(false)
				expectGetGardenNamespace(false)

				status, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(status.Conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleted),
				))
				Expect(wait).To(BeFalse())
				Expect(removeFinalizer).To(BeTrue())
			})
		})
	})
})

var _ = Describe("Utils", func() {
	Describe("#ensureGardenletEnvironment", func() {
		const (
			kubernetesServiceHost = "KUBERNETES_SERVICE_HOST"
			preserveDomain        = "preserve-value.example.com"
		)
		var (
			otherEnvDeployment = &seedmanagementv1alpha1.GardenletDeployment{
				Env: []corev1.EnvVar{
					{Name: "TEST_VAR", Value: "TEST_VALUE"},
				},
			}
			kubernetesServiceHostEnvDeployment = &seedmanagementv1alpha1.GardenletDeployment{
				Env: []corev1.EnvVar{
					{Name: kubernetesServiceHost, Value: preserveDomain},
				},
			}

			dnsWithDomain = &gardencorev1beta1.DNS{
				Domain: ptr.To("my-shoot.example.com"),
			}
			dnsWithoutDomain = &gardencorev1beta1.DNS{
				Domain: nil,
			}
		)

		It("should not overwrite existing KUBERNETES_SERVICE_HOST environment", func() {
			ensuredDeploymentWithDomain := ensureGardenletEnvironment(kubernetesServiceHostEnvDeployment, dnsWithDomain)
			ensuredDeploymentWithoutDomain := ensureGardenletEnvironment(kubernetesServiceHostEnvDeployment, dnsWithoutDomain)

			Expect(ensuredDeploymentWithDomain.Env[0].Name).To(Equal(kubernetesServiceHost))
			Expect(ensuredDeploymentWithDomain.Env[0].Value).To(Equal(preserveDomain))
			Expect(ensuredDeploymentWithDomain.Env[0].Value).ToNot(Equal(gardenerutils.GetAPIServerDomain(*dnsWithDomain.Domain)))

			Expect(ensuredDeploymentWithoutDomain.Env[0].Name).To(Equal(kubernetesServiceHost))
			Expect(ensuredDeploymentWithoutDomain.Env[0].Value).To(Equal(preserveDomain))

		})

		It("should should not inject KUBERNETES_SERVICE_HOST environment", func() {
			ensuredDeploymentWithoutDomain := ensureGardenletEnvironment(otherEnvDeployment, dnsWithoutDomain)

			Expect(ensuredDeploymentWithoutDomain.Env).To(HaveLen(1))
			Expect(ensuredDeploymentWithoutDomain.Env[0].Name).ToNot(Equal(kubernetesServiceHost))
		})
		It("should should inject KUBERNETES_SERVICE_HOST environment", func() {
			ensuredDeploymentWithoutDomain := ensureGardenletEnvironment(otherEnvDeployment, dnsWithDomain)

			Expect(ensuredDeploymentWithoutDomain.Env).To(HaveLen(2))
			Expect(ensuredDeploymentWithoutDomain.Env[0].Name).ToNot(Equal(kubernetesServiceHost))
			Expect(ensuredDeploymentWithoutDomain.Env[1].Name).To(Equal(kubernetesServiceHost))
			Expect(ensuredDeploymentWithoutDomain.Env[1].Value).To(Equal(gardenerutils.GetAPIServerDomain(*dnsWithDomain.Domain)))

		})
	})
})
