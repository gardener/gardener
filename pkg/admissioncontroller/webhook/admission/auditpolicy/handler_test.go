// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package auditpolicy_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/auditpolicy"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("handler", func() {
	var (
		ctx = context.TODO()
		log logr.Logger

		request admission.Request
		decoder *admission.Decoder
		handler *Handler

		ctrl       *gomock.Controller
		mockReader *mockclient.MockReader
		fakeClient client.Client

		statusCodeAllowed       int32 = http.StatusOK
		statusCodeInvalid       int32 = http.StatusUnprocessableEntity
		statusCodeInternalError int32 = http.StatusInternalServerError

		testEncoder runtime.Encoder

		cmName         = "fake-cm-name"
		cmNameOther    = "fake-cm-name-other"
		cmNamespace    = "fake-cm-namespace"
		shootName      = "fake-shoot-name"
		shootNamespace = cmNamespace

		cm                  *corev1.ConfigMap
		shootv1beta1        *gardencorev1beta1.Shoot
		shootv1beta1K8sV124 *gardencorev1beta1.Shoot
		shootv1beta1K8sV123 *gardencorev1beta1.Shoot

		validAuditPolicy = `
---
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: RequestResponse
    resources:
    - group: ""
      resources: ["pods"]
  - level: Metadata
    resources:
    - group: ""
      resources: ["pods/log", "pods/status"]
`
		anotherValidAuditPolicy = `
---
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: RequestResponse
    resources:
    - group: ""
      resources: ["pods"]
  - level: Metadata
    resources:
    - group: ""
      resources: ["pods/log"]
`
		missingKeyAuditPolicy = `
---
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: RequestResponse
    resources:
    - group: "
      resources: ["pods"]
`
		invalidAuditPolicy = `
---
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: FakeLevel
    resources:
    - group: ""
      resources: ["pods"]
  - level: Metadata
    resources:
    - group: ""
      resources: ["pods/log", "pods/status"]
`
		validAuditPolicyV1alpha1 = `
---
apiVersion: audit.k8s.io/v1alpha1
kind: Policy
rules:
  - level: RequestResponse
    resources:
    - group: ""
      resources: ["pods"]
  - level: Metadata
    resources:
    - group: ""
      resources: ["pods/log", "pods/status"]
`
		validAuditPolicyV1beta1 = `
---
apiVersion: audit.k8s.io/v1beta1
kind: Policy
rules:
  - level: RequestResponse
    resources:
    - group: ""
      resources: ["pods"]
  - level: Metadata
    resources:
    - group: ""
      resources: ["pods/log", "pods/status"]
`
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		testEncoder = &jsonserializer.Serializer{}

		ctrl = gomock.NewController(GinkgoT())
		mockReader = mockclient.NewMockReader(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		var err error
		decoder, err = admission.NewDecoder(kubernetes.GardenScheme)
		Expect(err).NotTo(HaveOccurred())

		handler = &Handler{Logger: log, APIReader: mockReader, Client: fakeClient}
		Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

		request = admission.Request{}

		shootv1beta1 = &gardencorev1beta1.Shoot{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						AuditConfig: &gardencorev1beta1.AuditConfig{
							AuditPolicy: &gardencorev1beta1.AuditPolicy{
								ConfigMapRef: &corev1.ObjectReference{
									Name: cmName,
								},
							},
						},
					},
				},
			},
		}

		shootv1beta1K8sV124 = &gardencorev1beta1.Shoot{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						AuditConfig: &gardencorev1beta1.AuditConfig{
							AuditPolicy: &gardencorev1beta1.AuditPolicy{
								ConfigMapRef: &corev1.ObjectReference{
									Name: cmName,
								},
							},
						},
					},
					Version: "1.24.0",
				},
			},
		}

		shootv1beta1K8sV123 = &gardencorev1beta1.Shoot{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						AuditConfig: &gardencorev1beta1.AuditConfig{
							AuditPolicy: &gardencorev1beta1.AuditPolicy{
								ConfigMapRef: &corev1.ObjectReference{
									Name: cmName,
								},
							},
						},
					},
					Version: "1.23.2",
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	test := func(op admissionv1.Operation, oldObj runtime.Object, obj runtime.Object, expectedAllowed bool, expectedStatusCode int32, expectedMsg string, expectedReason string) {
		request.Operation = op

		if oldObj != nil {
			objData, err := runtime.Encode(testEncoder, oldObj)
			Expect(err).NotTo(HaveOccurred())
			request.OldObject.Raw = objData
		}

		if obj != nil {
			objData, err := runtime.Encode(testEncoder, obj)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData
		}

		response := handler.Handle(ctx, request)
		Expect(response).To(Not(BeNil()))
		Expect(response.Allowed).To(Equal(expectedAllowed))
		Expect(response.Result.Code).To(Equal(expectedStatusCode))
		if expectedMsg != "" {
			Expect(response.Result.Message).To(ContainSubstring(expectedMsg))
		}
		if expectedReason != "" {
			Expect(string(response.Result.Reason)).To(ContainSubstring(expectedReason))
		}
		Expect(response.Patches).To(BeEmpty())
	}

	Context("Shoots", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "Shoot"}
		})

		Context("Allow", func() {
			It("has no KubeAPIServer config", func() {
				shootv1beta1.Spec.Kubernetes.KubeAPIServer = nil
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "shoot resource is not specifying any audit policy", "")
			})

			It("has no AuditConfig", func() {
				shootv1beta1.Spec.Kubernetes.KubeAPIServer.AuditConfig = nil
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "shoot resource is not specifying any audit policy", "")
			})

			It("has no audit policy cm Ref", func() {
				shootv1beta1.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef = nil
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "shoot resource is not specifying any audit policy", "")
			})

			It("references a valid auditPolicy (CREATE)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
					Data:       map[string]string{"policy": validAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "referenced audit policy is valid", "")
			})

			It("references a valid auditPolicy/v1alpha1 (CREATE k8s < 1.24.0)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
					Data:       map[string]string{"policy": validAuditPolicyV1alpha1},
				}
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1K8sV123, true, statusCodeAllowed, "referenced audit policy is valid", "")
			})

			It("references a valid auditPolicy/v1beta1 (CREATE k8s < 1.24.0)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
					Data:       map[string]string{"policy": validAuditPolicyV1beta1},
				}
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1K8sV123, true, statusCodeAllowed, "referenced audit policy is valid", "")
			})

			It("referenced auditPolicy name was not changed (UPDATE)", func() {
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.AllowPrivilegedContainers = pointer.Bool(false)
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "audit policy configmap was not changed", "")
			})

			It("auditPolicy name was added (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"policy": validAuditPolicy},
				}
				apiServerConfig := shootv1beta1.Spec.Kubernetes.KubeAPIServer.DeepCopy()
				shootv1beta1.Spec.Kubernetes.KubeAPIServer = nil
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer = apiServerConfig
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced audit policy is valid", "")
			})

			It("referenced auditPolicy name was changed (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"policy": validAuditPolicy},
				}
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name = cmNameOther
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmNameOther), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced audit policy is valid", "")
			})

			It("referenced auditPolicy name was removed (UPDATE)", func() {
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer = nil
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "shoot resource is not specifying any audit policy", "")
			})

			It("should not validate auditPolicy if already marked for deletion (UPDATE)", func() {
				now := metav1.Now()
				shootv1beta1.DeletionTimestamp = &now
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Labels = map[string]string{
					"foo": "bar",
				}
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "marked for deletion", "")
			})

			It("should not validate auditPolicy if spec wasn't changed (UPDATE)", func() {
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Labels = map[string]string{
					"foo": "bar",
				}
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "shoot spec was not changed", "")
			})
		})

		Context("Deny", func() {
			It("references a configmap that does not exist", func() {
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), &corev1.ConfigMap{}).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, cmName)
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "referenced audit policy does not exist", "")
			})

			It("fails getting cm", func() {
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), &corev1.ConfigMap{}).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					return fmt.Errorf("fake")
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInternalError, "could not retrieve config map: fake", "")
			})

			It("references configmap without a policy key", func() {
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), &corev1.ConfigMap{}).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = corev1.ConfigMap{
						Data: nil,
					}
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "missing '.data.policy' in audit policy configmap", "")
			})

			It("references audit policy which breaks validation rules", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": invalidAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "Unsupported value: \"FakeLevel\"", "")
			})

			It("references audit policy with invalid structure", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": missingKeyAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "did not find expected key", "")
			})

			It("references a valid auditPolicy/v1alpha1 (CREATE k8s >= 1.24.0)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": validAuditPolicyV1alpha1},
				}
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1K8sV124, false, statusCodeInvalid, "audit policy with apiVersion 'v1alpha1' is not supported for shoot 'fake-shoot-name' with Kubernetes version >= 1.24.0", "")
			})

			It("references a valid auditPolicy/v1alpha1 (UPDATE kubernetes version)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": validAuditPolicyV1alpha1},
				}
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})

				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.Version = "1.24.0"
				test(admissionv1.Update, shootv1beta1, newShoot, false, statusCodeInvalid, "audit policy with apiVersion 'v1alpha1' is not supported for shoot 'fake-shoot-name' with Kubernetes version >= 1.24.0", "")
			})

			It("references a valid auditPolicy/v1beta1 (CREATE k8s >= 1.24.0)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": validAuditPolicyV1beta1},
				}
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1K8sV124, false, statusCodeInvalid, "audit policy with apiVersion 'v1beta1' is not supported for shoot 'fake-shoot-name' with Kubernetes version >= 1.24.0", "")
			})

			It("references a valid auditPolicy/v1beta1 (UPDATE kubernetes version)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": validAuditPolicyV1beta1},
				}
				mockReader.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})

				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.Version = "1.24.0"
				test(admissionv1.Update, shootv1beta1, newShoot, false, statusCodeInvalid, "audit policy with apiVersion 'v1beta1' is not supported for shoot 'fake-shoot-name' with Kubernetes version >= 1.24.0", "")
			})

		})
	})

	Context("ConfigMaps", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}

			cm = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: cmNamespace,
				},
				Data: map[string]string{
					"policy": validAuditPolicy,
				},
			}
		})

		Context("Update", func() {
			BeforeEach(func() {
				request.Name = cmName
				request.Namespace = cmNamespace
			})

			Context("Allow", func() {
				It("is not reference by any shoot", func() {
					shootInSameNamespaceButNotReferencing := shootv1beta1.DeepCopy()
					shootInSameNamespaceButNotReferencing.Spec.Kubernetes.KubeAPIServer = nil
					Expect(fakeClient.Create(ctx, shootInSameNamespaceButNotReferencing)).To(Succeed())
					shootInDifferentNamespaceAndReferencing := shootv1beta1.DeepCopy()
					shootInDifferentNamespaceAndReferencing.Namespace = shootNamespace + "other"
					Expect(fakeClient.Create(ctx, shootInDifferentNamespaceAndReferencing)).To(Succeed())

					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "configmap is not referenced by a Shoot", "")
				})

				It("did not change policy field", func() {
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())
					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "audit policy not changed", "")
				})

				It("should allow if the auditPolicy is changed to something valid", func() {
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())
					shootv1beta1.Spec.Kubernetes.Version = "1.20"
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = anotherValidAuditPolicy

					test(admissionv1.Update, cm, newCm, true, statusCodeAllowed, "configmap change is valid", "")
				})
			})

			Context("Deny", func() {
				BeforeEach(func() {
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())
				})

				It("has no data key", func() {
					newCm := cm.DeepCopy()
					newCm.Data = nil
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "missing '.data.policy' in audit policy configmap", "")
				})

				It("has empty policy", func() {
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = ""
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "empty audit policy. Provide non-empty audit policy", "")
				})

				It("holds audit policy which breaks validation rules", func() {
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = invalidAuditPolicy

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "Unsupported value: \"FakeLevel\"", "")
				})

				It("holds audit policy with invalid YAML structure", func() {
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = missingKeyAuditPolicy

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "did not find expected key", "")
				})

				It("holds audit policy with unsupported version for the kubernetes version of one of the shoots", func() {
					patch := client.MergeFrom(shootv1beta1.DeepCopy())
					shootv1beta1.Spec.Kubernetes.Version = "1.23"
					Expect(fakeClient.Patch(ctx, shootv1beta1, patch)).To(Succeed())

					shootv1beta1K8sV124.Name = "fake-shoot-name-2"
					Expect(fakeClient.Create(ctx, shootv1beta1K8sV124)).To(Succeed())

					newCm := cm.DeepCopy()
					newCm.Data["policy"] = validAuditPolicyV1beta1

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "audit policy with apiVersion 'v1beta1' is not supported for shoot 'fake-shoot-name-2' with Kubernetes version >= 1.24.0", "")
				})
			})
		})
	})
})
