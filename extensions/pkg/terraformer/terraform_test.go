// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var (
	configMapGroupResource = schema.GroupResource{Resource: "ConfigMaps"}
	secretGroupResource    = schema.GroupResource{Resource: "Secrets"}
)

const (
	namespace         = "namespace"
	name              = "name"
	mainName          = "main"
	configurationName = "configuration"
	variablesName     = "variables"
	stateName         = "state"
	infraUID          = "2a540a5c-1b8c-11e8-b291-0a580a2c025a"
	purpose           = "purpose"
	image             = "image"
)

var _ = Describe("terraformer", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		ctx  context.Context
		log  logr.Logger

		infra = extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
				UID:       infraUID,
			},
		}
		ownerRef = metav1.NewControllerRef(&infra.ObjectMeta, extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.InfrastructureResource))
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		ctx = context.Background()

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#CreateOrUpdateConfigurationConfigMap", func() {
		It("Should create the config map without owner reference", func() {
			var (
				objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name}
				expected   = &corev1.ConfigMap{
					ObjectMeta: objectMeta,
					Data: map[string]string{
						MainKey:      mainName,
						VariablesKey: variablesName,
					},
				}
			)

			gomock.InOrder(
				c.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, &corev1.ConfigMap{ObjectMeta: objectMeta}).
					Return(apierrors.NewNotFound(configMapGroupResource, name)),
				c.EXPECT().
					Create(gomock.Any(), expected.DeepCopy()),
			)

			actual, err := CreateOrUpdateConfigurationConfigMap(ctx, c, namespace, name, mainName, variablesName, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(expected))
		})

		It("Should create the config map with owner reference", func() {
			var (
				objectMeta = metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				}
				objectMetaWithOwnerRef = metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
					OwnerReferences: []metav1.OwnerReference{
						*ownerRef,
					},
				}
				expected = &corev1.ConfigMap{
					ObjectMeta: objectMetaWithOwnerRef,
					Data: map[string]string{
						MainKey:      mainName,
						VariablesKey: variablesName,
					},
				}
			)

			gomock.InOrder(
				c.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, &corev1.ConfigMap{ObjectMeta: objectMeta}).
					Return(apierrors.NewNotFound(configMapGroupResource, name)),
				c.EXPECT().
					Create(gomock.Any(), expected.DeepCopy()),
			)

			actual, err := CreateOrUpdateConfigurationConfigMap(ctx, c, namespace, name, mainName, variablesName, ownerRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(expected))
		})
	})

	Describe("#StateConfigMapInitializer", func() {

		Describe("#CreateState", func() {
			var (
				stateConfigMap            *corev1.ConfigMap
				expected                  *corev1.ConfigMap
				stateConfigMapInitializer StateConfigMapInitializerFunc
			)

			BeforeEach(func() {
				stateConfigMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Data: map[string]string{
						StateKey: "",
					},
				}
				expected = stateConfigMap.DeepCopy()
				expected.OwnerReferences = []metav1.OwnerReference{
					*ownerRef,
				}
				stateConfigMapInitializer = CreateState
			})

			It("should create the ConfigMap", func() {
				c.EXPECT().Create(gomock.Any(), expected.DeepCopy())

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name, ownerRef)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil when the ConfigMap already exists", func() {
				c.EXPECT().
					Create(gomock.Any(), stateConfigMap.DeepCopy()).
					Return(apierrors.NewAlreadyExists(configMapGroupResource, name))

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return error when the ConfigMap creation fails", func() {
				c.EXPECT().
					Create(gomock.Any(), expected.DeepCopy()).
					Return(apierrors.NewForbidden(configMapGroupResource, name, errors.New("not allowed to create ConfigMap")))

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name, ownerRef)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})

		Describe("#CreateOrUpdateState", func() {
			It("Should create the ConfigMap", func() {
				var (
					state      = "state"
					stateKey   = client.ObjectKey{Namespace: namespace, Name: name}
					objectMeta = metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
						OwnerReferences: []metav1.OwnerReference{
							*ownerRef,
						},
					}
					getState = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      name,
						},
					}
					expected = &corev1.ConfigMap{
						ObjectMeta: objectMeta,
						Data: map[string]string{
							StateKey: state,
						},
					}
					stateConfigMapInitializer = &CreateOrUpdateState{State: &state}
					stateNotFound             = apierrors.NewNotFound(configMapGroupResource, name)
				)
				gomock.InOrder(
					c.EXPECT().
						Get(gomock.Any(), stateKey, getState.DeepCopy()).
						Return(stateNotFound),
					c.EXPECT().Create(gomock.Any(), expected.DeepCopy()),
				)

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name, ownerRef)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("#CreateOrUpdateTFVarsSecret", func() {
		It("Should create the secret without owner reference", func() {

			var (
				tfVars     = []byte("tfvars")
				objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name}
				expected   = &corev1.Secret{
					ObjectMeta: objectMeta,
					Data: map[string][]byte{
						TFVarsKey: tfVars,
					},
				}
			)

			gomock.InOrder(
				c.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{ObjectMeta: objectMeta}).
					Return(apierrors.NewNotFound(secretGroupResource, name)),
				c.EXPECT().
					Create(gomock.Any(), expected.DeepCopy()),
			)

			actual, err := CreateOrUpdateTFVarsSecret(ctx, c, namespace, name, tfVars, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(expected))
		})

		It("Should create the secret with owner reference", func() {
			var (
				tfVars     = []byte("tfvars")
				objectMeta = metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				}
				objectMetaWithOwnerRef = metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
					OwnerReferences: []metav1.OwnerReference{
						*ownerRef,
					},
				}
				expected = &corev1.Secret{
					ObjectMeta: objectMetaWithOwnerRef,
					Data: map[string][]byte{
						TFVarsKey: tfVars,
					},
				}
			)

			gomock.InOrder(
				c.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{ObjectMeta: objectMeta}).
					Return(apierrors.NewNotFound(secretGroupResource, name)),
				c.EXPECT().
					Create(gomock.Any(), expected.DeepCopy()),
			)

			actual, err := CreateOrUpdateTFVarsSecret(ctx, c, namespace, name, tfVars, ownerRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(expected))
		})
	})

	Describe("#Initializers", func() {
		var (
			tfVars []byte

			configurationKey client.ObjectKey
			variablesKey     client.ObjectKey
			stateKey         client.ObjectKey

			configurationObjectMeta metav1.ObjectMeta
			variablesObjectMeta     metav1.ObjectMeta
			stateObjectMeta         metav1.ObjectMeta

			getConfiguration *corev1.ConfigMap
			getVariables     *corev1.Secret
			getState         *corev1.ConfigMap

			createConfiguration *corev1.ConfigMap
			createVariables     *corev1.Secret
		)

		BeforeEach(func() {
			tfVars = []byte("tfvars")

			configurationKey = client.ObjectKey{Namespace: namespace, Name: configurationName}
			variablesKey = client.ObjectKey{Namespace: namespace, Name: variablesName}
			stateKey = client.ObjectKey{Namespace: namespace, Name: stateName}

			configurationObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: configurationName}
			variablesObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: variablesName}
			stateObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: stateName}

			getConfiguration = &corev1.ConfigMap{ObjectMeta: configurationObjectMeta}
			getVariables = &corev1.Secret{ObjectMeta: variablesObjectMeta}
			getState = &corev1.ConfigMap{ObjectMeta: stateObjectMeta}

			createConfiguration = &corev1.ConfigMap{
				ObjectMeta: configurationObjectMeta,
				Data: map[string]string{
					MainKey:      mainName,
					VariablesKey: variablesName,
				},
			}
			createVariables = &corev1.Secret{
				ObjectMeta: variablesObjectMeta,
				Data: map[string][]byte{
					TFVarsKey: tfVars,
				},
			}
		})

		Describe("#DefaultInitializer", func() {
			var (
				state                                                   string
				createState                                             *corev1.ConfigMap
				configurationNotFound, variablesNotFound, stateNotFound *apierrors.StatusError
				runInitializer                                          func(ctx context.Context, initializeState bool) error
			)

			Context("When there is no init state", func() {
				BeforeEach(func() {
					state = ""
					createState = &corev1.ConfigMap{
						ObjectMeta: stateObjectMeta,
						Data: map[string]string{
							StateKey: state,
						},
					}
					configurationNotFound = apierrors.NewNotFound(configMapGroupResource, configurationName)
					variablesNotFound = apierrors.NewNotFound(secretGroupResource, variablesName)

					runInitializer = func(ctx context.Context, initializeState bool) error {
						return DefaultInitializer(c, mainName, variablesName, tfVars, StateConfigMapInitializerFunc(CreateState)).Initialize(
							ctx,
							&InitializerConfig{
								Namespace:         namespace,
								ConfigurationName: configurationName,
								VariablesName:     variablesName,
								StateName:         stateName,
								InitializeState:   initializeState,
							},
							nil,
						)
					}
				})

				It("should not initialize state when initializeState is false", func() {
					gomock.InOrder(
						c.EXPECT().
							Get(gomock.Any(), configurationKey, getConfiguration.DeepCopy()).
							Return(configurationNotFound),
						c.EXPECT().
							Create(gomock.Any(), createConfiguration.DeepCopy()),

						c.EXPECT().
							Get(gomock.Any(), variablesKey, getVariables.DeepCopy()).
							Return(variablesNotFound),
						c.EXPECT().
							Create(gomock.Any(), createVariables.DeepCopy()),
					)

					Expect(runInitializer(ctx, false)).NotTo(HaveOccurred())
				})
			})

			Context("When there is init state", func() {
				BeforeEach(func() {
					state = "{\"data\":\"big data\"}"
					createState = &corev1.ConfigMap{
						ObjectMeta: stateObjectMeta,
						Data: map[string]string{
							StateKey: state,
						},
					}
					configurationNotFound = apierrors.NewNotFound(configMapGroupResource, configurationName)
					variablesNotFound = apierrors.NewNotFound(secretGroupResource, variablesName)
					stateNotFound = apierrors.NewNotFound(configMapGroupResource, stateName)

					runInitializer = func(ctx context.Context, initializeState bool) error {
						return DefaultInitializer(c, mainName, variablesName, tfVars, &CreateOrUpdateState{State: &state}).Initialize(
							ctx,
							&InitializerConfig{
								Namespace:         namespace,
								ConfigurationName: configurationName,
								VariablesName:     variablesName,
								StateName:         stateName,
								InitializeState:   initializeState,
							},
							nil,
						)
					}
				})

				It("should create all resources", func() {
					gomock.InOrder(
						c.EXPECT().
							Get(gomock.Any(), configurationKey, getConfiguration.DeepCopy()).
							Return(configurationNotFound),
						c.EXPECT().
							Create(gomock.Any(), createConfiguration.DeepCopy()),

						c.EXPECT().
							Get(gomock.Any(), variablesKey, getVariables.DeepCopy()).
							Return(variablesNotFound),
						c.EXPECT().
							Create(gomock.Any(), createVariables.DeepCopy()),

						c.EXPECT().
							Get(gomock.Any(), stateKey, getState.DeepCopy()).
							Return(stateNotFound),
						c.EXPECT().
							Create(gomock.Any(), createState.DeepCopy()),
					)

					Expect(runInitializer(ctx, true)).NotTo(HaveOccurred())
				})

				It("should not initialize state when initializeState is false", func() {
					gomock.InOrder(
						c.EXPECT().
							Get(gomock.Any(), configurationKey, getConfiguration.DeepCopy()).
							Return(configurationNotFound),
						c.EXPECT().
							Create(gomock.Any(), createConfiguration.DeepCopy()),

						c.EXPECT().
							Get(gomock.Any(), variablesKey, getVariables.DeepCopy()).
							Return(variablesNotFound),
						c.EXPECT().
							Create(gomock.Any(), createVariables.DeepCopy()),
					)

					Expect(runInitializer(ctx, false)).NotTo(HaveOccurred())
				})
			})
		})
	})

	Describe("#Apply", func() {
		It("should return err when config is not defined", func() {
			tf := New(log, c, nil, purpose, namespace, name, image)

			err := tf.Apply(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#GetStateOutputVariables", func() {
		var (
			stateName = fmt.Sprintf("%s.%s.tf-state", name, purpose)
			stateKey  = client.ObjectKey{Namespace: namespace, Name: stateName}
		)

		It("should return err when state version is not supported", func() {
			state := map[string]any{
				"version": 1,
			}
			stateJSON, err := json.Marshal(state)
			Expect(err).NotTo(HaveOccurred())

			c.EXPECT().
				Get(gomock.Any(), stateKey, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					cm.Data = map[string]string{
						StateKey: string(stateJSON),
					}
					return nil
				})

			terraformer := New(log, c, nil, purpose, namespace, name, image)
			actual, err := terraformer.GetStateOutputVariables(ctx, "variableV1")

			Expect(actual).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should get state v3 output variables", func() {
			state := map[string]any{
				"version": 3,
				"modules": []map[string]any{
					{
						"outputs": map[string]any{
							"variableV3": map[string]string{
								"value": "valueV3",
							},
						},
					},
				},
			}
			stateJSON, err := json.Marshal(state)
			Expect(err).NotTo(HaveOccurred())

			c.EXPECT().
				Get(gomock.Any(), stateKey, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					cm.Data = map[string]string{
						StateKey: string(stateJSON),
					}
					return nil
				})

			terraformer := New(log, c, nil, purpose, namespace, name, image)
			actual, err := terraformer.GetStateOutputVariables(ctx, "variableV3")

			expected := map[string]string{
				"variableV3": "valueV3",
			}
			Expect(actual).To(Equal(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should get state v4 output variables", func() {
			state := map[string]any{
				"version": 4,
				"outputs": map[string]any{
					"variableV4": map[string]string{
						"value": "valueV4",
					},
				},
			}
			stateJSON, err := json.Marshal(state)
			Expect(err).NotTo(HaveOccurred())

			c.EXPECT().
				Get(gomock.Any(), stateKey, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					cm.Data = map[string]string{
						StateKey: string(stateJSON),
					}
					return nil
				})

			terraformer := New(log, c, nil, purpose, namespace, name, image)
			actual, err := terraformer.GetStateOutputVariables(ctx, "variableV4")

			expected := map[string]string{
				"variableV4": "valueV4",
			}
			Expect(actual).To(Equal(expected))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Cleanup", func() {
		var (
			prefix = fmt.Sprintf("%s.%s", name, purpose)

			configName    = prefix + ConfigSuffix
			variablesName = prefix + VariablesSuffix
			stateName     = prefix + StateSuffix

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: variablesName},
			}
			config = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: configName},
			}
			state = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: stateName},
			}
		)

		It("should delete all resources", func() {
			t := New(log, c, nil, purpose, namespace, name, image)

			gomock.InOrder(
				c.EXPECT().
					Delete(gomock.Any(), state.DeepCopy()),
				c.EXPECT().
					Delete(gomock.Any(), secret.DeepCopy()),
				c.EXPECT().
					Delete(gomock.Any(), config.DeepCopy()),
			)

			Expect(t.CleanupConfiguration(ctx)).NotTo(HaveOccurred())
		})

		It("should remove the terraform finalizer from all resources", func() {
			t := New(log, c, nil, purpose, namespace, name, image)

			gomock.InOrder(
				c.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: variablesName}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
						s.SetFinalizers([]string{TerraformerFinalizer})
						return nil
					}),
				c.EXPECT().
					Patch(gomock.Any(), gomock.AssignableToTypeOf(secret.DeepCopy()), gomock.AssignableToTypeOf(client.MergeFromWithOptions(secret.DeepCopy(), client.MergeFromWithOptimisticLock{}))),

				c.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: stateName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, configMap *corev1.ConfigMap, _ ...client.GetOption) error {
						configMap.SetFinalizers([]string{TerraformerFinalizer})
						return nil
					}),
				c.EXPECT().
					Patch(gomock.Any(), gomock.AssignableToTypeOf(config.DeepCopy()), gomock.AssignableToTypeOf(client.MergeFromWithOptions(config.DeepCopy(), client.MergeFromWithOptimisticLock{}))),

				c.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: configName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, configMap *corev1.ConfigMap, _ ...client.GetOption) error {
						configMap.SetFinalizers([]string{TerraformerFinalizer})
						return nil
					}),
				c.EXPECT().
					Patch(gomock.Any(), gomock.AssignableToTypeOf(state.DeepCopy()), gomock.AssignableToTypeOf(client.MergeFromWithOptions(state.DeepCopy(), client.MergeFromWithOptimisticLock{}))),
			)

			Expect(t.RemoveTerraformerFinalizerFromConfig(ctx)).NotTo(HaveOccurred())
		})
	})
})
