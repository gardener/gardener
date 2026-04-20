// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenletdeployer

import (
	"context"
	"fmt"
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
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/charts"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockgardenletdepoyer "github.com/gardener/gardener/pkg/controller/gardenletdeployer/mock"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockevents "github.com/gardener/gardener/third_party/mock/client-go/tools/events"
)

const (
	name             = "test"
	namespace        = "garden"
	backupSecretName = "test-backup-secret"
)

var _ = Describe("Interface", func() {
	var (
		ctrl *gomock.Controller

		gardenClient      client.Client
		seedClient        client.Client
		shootClient       client.Client
		shootClientSet    kubernetes.Interface
		vh                *mockgardenletdepoyer.MockValuesHelper
		shootChartApplier *kubernetesmock.MockChartApplier
		recorder          *mockevents.MockEventRecorder

		log      logr.Logger
		actuator *Actuator

		ctx context.Context

		managedSeed *seedmanagementv1alpha1.ManagedSeed

		seedTemplate *gardencorev1beta1.SeedTemplate
		gardenlet    seedmanagementv1alpha1.GardenletConfig

		gardenNamespace     *corev1.Namespace
		backupSecret        *corev1.Secret
		seed                *gardencorev1beta1.Seed
		gardenletDeployment *appsv1.Deployment

		mergedDeployment      *seedmanagementv1alpha1.GardenletDeployment
		mergedGardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration
		gardenletChartValues  map[string]any
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		vh = mockgardenletdepoyer.NewMockValuesHelper(ctrl)
		shootChartApplier = kubernetesmock.NewMockChartApplier(ctrl)
		recorder = mockevents.NewMockEventRecorder(ctrl)

		shootClientSet = fakekubernetes.NewClientSetBuilder().WithClient(shootClient).WithChartApplier(shootChartApplier).Build()

		log = logr.Discard()
		actuator = &Actuator{
			GardenConfig: &rest.Config{},
			GardenClient: gardenClient,
			GetTargetClientFunc: func(_ context.Context) (kubernetes.Interface, error) {
				return shootClientSet, nil
			},
			CheckIfVPAAlreadyExists: func(ctx context.Context) (bool, error) {
				if err := seedClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpa-admission-controller"}, &appsv1.Deployment{}); err != nil {
					if apierrors.IsNotFound(err) {
						return false, nil
					}
					return false, err
				}
				return true, nil
			},
			GetTargetDomain: func() string {
				return ""
			},
			ApplyGardenletChart: func(ctx context.Context, targetChartApplier kubernetes.ChartApplier, values map[string]any) error {
				return targetChartApplier.ApplyFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, namespace, "gardenlet", kubernetes.Values(values))
			},
			DeleteGardenletChart: func(ctx context.Context, targetChartApplier kubernetes.ChartApplier, values map[string]any) error {
				return targetChartApplier.DeleteFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, namespace, "gardenlet", kubernetes.Values(values))
			},
			Clock:                    clock.RealClock{},
			ValuesHelper:             vh,
			Recorder:                 recorder,
			GardenletNamespaceTarget: namespace,
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
				Backup: &gardencorev1beta1.Backup{
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       backupSecretName,
						Namespace:  namespace,
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
		gardenlet = seedmanagementv1alpha1.GardenletConfig{
			Deployment: &seedmanagementv1alpha1.GardenletDeployment{
				ReplicaCount:         ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](1),
				Image: &seedmanagementv1alpha1.Image{
					PullPolicy: ptr.To(corev1.PullIfNotPresent),
				},
			},
			Config: runtime.RawExtension{
				Object: &gardenletconfigv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
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
			Data: map[string][]byte{
				"backupKey": []byte("backupValue"),
			},
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
		expectCreateGardenNamespace = func() {
			GinkgoHelper()

			namespace := &corev1.Namespace{}
			Expect(shootClient.Get(ctx, client.ObjectKey{Name: v1beta1constants.GardenNamespace}, namespace)).To(Succeed())
			Expect(namespace.Labels).To(HaveKeyWithValue("gardener.cloud/role", "garden"))
		}

		expectCreateSeedSecrets = func() {
			GinkgoHelper()

			// Verify backup secret owner reference was removed
			backupSecret := &corev1.Secret{}
			Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: backupSecretName}, backupSecret)).To(Succeed())
			Expect(backupSecret.OwnerReferences).To(BeEmpty())
			Expect(backupSecret.Labels).To(HaveKeyWithValue("secret.backup.gardener.cloud/status", "previously-managed"))
		}

		expectMergeWithParent = func() {
			mergedDeployment = managedSeed.Spec.Gardenlet.Deployment.DeepCopy()
			mergedDeployment.Image = &seedmanagementv1alpha1.Image{
				Repository: ptr.To("repository"),
				Tag:        ptr.To("tag"),
				PullPolicy: ptr.To(corev1.PullIfNotPresent),
			}

			mergedGardenletConfig = managedSeed.Spec.Gardenlet.Config.Object.(*gardenletconfigv1alpha1.GardenletConfiguration).DeepCopy()
			mergedGardenletConfig.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
				ClientConnectionConfiguration: v1alpha1.ClientConnectionConfiguration{
					Kubeconfig: "kubeconfig",
				},
			}

			vh.EXPECT().MergeGardenletDeployment(managedSeed.Spec.Gardenlet.Deployment).Return(mergedDeployment, nil)
			vh.EXPECT().MergeGardenletConfiguration(managedSeed.Spec.Gardenlet.Config.Object).Return(mergedGardenletConfig, nil)
		}

		expectPrepareGardenClientConnection = func() {
			GinkgoHelper()

			// Verify bootstrap kubeconfig secret was created
			bootstrapKubeconfig := &corev1.Secret{}
			Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "bootstrap-token-295eab"}, bootstrapKubeconfig)).To(Succeed())
			Expect(bootstrapKubeconfig.Type).To(Equal(corev1.SecretTypeBootstrapToken))
			Expect(bootstrapKubeconfig.Data).To(HaveKeyWithValue("token-id", []byte("295eab")))
			Expect(bootstrapKubeconfig.Data).To(HaveKey("token-secret"))
			Expect(bootstrapKubeconfig.Data).To(HaveKeyWithValue("usage-bootstrap-signing", []byte("true")))
			Expect(bootstrapKubeconfig.Data).To(HaveKeyWithValue("usage-bootstrap-authentication", []byte("true")))
		}

		expectGetGardenletChartValues = func(withBootstrap, seedIsGarden, selfHostedShoot bool) {
			gardenletChartValues = map[string]any{"foo": "bar"}

			vh.EXPECT().GetGardenletChartValues(gomock.AssignableToTypeOf(&seedmanagementv1alpha1.GardenletDeployment{}), gomock.AssignableToTypeOf(&gardenletconfigv1alpha1.GardenletConfiguration{}), gomock.AssignableToTypeOf("")).DoAndReturn(
				func(deployment *seedmanagementv1alpha1.GardenletDeployment, gc *gardenletconfigv1alpha1.GardenletConfiguration, _ string) (map[string]any, error) {
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

					if seedIsGarden {
						Expect(deployment.PodLabels).To(HaveKeyWithValue("networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443", "allowed"))
					} else {
						Expect(deployment.PodLabels).To(BeEmpty())
					}

					if !selfHostedShoot {
						Expect(gc.SeedConfig.SeedTemplate).To(Equal(gardencorev1beta1.SeedTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Name:        name,
								Labels:      seedTemplate.Labels,
								Annotations: seedTemplate.Annotations,
							},
							Spec: seedTemplate.Spec,
						}))
					}

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
	)

	Describe("#Reconcile", func() {
		Context("gardenlet", func() {
			BeforeEach(func() {
				managedSeed.Spec.Gardenlet = gardenlet
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap)", func() {
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")

				expectMergeWithParent()
				expectGetGardenletChartValues(true, false, false)
				expectApplyGardenletChart()

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()
				expectPrepareGardenClientConnection()
			})

			It("should not create seed secrets when backup is using WorkloadIdentity credentials", func() {
				seedTemplate.Spec.Backup.CredentialsRef = &corev1.ObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Namespace:  "garden",
					Name:       "backup",
				}
				managedSeed.Spec.Gardenlet.Config = runtime.RawExtension{
					Object: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: *seedTemplate,
						},
					},
				}

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				expectMergeWithParent()
				expectGetGardenletChartValues(true, false, false)
				expectApplyGardenletChart()

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectPrepareGardenClientConnection()
			})

			It("should return error when backup secret does not exist", func() {
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, gardencorev1beta1.EventActionReconcile, gomock.Any())

				_, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("could not reconcile seed test secrets: the configured backup secret does not exist")))
			})

			It("should remove owner reference from backup secret", func() {
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")
				expectMergeWithParent()
				expectGetGardenletChartValues(true, false, false)
				expectApplyGardenletChart()
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()
				expectPrepareGardenClientConnection()
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap and non-expired gardenlet client cert)", func() {
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: time.Now().Add(time.Hour)}

				Expect(gardenClient.Create(ctx, seed.DeepCopy())).To(Succeed())
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")

				expectMergeWithParent()
				expectGetGardenletChartValues(true, false, false)
				expectApplyGardenletChart()

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap and expired gardenlet client cert)", func() {
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: time.Now().Add(-time.Hour)}

				Expect(gardenClient.Create(ctx, seed.DeepCopy())).To(Succeed())
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())
				Expect(shootClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      "gardenlet-kubeconfig",
					Namespace: v1beta1constants.GardenNamespace,
				}})).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")

				expectMergeWithParent()
				expectGetGardenletChartValues(true, false, false)
				expectApplyGardenletChart()

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()
				expectPrepareGardenClientConnection()

				Expect(shootClient.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet-kubeconfig"}, &corev1.Secret{})).To(BeNotFoundError())
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap, non-expired gardenlet client cert, and renew-kubeconfig annotation)", func() {
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: time.Now().Add(time.Hour)}
				managedSeed.Annotations = map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRenewKubeconfig,
				}

				Expect(gardenClient.Create(ctx, seed.DeepCopy())).To(Succeed())
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())
				Expect(gardenClient.Create(ctx, managedSeed.DeepCopy())).To(Succeed())
				Expect(shootClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      "gardenlet-kubeconfig",
					Namespace: v1beta1constants.GardenNamespace,
				}})).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Renewing gardenlet kubeconfig secret due to operation annotation")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")

				expectMergeWithParent()
				expectGetGardenletChartValues(true, false, false)
				expectApplyGardenletChart()

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()
				expectPrepareGardenClientConnection()

				Expect(shootClient.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet-kubeconfig"}, &corev1.Secret{})).To(BeNotFoundError())

				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &seedmanagementv1alpha1.ManagedSeed{})).To(Succeed())
				Expect(managedSeed.Annotations).NotTo(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig))
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (without bootstrap)", func() {
				managedSeed.Spec.Gardenlet.Bootstrap = ptr.To(seedmanagementv1alpha1.BootstrapNone)

				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")

				expectMergeWithParent()
				expectGetGardenletChartValues(false, false, false)
				expectApplyGardenletChart()

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()

				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "bootstrap-token-295eab"}, &corev1.Secret{})).To(BeNotFoundError())
			})
		})

		Context("self-hosted shoot", func() {
			var (
				deployment *seedmanagementv1alpha1.GardenletDeployment
				config     *gardenletconfigv1alpha1.GardenletConfiguration
				shoot      *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				actuator.BootstrapToken = "token-for-gardenlet"

				deployment = &seedmanagementv1alpha1.GardenletDeployment{}
				config = &gardenletconfigv1alpha1.GardenletConfiguration{}
				shoot = &gardencorev1beta1.Shoot{}
			})

			It("should deploy the gardenlet", func() {
				recorder.EXPECT().Eventf(shoot, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(shoot, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				vh.EXPECT().MergeGardenletDeployment(deployment).Return(deployment, nil)
				expectGetGardenletChartValues(true, false, true)
				expectApplyGardenletChart()

				conditions, err := actuator.Reconcile(ctx, log, shoot, nil, deployment, &runtime.RawExtension{Object: config}, seedmanagementv1alpha1.BootstrapToken, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
			})
		})

		Context("seed is garden", func() {
			BeforeEach(func() {
				managedSeed.Spec.Gardenlet = gardenlet

				Expect(shootClient.Create(ctx, &operatorv1alpha1.Garden{
					ObjectMeta: metav1.ObjectMeta{Name: "garden"},
				})).To(Succeed())
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap)", func() {
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")
				expectMergeWithParent()
				expectGetGardenletChartValues(true, true, false)
				expectApplyGardenletChart()
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()
				expectPrepareGardenClientConnection()
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap and non-expired gardenlet client cert)", func() {
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: time.Now().Add(time.Hour)}

				Expect(gardenClient.Create(ctx, seed.DeepCopy())).To(Succeed())
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")

				expectMergeWithParent()
				expectGetGardenletChartValues(true, true, false)
				expectApplyGardenletChart()

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap and expired gardenlet client cert)", func() {
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: time.Now().Add(-time.Hour)}

				Expect(gardenClient.Create(ctx, seed.DeepCopy())).To(Succeed())
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())
				Expect(shootClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      "gardenlet-kubeconfig",
					Namespace: v1beta1constants.GardenNamespace,
				}})).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")

				expectMergeWithParent()
				expectGetGardenletChartValues(true, true, false)
				expectApplyGardenletChart()

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()
				expectPrepareGardenClientConnection()

				Expect(shootClient.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet-kubeconfig"}, &corev1.Secret{})).To(BeNotFoundError())
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (with bootstrap, non-expired gardenlet client cert, and renew-kubeconfig annotation)", func() {
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: time.Now().Add(time.Hour)}
				managedSeed.Annotations = map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRenewKubeconfig,
				}

				Expect(gardenClient.Create(ctx, seed.DeepCopy())).To(Succeed())
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())
				Expect(gardenClient.Create(ctx, managedSeed.DeepCopy())).To(Succeed())
				Expect(shootClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      "gardenlet-kubeconfig",
					Namespace: v1beta1constants.GardenNamespace,
				}})).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Renewing gardenlet kubeconfig secret due to operation annotation")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")

				expectMergeWithParent()
				expectGetGardenletChartValues(true, true, false)
				expectApplyGardenletChart()

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()
				expectPrepareGardenClientConnection()

				Expect(shootClient.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet-kubeconfig"}, &corev1.Secret{})).To(BeNotFoundError())

				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &seedmanagementv1alpha1.ManagedSeed{})).To(Succeed())
				Expect(managedSeed.Annotations).NotTo(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig))
			})

			It("should create the garden namespace and seed secrets, and deploy gardenlet (without bootstrap)", func() {
				managedSeed.Spec.Gardenlet.Bootstrap = ptr.To(seedmanagementv1alpha1.BootstrapNone)

				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Ensuring gardenlet namespace in target cluster")
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Reconciling seed secrets")

				expectMergeWithParent()
				expectGetGardenletChartValues(false, true, false)
				expectApplyGardenletChart()

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, gardencorev1beta1.EventActionReconcile, "Deploying gardenlet into target cluster")

				conditions, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(gardencorev1beta1.EventReconciled),
				))

				expectCreateGardenNamespace()
				expectCreateSeedSecrets()

				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "bootstrap-token-295eab"}, &corev1.Secret{})).To(BeNotFoundError())
			})

			It("should return error when existing gardenlet uses different seed name", func() {
				Expect(shootClient.Create(ctx, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardenlet",
						Namespace: v1beta1constants.GardenNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Volumes: []corev1.Volume{{
									Name: "gardenlet-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{Name: "gardenlet-config-12345"},
										},
									},
								}},
							},
						},
					},
				})).To(Succeed())

				Expect(shootClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardenlet-config-12345",
						Namespace: v1beta1constants.GardenNamespace,
					},
					Data: map[string]string{
						"config.yaml": fmt.Sprintf(`apiVersion: gardenlet.config.gardener.cloud/v1alpha1
kind: GardenletConfiguration
seedConfig:
  metadata:
    name: %s
`, "other-seed"),
					},
				})).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, gardencorev1beta1.EventActionReconcile, gomock.Any())

				_, err := actuator.Reconcile(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("found existing gardenlet deployment with ConfigMap")))
				Expect(err).To(MatchError(ContainSubstring(`uses seed name "other-seed"`)))
				Expect(err).To(MatchError(ContainSubstring(`doesn't match to "test"`)))
			})
		})

	})

	Describe("#Delete", func() {
		Context("gardenlet", func() {
			BeforeEach(func() {
				managedSeed.Spec.Gardenlet = gardenlet
			})

			It("should delete the seed if it still exists", func() {
				Expect(gardenClient.Create(ctx, seed.DeepCopy())).To(Succeed())
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, gardencorev1beta1.EventActionDelete, "Deleting seed %s", name)

				conditions, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).ToNot(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleting),
				))
				Expect(wait).To(BeFalse())
				Expect(removeFinalizer).To(BeFalse())

				Expect(gardenClient.Get(ctx, client.ObjectKey{Name: name}, &gardencorev1beta1.Seed{})).To(BeNotFoundError())
			})

			It("should delete gardenlet if it still exists", func() {
				Expect(shootClient.Create(ctx, gardenletDeployment.DeepCopy())).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, gardencorev1beta1.EventActionDelete, "Deleting gardenlet from target cluster")
				expectMergeWithParent()
				expectGetGardenletChartValues(true, false, false)
				expectDeleteGardenletChart()

				conditions, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).ToNot(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleting),
				))
				Expect(wait).To(BeTrue())
				Expect(removeFinalizer).To(BeFalse())

				expectPrepareGardenClientConnection()
			})

			It("should delete the seed secrets if they still exist", func() {
				Expect(gardenClient.Create(ctx, backupSecret.DeepCopy())).To(Succeed())
				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, gardencorev1beta1.EventActionDelete, "Deleting seed secrets")

				conditions, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).ToNot(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleting),
				))
				Expect(wait).To(BeTrue())
				Expect(removeFinalizer).To(BeFalse())

				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: backupSecretName}, &corev1.Secret{})).To(BeNotFoundError())
			})

			It("should not delete any seed secrets when backup is using WorkloadIdentity credentials", func() {
				seedTemplate.Spec.Backup.CredentialsRef = &corev1.ObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Namespace:  "garden",
					Name:       "backup",
				}
				managedSeed.Spec.Gardenlet.Config = runtime.RawExtension{
					Object: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: *seedTemplate,
						},
					},
				}

				conditions, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).ToNot(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleted),
				))
				Expect(wait).To(BeFalse())
				Expect(removeFinalizer).To(BeTrue())
			})

			It("should delete the garden namespace if it still exists, and set wait to true", func() {
				Expect(shootClient.Create(ctx, gardenNamespace.DeepCopy())).To(Succeed())

				recorder.EXPECT().Eventf(managedSeed, nil, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, gardencorev1beta1.EventActionDelete, "Deleting garden namespace from target cluster")

				conditions, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).ToNot(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason(gardencorev1beta1.EventDeleting),
				))
				Expect(wait).To(BeTrue())
				Expect(removeFinalizer).To(BeFalse())
			})

			It("should do nothing if neither the seed, nor gardenlet, nor the seed secrets, nor the garden namespace exist, and set removeFinalizer to true", func() {
				conditions, wait, removeFinalizer, err := actuator.Delete(ctx, log, managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &gardenlet.Config, *managedSeed.Spec.Gardenlet.Bootstrap, *managedSeed.Spec.Gardenlet.MergeWithParent)
				Expect(err).ToNot(HaveOccurred())
				Expect(conditions).To(ContainCondition(
					OfType(seedmanagementv1alpha1.SeedRegistered),
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

			domain = "my-shoot.example.com"
		)

		newActuator := func(domain string) *Actuator {
			return &Actuator{GetTargetDomain: func() string { return domain }}
		}

		It("should not overwrite existing KUBERNETES_SERVICE_HOST environment", func() {
			ensuredDeploymentWithDomain := newActuator(domain).ensureGardenletEnvironment(kubernetesServiceHostEnvDeployment)
			ensuredDeploymentWithoutDomain := newActuator("").ensureGardenletEnvironment(kubernetesServiceHostEnvDeployment)

			Expect(ensuredDeploymentWithDomain.Env[0].Name).To(Equal(kubernetesServiceHost))
			Expect(ensuredDeploymentWithDomain.Env[0].Value).To(Equal(preserveDomain))
			Expect(ensuredDeploymentWithDomain.Env[0].Value).ToNot(Equal(v1beta1helper.GetAPIServerDomain(domain)))

			Expect(ensuredDeploymentWithoutDomain.Env[0].Name).To(Equal(kubernetesServiceHost))
			Expect(ensuredDeploymentWithoutDomain.Env[0].Value).To(Equal(preserveDomain))
		})

		It("should should not inject KUBERNETES_SERVICE_HOST environment", func() {
			ensuredDeploymentWithoutDomain := newActuator("").ensureGardenletEnvironment(otherEnvDeployment)

			Expect(ensuredDeploymentWithoutDomain.Env).To(HaveLen(1))
			Expect(ensuredDeploymentWithoutDomain.Env[0].Name).ToNot(Equal(kubernetesServiceHost))
		})

		It("should should inject KUBERNETES_SERVICE_HOST environment", func() {
			ensuredDeploymentWithoutDomain := newActuator(domain).ensureGardenletEnvironment(otherEnvDeployment)

			Expect(ensuredDeploymentWithoutDomain.Env).To(HaveLen(2))
			Expect(ensuredDeploymentWithoutDomain.Env[0].Name).ToNot(Equal(kubernetesServiceHost))
			Expect(ensuredDeploymentWithoutDomain.Env[1].Name).To(Equal(kubernetesServiceHost))
			Expect(ensuredDeploymentWithoutDomain.Env[1].Value).To(Equal(v1beta1helper.GetAPIServerDomain(domain)))
		})

		It("should skip when SeedIsSelfHostedShoot is set", func() {
			a := &Actuator{
				GetTargetDomain:       func() string { return domain },
				SeedIsSelfHostedShoot: true,
			}
			deployment := &seedmanagementv1alpha1.GardenletDeployment{
				Env: []corev1.EnvVar{
					{Name: "TEST_VAR", Value: "TEST_VALUE"},
				},
			}
			result := a.ensureGardenletEnvironment(deployment)

			Expect(result.Env).To(HaveLen(1))
			Expect(result.Env[0].Name).To(Equal("TEST_VAR"))
		})
	})
})
