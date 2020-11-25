// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package terraformer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	configMapGroupResource = schema.GroupResource{Resource: "ConfigMaps"}
	secretGroupResource    = schema.GroupResource{Resource: "Secrets"}
)

var _ = Describe("terraformer", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		ctx  context.Context

		log logr.Logger
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		ctx = context.Background()

		log = logzap.New(logzap.WriteTo(GinkgoWriter))
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#CreateOrUpdateConfigurationConfigMap", func() {
		It("Should create the config map", func() {
			const (
				namespace = "namespace"
				name      = "name"

				main      = "main"
				variables = "variables"
			)

			var (
				objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name}
				expected   = &corev1.ConfigMap{
					ObjectMeta: objectMeta,
					Data: map[string]string{
						MainKey:      main,
						VariablesKey: variables,
					},
				}
			)

			gomock.InOrder(
				c.EXPECT().
					Get(gomock.Any(), kutil.Key(namespace, name), &corev1.ConfigMap{ObjectMeta: objectMeta}).
					Return(apierrors.NewNotFound(configMapGroupResource, name)),
				c.EXPECT().
					Create(gomock.Any(), expected.DeepCopy()),
			)

			actual, err := CreateOrUpdateConfigurationConfigMap(ctx, c, namespace, name, main, variables)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(expected))
		})
	})

	Describe("#StateConfigMapInitializer", func() {
		const (
			namespace = "namespace"
			name      = "name"
		)

		Describe("#CreateState", func() {
			var (
				expected                  *corev1.ConfigMap
				stateConfigMapInitializer StateConfigMapInitializerFunc
			)

			BeforeEach(func() {
				expected = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
					Data: map[string]string{
						StateKey: "",
					},
				}
				stateConfigMapInitializer = CreateState
			})

			It("should create the ConfigMap", func() {
				c.EXPECT().Create(gomock.Any(), expected.DeepCopy())

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil when the ConfigMap already exists", func() {
				c.EXPECT().
					Create(gomock.Any(), expected.DeepCopy()).
					Return(apierrors.NewAlreadyExists(configMapGroupResource, name))

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return error when the ConfigMap creation fails", func() {
				c.EXPECT().
					Create(gomock.Any(), expected.DeepCopy()).
					Return(apierrors.NewForbidden(configMapGroupResource, name, fmt.Errorf("not allowed to create ConfigMap")))

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})

		Describe("#CreateOrUpdateState", func() {
			It("Should create the ConfigMap", func() {
				var (
					state      = "state"
					stateKey   = kutil.Key(namespace, name)
					objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name}
					getState   = &corev1.ConfigMap{ObjectMeta: objectMeta}
					expected   = &corev1.ConfigMap{
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

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("#CreateOrUpdateTFVarsSecret", func() {
		It("Should create the secret", func() {
			const (
				namespace = "namespace"
				name      = "name"
			)

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
					Get(gomock.Any(), kutil.Key(namespace, name), &corev1.Secret{ObjectMeta: objectMeta}).
					Return(apierrors.NewNotFound(secretGroupResource, name)),
				c.EXPECT().
					Create(gomock.Any(), expected.DeepCopy()),
			)

			actual, err := CreateOrUpdateTFVarsSecret(ctx, c, namespace, name, tfVars)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(expected))
		})
	})

	Describe("#Initializers", func() {
		const (
			namespace         = "namespace"
			configurationName = "configuration"
			variablesName     = "variables"
			stateName         = "state"

			main      = "main"
			variables = "variables"
		)
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

			configurationKey = kutil.Key(namespace, configurationName)
			variablesKey = kutil.Key(namespace, variablesName)
			stateKey = kutil.Key(namespace, stateName)

			configurationObjectMeta = kutil.ObjectMeta(namespace, configurationName)
			variablesObjectMeta = kutil.ObjectMeta(namespace, variablesName)
			stateObjectMeta = kutil.ObjectMeta(namespace, stateName)

			getConfiguration = &corev1.ConfigMap{ObjectMeta: configurationObjectMeta}
			getVariables = &corev1.Secret{ObjectMeta: variablesObjectMeta}
			getState = &corev1.ConfigMap{ObjectMeta: stateObjectMeta}

			createConfiguration = &corev1.ConfigMap{
				ObjectMeta: configurationObjectMeta,
				Data: map[string]string{
					MainKey:      main,
					VariablesKey: variables,
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
						return DefaultInitializer(c, main, variables, tfVars, StateConfigMapInitializerFunc(CreateState)).Initialize(ctx, &InitializerConfig{
							Namespace:         namespace,
							ConfigurationName: configurationName,
							VariablesName:     variablesName,
							StateName:         stateName,
							InitializeState:   initializeState,
						})
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
						return DefaultInitializer(c, main, variables, tfVars, &CreateOrUpdateState{State: &state}).Initialize(ctx, &InitializerConfig{
							Namespace:         namespace,
							ConfigurationName: configurationName,
							VariablesName:     variablesName,
							StateName:         stateName,
							InitializeState:   initializeState,
						})
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
			tf := New(log, c, nil, "purpose", "namespace", "name", "image")

			err := tf.Apply(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#GetStateOutputVariables", func() {
		const (
			namespace = "namespace"
			name      = "name"
			purpose   = "purpose"
			image     = "image"
		)

		var (
			stateName = fmt.Sprintf("%s.%s.tf-state", name, purpose)
			stateKey  = kutil.Key(namespace, stateName)
		)

		It("should return err when state version is not supported", func() {
			state := map[string]interface{}{
				"version": 1,
			}
			stateJSON, err := json.Marshal(state)
			Expect(err).NotTo(HaveOccurred())

			c.EXPECT().
				Get(gomock.Any(), stateKey, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap) error {
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
			state := map[string]interface{}{
				"version": 3,
				"modules": []map[string]interface{}{
					{
						"outputs": map[string]interface{}{
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
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap) error {
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
			state := map[string]interface{}{
				"version": 4,
				"outputs": map[string]interface{}{
					"variableV4": map[string]string{
						"value": "valueV4",
					},
				},
			}
			stateJSON, err := json.Marshal(state)
			Expect(err).NotTo(HaveOccurred())

			c.EXPECT().
				Get(gomock.Any(), stateKey, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap) error {
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
})
