// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/gardener/gardener/landscaper/pkg/gardenlet/apis/imports"
	appliercommon "github.com/gardener/gardener/landscaper/pkg/gardenlet/chart/charttest"
)

const (
	chartsRootPath = "../../../../charts"
)

var _ = Describe("Gardenlet Landscaper reconciliation testing", func() {
	var (
		landscaper Landscaper
		seed       = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{
			Name: "sweet-seed",
		}}
		gardenletDeploymentConfiguration seedmanagement.GardenletDeployment

		gardenletConfigurationv1alpha1 = &gardenletconfigv1alpha1.GardenletConfiguration{
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{ObjectMeta: seed.ObjectMeta},
			},
			GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
				// need to explicitly set, otherwise will be defaulted to by RecommendedDefaultClientConnectionConfiguration
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   100,
					Burst: 130,
				},
				KubeconfigSecret: &corev1.SecretReference{
					Name:      "gardenlet-kubeconfig",
					Namespace: "garden",
				},
				BootstrapKubeconfig: &corev1.SecretReference{
					Name:      "gardenlet-kubeconfig-bootstrap",
					Namespace: "garden",
				},
			},
		}

		mockController      *gomock.Controller
		mockGardenClient    *mockclient.MockClient
		mockSeedClient      *mockclient.MockClient
		mockGardenInterface *mock.MockInterface
		mockSeedInterface   *mock.MockInterface

		ctx         = context.TODO()
		cleanupFunc func()
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())

		mockGardenClient = mockclient.NewMockClient(mockController)
		mockGardenInterface = mock.NewMockInterface(mockController)

		mockSeedClient = mockclient.NewMockClient(mockController)
		mockSeedInterface = mock.NewMockInterface(mockController)

		gardenletDeploymentConfiguration = seedmanagement.GardenletDeployment{
			// the repository and tag are required values in the gardenlet chart
			// use default values from the gardenlet helm chart
			Image: &seedmanagement.Image{
				Repository: pointer.String("eu.gcr.io/gardener-project/gardener/gardenlet"),
				Tag:        pointer.String("latest"),
			},
		}

		landscaper = Landscaper{
			gardenClient: mockGardenInterface,
			seedClient:   mockSeedInterface,
			log:          logger.NewNopLogger().WithContext(ctx),
			imports: &imports.Imports{
				ComponentConfiguration: gardenletConfigurationv1alpha1,
				// deployment configuration tested in applier tests
				DeploymentConfiguration: &gardenletDeploymentConfiguration,
			},
			gardenletConfiguration: gardenletConfigurationv1alpha1,
			chartPath:              chartsRootPath,
			rolloutSleepDuration:   0 * time.Second,
		}

		waiter := &retryfake.Ops{MaxAttempts: 1}
		cleanupFunc = test.WithVars(
			&retry.UntilTimeout, waiter.UntilTimeout,
		)
	})

	AfterEach(func() {
		mockController.Finish()
		cleanupFunc()
	})

	Describe("#Reconcile", func() {
		var (
			gardenletChartApplier          kubernetes.ChartApplier
			fakeGardenletChartClient       client.Client
			gardenletChartUniversalDecoder runtime.Decoder

			seedSecretName      = "seed-secret"
			seedSecretNamespace = "garden"
			seedSecret          = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      seedSecretName,
					Namespace: seedSecretNamespace,
				},
			}
			seedSecretRef = &corev1.SecretReference{
				Name:      seedSecretName,
				Namespace: seedSecretNamespace,
			}

			kubeconfigContentRuntimeCluster = []byte("very secure")

			backupSecretName      = "backup-secret"
			backupSecretNamespace = "garden"
			backupSecret          = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupSecretName,
					Namespace: backupSecretNamespace,
				},
			}
			backupSecretRef = corev1.SecretReference{
				Name:      backupSecretName,
				Namespace: backupSecretNamespace,
			}

			backupProviderName = "abc"
			backupCredentials  = map[string][]byte{
				"KEY": []byte("value"),
			}

			restConfig = &rest.Config{
				Host: "apiserver.dummy",
			}
			tokenID                  = utils.ComputeSHA256Hex([]byte(seed.Name))[:6]
			bootstrapTokenSecretName = bootstraptokenutil.BootstrapTokenSecretName(tokenID)
			timestampInTheFuture     = time.Now().UTC().Add(15 * time.Hour).Format(time.RFC3339)
		)

		// Before each ensures the chart appliers to have fake clients (cannot use mocks when applying charts)
		BeforeEach(func() {
			gardenletChartScheme := runtime.NewScheme()

			Expect(corev1.AddToScheme(gardenletChartScheme)).NotTo(HaveOccurred())
			Expect(appsv1.AddToScheme(gardenletChartScheme)).NotTo(HaveOccurred())
			Expect(gardenletconfig.AddToScheme(gardenletChartScheme)).NotTo(HaveOccurred())
			Expect(gardenletconfigv1alpha1.AddToScheme(gardenletChartScheme)).NotTo(HaveOccurred())
			Expect(schedulingv1.AddToScheme(gardenletChartScheme)).NotTo(HaveOccurred())
			Expect(rbacv1.AddToScheme(gardenletChartScheme)).NotTo(HaveOccurred())
			Expect(policyv1beta1.AddToScheme(gardenletChartScheme)).NotTo(HaveOccurred())

			codecs := serializer.NewCodecFactory(gardenletChartScheme)
			gardenletChartUniversalDecoder = codecs.UniversalDecoder()

			fakeGardenletChartClient = fake.NewClientBuilder().WithScheme(gardenletChartScheme).Build()

			gardenletMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion, appsv1.SchemeGroupVersion})
			gardenletMapper.Add(appsv1.SchemeGroupVersion.WithKind("Deployment"), meta.RESTScopeNamespace)
			gardenletMapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
			gardenletMapper.Add(schedulingv1.SchemeGroupVersion.WithKind("PriorityClass"), meta.RESTScopeRoot)
			gardenletMapper.Add(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"), meta.RESTScopeRoot)
			gardenletMapper.Add(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"), meta.RESTScopeRoot)

			// set git version as the chart helpers in the gardenlet determine resource API versions based on that
			gardenletChartRenderer := cr.NewWithServerVersion(&version.Info{
				GitVersion: "1.14.0",
			})

			gardenletChartApplier = kubernetes.NewChartApplier(gardenletChartRenderer, kubernetes.NewApplier(fakeGardenletChartClient, gardenletMapper))
		})

		// only test the happy case here, as there are more fine grained tests for each individual function
		DescribeTable("#Reconcile",
			func(
				useBootstrapKubeconfig bool,
				imageVectorOverride *string,
				componentImageVectorOverrides *string,
			) {
				// deploy seed secret
				gardenletConfigurationv1alpha1.SeedConfig.Spec = gardencorev1beta1.SeedSpec{SecretRef: seedSecretRef}
				landscaper.imports.ComponentConfiguration = gardenletConfigurationv1alpha1
				landscaper.imports.SeedCluster = landscaperv1alpha1.Target{
					Spec: landscaperv1alpha1.TargetSpec{
						Configuration: landscaperv1alpha1.AnyJSON{RawMessage: kubeconfigContentRuntimeCluster},
					},
				}

				if imageVectorOverride != nil {
					raw, err := json.Marshal(imageVectorOverride)
					Expect(err).ToNot(HaveOccurred())
					imageVectorOverride := json.RawMessage(raw)
					landscaper.imports.ImageVectorOverwrite = &imageVectorOverride
				}

				if componentImageVectorOverrides != nil {
					raw, err := json.Marshal(imageVectorOverride)
					Expect(err).ToNot(HaveOccurred())
					componentImageVectorOverrides := json.RawMessage(raw)
					landscaper.imports.ComponentImageVectorOverwrites = &componentImageVectorOverrides
				}

				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seedSecret.Namespace, seedSecret.Name), seedSecret).Return(nil)

				expectedSecret := *seedSecret
				expectedSecret.Data = map[string][]byte{
					"kubeconfig": kubeconfigContentRuntimeCluster,
				}
				expectedSecret.Type = corev1.SecretTypeOpaque
				mockGardenClient.EXPECT().Patch(ctx, &expectedSecret, gomock.Any()).Return(nil)

				// deployBackupSecret
				rawBackupCredentials, err := json.Marshal(backupCredentials)
				Expect(err).ToNot(HaveOccurred())
				message := json.RawMessage(rawBackupCredentials)
				landscaper.imports.SeedBackupCredentials = &message

				gardenletConfigurationv1alpha1.SeedConfig.Spec.Backup = &gardencorev1beta1.SeedBackup{
					Provider:  backupProviderName,
					SecretRef: backupSecretRef,
				}

				landscaper.imports.ComponentConfiguration = gardenletConfigurationv1alpha1

				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(backupSecret.Namespace, backupSecret.Name), backupSecret).Return(apierrors.NewNotFound(schema.GroupResource{}, backupSecret.Name))

				expectedBackupSecret := *backupSecret
				expectedBackupSecret.Data = backupCredentials
				expectedBackupSecret.Type = corev1.SecretTypeOpaque
				expectedBackupSecret.Labels = map[string]string{
					"provider": backupProviderName,
				}
				mockGardenClient.EXPECT().Create(ctx, &expectedBackupSecret).Return(nil)

				// isSeedBootstrapped
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				if useBootstrapKubeconfig {
					mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
						s.ObjectMeta = seed.ObjectMeta
						s.Status.KubernetesVersion = nil
						return nil
					})

					// getKubeconfigWithBootstrapToken - re-use existing bootstrap token
					mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
					mockGardenInterface.EXPECT().RESTConfig().Return(restConfig)
					mockGardenClient.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, bootstrapTokenSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret) error {
						s.Data = map[string][]byte{
							bootstraptokenapi.BootstrapTokenExpirationKey: []byte(timestampInTheFuture),
							bootstraptokenapi.BootstrapTokenIDKey:         []byte("dummy"),
							bootstraptokenapi.BootstrapTokenSecretKey:     []byte(bootstrapTokenSecretName),
						}
						return nil
					})
				} else {
					mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
						s.ObjectMeta = seed.ObjectMeta
						s.Generation = 1
						s.Status.ObservedGeneration = 1
						s.Status.Conditions = []gardencorev1beta1.Condition{
							{
								Type:   gardencorev1beta1.SeedGardenletReady,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   gardencorev1beta1.SeedBootstrapped,
								Status: gardencorev1beta1.ConditionTrue,
							},
						}
						return nil
					})
				}

				// Gardenlet chart for the Seed cluster
				mockSeedInterface.EXPECT().Client().Return(mockSeedClient)
				mockSeedClient.EXPECT().Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "garden",
					},
				})

				// required configuration that is validated when parsing the Gardenlet landscaper configuration
				// validated via the Seed validation
				// this test uses the default configuration from the helm chart
				gardenletConfigurationv1alpha1.Server = &gardenletconfigv1alpha1.ServerConfiguration{
					HTTPS: gardenletconfigv1alpha1.HTTPSServer{
						Server: gardenletconfigv1alpha1.Server{
							BindAddress: "0.0.0.0",
							Port:        2720,
						},
					},
				}

				mockSeedInterface.EXPECT().ChartApplier().Return(gardenletChartApplier)
				mockSeedInterface.EXPECT().Client().Return(mockSeedClient)

				mockSeedClient.EXPECT().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, "gardenlet"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, d *appsv1.Deployment) error {
					d.Generation = 1
					d.Status.ObservedGeneration = 1
					d.Status.Conditions = []appsv1.DeploymentCondition{
						{
							Type:   appsv1.DeploymentAvailable,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   appsv1.DeploymentReplicaFailure,
							Status: corev1.ConditionFalse,
						},
					}
					return nil
				})

				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)

				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
					s.ObjectMeta = seed.ObjectMeta
					s.Generation = 1
					s.Status.ObservedGeneration = 1
					s.Status.Conditions = []gardencorev1beta1.Condition{
						{
							Type:   gardencorev1beta1.SeedGardenletReady,
							Status: gardencorev1beta1.ConditionTrue,
						},
						{
							Type:   gardencorev1beta1.SeedBootstrapped,
							Status: gardencorev1beta1.ConditionTrue,
						},
					}
					return nil
				})

				// start reconciliation
				err = landscaper.Reconcile(ctx)
				Expect(err).ToNot(HaveOccurred())

				// verify resources applied via the Gardenlet chart
				expectedLabels := map[string]string{
					"app":      "gardener",
					"role":     "gardenlet",
					"chart":    "runtime-0.1.0",
					"release":  "gardenlet",
					"heritage": "Tiller",
				}

				appliercommon.ValidateGardenletChartPriorityClass(ctx, fakeGardenletChartClient)

				appliercommon.ValidateGardenletChartRBAC(
					ctx,
					fakeGardenletChartClient,
					expectedLabels,
					"gardenlet",
					nil)

				appliercommon.ValidateGardenletChartServiceAccount(ctx,
					fakeGardenletChartClient,
					false,
					expectedLabels,
					"gardenlet")

				expectedGardenletConfig := appliercommon.ComputeExpectedGardenletConfiguration(
					false,
					false,
					false,
					landscaper.gardenletConfiguration.GardenClientConnection.BootstrapKubeconfig,
					landscaper.gardenletConfiguration.GardenClientConnection.KubeconfigSecret,
					landscaper.gardenletConfiguration.SeedConfig,
					nil)

				appliercommon.VerifyGardenletComponentConfigConfigMap(ctx,
					fakeGardenletChartClient,
					gardenletChartUniversalDecoder,
					expectedGardenletConfig,
					expectedLabels)

				expectedGardenletDeploymentSpec := appliercommon.ComputeExpectedGardenletDeploymentSpec(
					&gardenletDeploymentConfiguration,
					false,
					nil,
					nil,
					expectedLabels,
					imageVectorOverride,
					componentImageVectorOverrides)

				appliercommon.VerifyGardenletDeployment(ctx,
					fakeGardenletChartClient,
					expectedGardenletDeploymentSpec,
					&gardenletDeploymentConfiguration,
					false,
					false,
					false,
					false,
					expectedLabels,
					imageVectorOverride,
					componentImageVectorOverrides)
			},
			Entry("should successfully reconcile with bootstrap kubeconfig", true, nil, nil),
			Entry("should successfully reconcile with bootstrap kubeconfig", false, nil, nil),
			Entry("should successfully reconcile with image vectors", false, pointer.String("abc"), pointer.String("dxy")),
		)

	})
	Describe("#waitForRolloutToBeComplete", func() {
		It("should fail because the Gardenlet deployment is not healthy (outdated Generation)", func() {
			mockSeedInterface.EXPECT().Client().Return(mockSeedClient)
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, "gardenlet"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, d *appsv1.Deployment) error {
				d.Generation = 2
				d.Status.ObservedGeneration = 1
				return nil
			})

			err := landscaper.waitForRolloutToBeComplete(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should fail because the Seed is not yet registered", func() {
			mockSeedInterface.EXPECT().Client().Return(mockSeedClient)
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, "gardenlet"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, d *appsv1.Deployment) error {
				d.Generation = 1
				d.Status.ObservedGeneration = 1
				d.Status.Conditions = []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   appsv1.DeploymentReplicaFailure,
						Status: corev1.ConditionFalse,
					},
				}
				return nil
			})

			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(apierrors.NewNotFound(schema.GroupResource{}, seed.Name))

			err := landscaper.waitForRolloutToBeComplete(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should fail because failed to get the Seed resource", func() {
			mockSeedInterface.EXPECT().Client().Return(mockSeedClient)
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, "gardenlet"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, d *appsv1.Deployment) error {
				d.Generation = 1
				d.Status.ObservedGeneration = 1
				d.Status.Conditions = []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   appsv1.DeploymentReplicaFailure,
						Status: corev1.ConditionFalse,
					},
				}
				return nil
			})

			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(fmt.Errorf("some error"))

			err := landscaper.waitForRolloutToBeComplete(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should fail because the Seed is unhealthy (not bootstrapped yet)", func() {
			mockSeedInterface.EXPECT().Client().Return(mockSeedClient)
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, "gardenlet"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, d *appsv1.Deployment) error {
				d.Generation = 1
				d.Status.ObservedGeneration = 1
				d.Status.Conditions = []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   appsv1.DeploymentReplicaFailure,
						Status: corev1.ConditionFalse,
					},
				}
				return nil
			})

			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)

			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
				s.ObjectMeta = seed.ObjectMeta
				s.Generation = 1
				s.Status.ObservedGeneration = 1
				s.Status.Conditions = []gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.SeedGardenletReady,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.SeedBootstrapped,
						Status: gardencorev1beta1.ConditionFalse,
					},
				}
				return nil
			})

			err := landscaper.waitForRolloutToBeComplete(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should succeed  because the Seed is registered, bootstrapped and ready", func() {
			mockSeedInterface.EXPECT().Client().Return(mockSeedClient)
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, "gardenlet"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, d *appsv1.Deployment) error {
				d.Generation = 1
				d.Status.ObservedGeneration = 1
				d.Status.Conditions = []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   appsv1.DeploymentReplicaFailure,
						Status: corev1.ConditionFalse,
					},
				}
				return nil
			})

			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)

			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
				s.ObjectMeta = seed.ObjectMeta
				s.Generation = 1
				s.Status.ObservedGeneration = 1
				s.Status.Conditions = []gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.SeedGardenletReady,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.SeedBootstrapped,
						Status: gardencorev1beta1.ConditionTrue,
					},
				}
				return nil
			})

			err := landscaper.waitForRolloutToBeComplete(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#deployBackupSecret", func() {
		secretName := "backup-secret"
		secretNamespace := "garden"
		var (
			backupSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: secretNamespace,
				},
			}
			backupSecretRef = corev1.SecretReference{
				Name:      secretName,
				Namespace: secretNamespace,
			}

			providerName = "abc"
			credentials  = map[string][]byte{
				"KEY": []byte("value"),
			}
		)

		It("should create the backup secret successfully", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(backupSecret.Namespace, backupSecret.Name), backupSecret).Return(apierrors.NewNotFound(schema.GroupResource{}, backupSecret.Name))

			expectedSecret := *backupSecret
			expectedSecret.Data = credentials
			expectedSecret.Type = corev1.SecretTypeOpaque
			expectedSecret.Labels = map[string]string{
				"provider": providerName,
			}
			mockGardenClient.EXPECT().Create(ctx, &expectedSecret).Return(nil)

			err := landscaper.deployBackupSecret(ctx, providerName, credentials, backupSecretRef)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should update the secret successfully", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(backupSecret.Namespace, backupSecret.Name), backupSecret).Return(nil)

			expectedSecret := *backupSecret
			expectedSecret.Data = credentials
			expectedSecret.Type = corev1.SecretTypeOpaque
			expectedSecret.Labels = map[string]string{
				"provider": providerName,
			}
			mockGardenClient.EXPECT().Patch(ctx, &expectedSecret, gomock.Any()).Return(nil)

			err := landscaper.deployBackupSecret(ctx, providerName, credentials, backupSecretRef)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error when failing to deploy the secret", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(backupSecret.Namespace, backupSecret.Name), backupSecret).Return(nil)
			mockGardenClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("some error"))

			err := landscaper.deployBackupSecret(ctx, providerName, credentials, backupSecretRef)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#deploySeedSecret", func() {
		secretName := "seed-secret"
		secretNamespace := "garden"
		var (
			seedSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: secretNamespace,
				},
			}
			secretRef = &corev1.SecretReference{
				Name:      secretName,
				Namespace: secretNamespace,
			}

			kubeconfigContent = []byte("very secure")
		)

		It("should create the secret successfully", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seedSecret.Namespace, seedSecret.Name), seedSecret).Return(apierrors.NewNotFound(schema.GroupResource{}, seedSecret.Name))

			expectedSecret := *seedSecret
			expectedSecret.Data = map[string][]byte{
				"kubeconfig": kubeconfigContent,
			}
			expectedSecret.Type = corev1.SecretTypeOpaque
			mockGardenClient.EXPECT().Create(ctx, &expectedSecret).Return(nil)

			err := landscaper.deploySeedSecret(ctx, kubeconfigContent, secretRef)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should update the secret successfully", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seedSecret.Namespace, seedSecret.Name), seedSecret).Return(nil)

			expectedSecret := *seedSecret
			expectedSecret.Data = map[string][]byte{
				"kubeconfig": kubeconfigContent,
			}
			expectedSecret.Type = corev1.SecretTypeOpaque
			mockGardenClient.EXPECT().Patch(ctx, &expectedSecret, gomock.Any()).Return(nil)

			err := landscaper.deploySeedSecret(ctx, kubeconfigContent, secretRef)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error when failing to deploy the secret", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seedSecret.Namespace, seedSecret.Name), seedSecret).Return(nil)
			mockGardenClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("some error"))

			err := landscaper.deploySeedSecret(ctx, kubeconfigContent, secretRef)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#isSeedBootstrapped", func() {
		It("the requested seed is bootstrapped", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
				s.ObjectMeta = seed.ObjectMeta
				s.Generation = 1
				s.Status.ObservedGeneration = 1
				s.Status.Conditions = []gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.SeedGardenletReady,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.SeedBootstrapped,
						Status: gardencorev1beta1.ConditionTrue,
					},
				}
				return nil
			})

			exists, err := landscaper.isSeedBootstrapped(ctx, seed.ObjectMeta)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(Equal(true))
		})

		It("the requested seed does NOT exist", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(apierrors.NewNotFound(schema.GroupResource{}, seed.Name))

			exists, err := landscaper.isSeedBootstrapped(ctx, seed.ObjectMeta)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(Equal(false))
		})

		It("the requested seed's status does not indicate proper health", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
				s.ObjectMeta = seed.ObjectMeta
				s.Status.KubernetesVersion = nil
				s.Generation = 2
				s.Status.ObservedGeneration = 1
				s.Status.Conditions = []gardencorev1beta1.Condition{}
				return nil
			})

			exists, err := landscaper.isSeedBootstrapped(ctx, seed.ObjectMeta)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(Equal(false))
		})

		It("expecting an error", func() {
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
			mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(fmt.Errorf("fake error"))

			exists, err := landscaper.isSeedBootstrapped(ctx, seed.ObjectMeta)
			Expect(err).To(HaveOccurred())
			Expect(exists).To(Equal(false))
		})
	})
})
