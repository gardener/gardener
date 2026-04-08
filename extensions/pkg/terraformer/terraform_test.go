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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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
		c      client.Client
		ctx    context.Context
		log    logr.Logger
		scheme *runtime.Scheme

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
		ctx = context.Background()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		c = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
	})

	Describe("#CreateOrUpdateConfigurationConfigMap", func() {
		It("Should create the config map without owner reference", func() {
			actual, err := CreateOrUpdateConfigurationConfigMap(ctx, c, namespace, name, mainName, variablesName, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            name,
					ResourceVersion: "1",
				},
				Data: map[string]string{
					MainKey:      mainName,
					VariablesKey: variablesName,
				},
			}
			Expect(actual).To(Equal(expected))

			// Verify state: ConfigMap exists in the fake client
			got := &corev1.ConfigMap{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, got)).To(Succeed())
			Expect(got.Data).To(Equal(expected.Data))
		})

		It("Should create the config map with owner reference", func() {
			actual, err := CreateOrUpdateConfigurationConfigMap(ctx, c, namespace, name, mainName, variablesName, ownerRef)
			Expect(err).NotTo(HaveOccurred())

			expected := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            name,
					ResourceVersion: "1",
					OwnerReferences: []metav1.OwnerReference{
						*ownerRef,
					},
				},
				Data: map[string]string{
					MainKey:      mainName,
					VariablesKey: variablesName,
				},
			}
			Expect(actual).To(Equal(expected))

			// Verify state: ConfigMap exists with owner reference
			got := &corev1.ConfigMap{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, got)).To(Succeed())
			Expect(got.OwnerReferences).To(ConsistOf(*ownerRef))
		})
	})

	Describe("#StateConfigMapInitializer", func() {
		Describe("#CreateState", func() {
			var (
				stateConfigMapInitializer StateConfigMapInitializerFunc
			)

			BeforeEach(func() {
				stateConfigMapInitializer = CreateState
			})

			It("should create the ConfigMap", func() {
				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name, ownerRef)
				Expect(err).NotTo(HaveOccurred())

				// Verify state: ConfigMap was created with owner ref and empty state
				got := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, got)).To(Succeed())
				Expect(got.Data).To(Equal(map[string]string{StateKey: ""}))
				Expect(got.OwnerReferences).To(ConsistOf(*ownerRef))
			})

			It("should return nil when the ConfigMap already exists", func() {
				// Pre-create the ConfigMap
				existing := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Data: map[string]string{
						StateKey: "existing-state",
					},
				}
				Expect(c.Create(ctx, existing)).To(Succeed())

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name, nil)
				Expect(err).NotTo(HaveOccurred())

				// Verify original state was preserved (not overwritten)
				got := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, got)).To(Succeed())
				Expect(got.Data[StateKey]).To(Equal("existing-state"))
			})

			It("should return error when the ConfigMap creation fails", func() {
				// Use interceptor to simulate a forbidden error on Create
				fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
					Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
						return apierrors.NewForbidden(corev1.Resource("configmaps"), name, errors.New("not allowed to create ConfigMap"))
					},
				}).Build()

				err := stateConfigMapInitializer.Initialize(ctx, fakeClient, namespace, name, ownerRef)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})

		Describe("#CreateOrUpdateState", func() {
			It("Should create the ConfigMap", func() {
				state := "state"
				stateConfigMapInitializer := &CreateOrUpdateState{State: &state}

				err := stateConfigMapInitializer.Initialize(ctx, c, namespace, name, ownerRef)
				Expect(err).NotTo(HaveOccurred())

				// Verify state: ConfigMap was created with the state data
				got := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, got)).To(Succeed())
				Expect(got.Data).To(Equal(map[string]string{StateKey: state}))
				Expect(got.OwnerReferences).To(ConsistOf(*ownerRef))
			})
		})
	})

	Describe("#CreateOrUpdateTFVarsSecret", func() {
		It("Should create the secret without owner reference", func() {
			tfVars := []byte("tfvars")

			actual, err := CreateOrUpdateTFVarsSecret(ctx, c, namespace, name, tfVars, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            name,
					ResourceVersion: "1",
				},
				Data: map[string][]byte{
					TFVarsKey: tfVars,
				},
			}
			Expect(actual).To(Equal(expected))

			// Verify state: Secret exists in the fake client
			got := &corev1.Secret{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, got)).To(Succeed())
			Expect(got.Data).To(Equal(expected.Data))
		})

		It("Should create the secret with owner reference", func() {
			tfVars := []byte("tfvars")

			actual, err := CreateOrUpdateTFVarsSecret(ctx, c, namespace, name, tfVars, ownerRef)
			Expect(err).NotTo(HaveOccurred())

			expected := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            name,
					ResourceVersion: "1",
					OwnerReferences: []metav1.OwnerReference{
						*ownerRef,
					},
				},
				Data: map[string][]byte{
					TFVarsKey: tfVars,
				},
			}
			Expect(actual).To(Equal(expected))

			// Verify state: Secret exists with owner reference
			got := &corev1.Secret{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, got)).To(Succeed())
			Expect(got.OwnerReferences).To(ConsistOf(*ownerRef))
		})
	})

	Describe("#Initializers", func() {
		var (
			tfVars []byte
		)

		BeforeEach(func() {
			tfVars = []byte("tfvars")
		})

		Describe("#DefaultInitializer", func() {
			var (
				runInitializer func(ctx context.Context, initializeState bool) error
			)

			Context("When there is no init state", func() {
				BeforeEach(func() {
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
					Expect(runInitializer(ctx, false)).NotTo(HaveOccurred())

					// Verify configuration ConfigMap was created
					configCM := &corev1.ConfigMap{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: configurationName}, configCM)).To(Succeed())
					Expect(configCM.Data).To(Equal(map[string]string{
						MainKey:      mainName,
						VariablesKey: variablesName,
					}))

					// Verify variables Secret was created
					varSecret := &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: variablesName}, varSecret)).To(Succeed())
					Expect(varSecret.Data).To(Equal(map[string][]byte{
						TFVarsKey: tfVars,
					}))

					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: stateName}, &corev1.ConfigMap{})).To(BeNotFoundError())
				})
			})

			Context("When there is init state", func() {
				var state string

				BeforeEach(func() {
					state = "{\"data\":\"big data\"}"

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
					Expect(runInitializer(ctx, true)).NotTo(HaveOccurred())

					// Verify configuration ConfigMap was created
					configCM := &corev1.ConfigMap{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: configurationName}, configCM)).To(Succeed())
					Expect(configCM.Data).To(Equal(map[string]string{
						MainKey:      mainName,
						VariablesKey: variablesName,
					}))

					// Verify variables Secret was created
					varSecret := &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: variablesName}, varSecret)).To(Succeed())
					Expect(varSecret.Data).To(Equal(map[string][]byte{
						TFVarsKey: tfVars,
					}))

					// Verify state ConfigMap was created with the state data
					stateCM := &corev1.ConfigMap{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: stateName}, stateCM)).To(Succeed())
					Expect(stateCM.Data).To(Equal(map[string]string{
						StateKey: state,
					}))
				})

				It("should not initialize state when initializeState is false", func() {
					Expect(runInitializer(ctx, false)).NotTo(HaveOccurred())

					// Verify configuration ConfigMap was created
					configCM := &corev1.ConfigMap{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: configurationName}, configCM)).To(Succeed())
					Expect(configCM.Data).To(Equal(map[string]string{
						MainKey:      mainName,
						VariablesKey: variablesName,
					}))

					// Verify variables Secret was created
					varSecret := &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: variablesName}, varSecret)).To(Succeed())
					Expect(varSecret.Data).To(Equal(map[string][]byte{
						TFVarsKey: tfVars,
					}))

					// Verify state ConfigMap was NOT created
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: stateName}, &corev1.ConfigMap{})).To(BeNotFoundError())
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
			tfStateName = fmt.Sprintf("%s.%s.tf-state", name, purpose)
		)

		It("should return err when state version is not supported", func() {
			state := map[string]any{
				"version": 1,
			}
			stateJSON, err := json.Marshal(state)
			Expect(err).NotTo(HaveOccurred())

			// Create the state ConfigMap
			stateCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      tfStateName,
				},
				Data: map[string]string{
					StateKey: string(stateJSON),
				},
			}
			Expect(c.Create(ctx, stateCM)).To(Succeed())

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

			// Create the state ConfigMap
			stateCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      tfStateName,
				},
				Data: map[string]string{
					StateKey: string(stateJSON),
				},
			}
			Expect(c.Create(ctx, stateCM)).To(Succeed())

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

			// Create the state ConfigMap
			stateCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      tfStateName,
				},
				Data: map[string]string{
					StateKey: string(stateJSON),
				},
			}
			Expect(c.Create(ctx, stateCM)).To(Succeed())

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

			tfConfigName    = prefix + ConfigSuffix
			tfVariablesName = prefix + VariablesSuffix
			tfStateName     = prefix + StateSuffix
		)

		It("should delete all resources", func() {
			// Create the resources to be deleted
			Expect(c.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: tfStateName},
			})).To(Succeed())
			Expect(c.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: tfVariablesName},
			})).To(Succeed())
			Expect(c.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: tfConfigName},
			})).To(Succeed())

			t := New(log, c, nil, purpose, namespace, name, image)
			Expect(t.CleanupConfiguration(ctx)).NotTo(HaveOccurred())

			// Verify all resources are deleted
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: tfStateName}, &corev1.ConfigMap{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: tfVariablesName}, &corev1.Secret{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: tfConfigName}, &corev1.ConfigMap{})).To(BeNotFoundError())
		})

		It("should remove the terraform finalizer from all resources", func() {
			// Create the resources with the terraformer finalizer
			Expect(c.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  namespace,
					Name:       tfVariablesName,
					Finalizers: []string{TerraformerFinalizer},
				},
			})).To(Succeed())
			Expect(c.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  namespace,
					Name:       tfStateName,
					Finalizers: []string{TerraformerFinalizer},
				},
			})).To(Succeed())
			Expect(c.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  namespace,
					Name:       tfConfigName,
					Finalizers: []string{TerraformerFinalizer},
				},
			})).To(Succeed())

			t := New(log, c, nil, purpose, namespace, name, image)
			Expect(t.RemoveTerraformerFinalizerFromConfig(ctx)).NotTo(HaveOccurred())

			// Verify finalizers are removed from all resources
			secret := &corev1.Secret{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: tfVariablesName}, secret)).To(Succeed())
			Expect(secret.Finalizers).To(BeEmpty())

			stateCM := &corev1.ConfigMap{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: tfStateName}, stateCM)).To(Succeed())
			Expect(stateCM.Finalizers).To(BeEmpty())

			configCM := &corev1.ConfigMap{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: tfConfigName}, configCM)).To(Succeed())
			Expect(configCM.Finalizers).To(BeEmpty())
		})
	})
})
