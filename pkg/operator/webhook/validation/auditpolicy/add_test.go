// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package auditpolicy_test

import (
	"context"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/webhook/validation/auditpolicy"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("handler", func() {
	const (
		statusCodeAllowed       int32 = http.StatusOK
		statusCodeInvalid       int32 = http.StatusUnprocessableEntity
		statusCodeInternalError int32 = http.StatusInternalServerError
	)

	var (
		ctx = context.TODO()

		request admission.Request
		decoder admission.Decoder
		handler admission.Handler

		ctrl       *gomock.Controller
		mockReader *mockclient.MockReader
		fakeClient client.Client

		testEncoder runtime.Encoder

		cmName, cmNameOther, gardenName, gardenNs string

		cm     *corev1.ConfigMap
		garden *operatorv1alpha1.Garden

		validAuditPolicy, anotherValidAuditPolicy, missingKeyAuditPolicy,
		invalidAuditPolicy, validAuditPolicyV1alpha1 string
	)

	BeforeEach(func() {
		cmName = "fake-cm-name"
		cmNameOther = "fake-cm-name-other"
		gardenName = "fake-garden"
		gardenNs = "garden"

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

		testEncoder = &jsonserializer.Serializer{}

		ctrl = gomock.NewController(GinkgoT())
		mockReader = mockclient.NewMockReader(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		decoder = admission.NewDecoder(operatorclient.RuntimeScheme)

		handler = NewHandler(mockReader, fakeClient, decoder, gardenNs)

		request = admission.Request{}

		garden = &operatorv1alpha1.Garden{
			TypeMeta: metav1.TypeMeta{
				APIVersion: operatorv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Garden",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: gardenName,
			},
			Spec: operatorv1alpha1.GardenSpec{
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					Gardener: operatorv1alpha1.Gardener{
						APIServer: &operatorv1alpha1.GardenerAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: cmName,
									},
								},
							},
						},
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.31.1",
					},
				},
			},
		}

		cm = &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: gardenNs,
			},
			Data: map[string]string{
				"policy": validAuditPolicy,
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
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
			request.OldObject.Raw = objData
		}

		if obj != nil {
			objData, err := runtime.Encode(testEncoder, obj)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
			request.Object.Raw = objData
		}

		response := handler.Handle(ctx, request)
		ExpectWithOffset(1, response).ToNot(BeNil())
		ExpectWithOffset(1, response.Allowed).To(Equal(expectedAllowed))
		ExpectWithOffset(1, response.Result.Code).To(Equal(expectedStatusCode))
		if expectedMsg != "" {
			ExpectWithOffset(1, response.Result.Message).To(ContainSubstring(expectedMsg))
		}
		if expectedReason != "" {
			ExpectWithOffset(1, string(response.Result.Reason)).To(ContainSubstring(expectedReason))
		}
		ExpectWithOffset(1, response.Patches).To(BeEmpty())
	}

	Context("Gardens", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "operator.gardener.cloud", Version: "v1alpha1", Kind: "Garden"}
		})

		Context("Allow", func() {
			It("has no APIServer config", func() {
				garden.Spec.VirtualCluster.Gardener.APIServer = nil
				test(admissionv1.Create, nil, garden, true, statusCodeAllowed, "no audit policy config map reference found in garden spec", "")
			})

			It("has no AuditConfig", func() {
				garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig = nil
				test(admissionv1.Create, nil, garden, true, statusCodeAllowed, "no audit policy config map reference found in garden spec", "")
			})

			It("has no audit policy", func() {
				garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy = nil
				test(admissionv1.Create, nil, garden, true, statusCodeAllowed, "no audit policy config map reference found in garden spec", "")
			})

			It("has no audit policy cm Ref", func() {
				garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef = nil
				test(admissionv1.Create, nil, garden, true, statusCodeAllowed, "no audit policy config map reference found in garden spec", "")
			})

			It("references a valid auditPolicy (CREATE)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
					Data:       map[string]string{"policy": validAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: gardenNs, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: gardenNs, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, garden, true, statusCodeAllowed, "referenced configMap is valid", "")
			})

			It("referenced auditPolicy name was not changed (UPDATE)", func() {
				newGarden := garden.DeepCopy()
				newGarden.Labels = map[string]string{"foo": "bar"}
				test(admissionv1.Update, garden, newGarden, true, statusCodeAllowed, "garden spec was not changed", "")
			})

			It("auditPolicy name was added (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"policy": validAuditPolicy},
				}
				apiServerConfig := garden.Spec.VirtualCluster.Gardener.APIServer.DeepCopy()
				garden.Spec.VirtualCluster.Gardener.APIServer = nil
				newGarden := garden.DeepCopy()
				newGarden.Spec.VirtualCluster.Gardener.APIServer = apiServerConfig
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: gardenNs, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: gardenNs, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, garden, newGarden, true, statusCodeAllowed, "referenced configMap is valid", "")
			})

			It("referenced auditPolicy name was changed (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"policy": validAuditPolicy},
				}
				newGarden := garden.DeepCopy()
				newGarden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name = cmNameOther
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: gardenNs, Name: cmNameOther}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, garden, newGarden, true, statusCodeAllowed, "referenced configMap is valid", "")
			})

			It("referenced auditPolicy name was removed (UPDATE)", func() {
				newGarden := garden.DeepCopy()
				newGarden.Spec.VirtualCluster.Gardener.APIServer = nil
				test(admissionv1.Update, garden, newGarden, true, statusCodeAllowed, "no audit policy config map reference found in garden spec", "")
			})

			It("should not validate auditPolicy if already marked for deletion (UPDATE)", func() {
				now := metav1.Now()
				garden.DeletionTimestamp = &now
				newGarden := garden.DeepCopy()
				newGarden.Labels = map[string]string{
					"foo": "bar",
				}
				test(admissionv1.Update, garden, newGarden, true, statusCodeAllowed, "marked for deletion", "")
			})
		})

		Context("Deny", func() {
			It("references a configmap that does not exist", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: gardenNs, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: gardenNs, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ *corev1.ConfigMap, _ ...client.GetOption) error {
					return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, cmName)
				})
				test(admissionv1.Create, nil, garden, false, statusCodeInvalid, `referenced ConfigMap garden/fake-cm-name does not exist: configmaps "fake-cm-name" not found`, "")
			})

			It("fails getting cm", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: gardenNs, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: gardenNs, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ *corev1.ConfigMap, _ ...client.GetOption) error {
					return errors.New("fake")
				})
				test(admissionv1.Create, nil, garden, false, statusCodeInternalError, "could not retrieve ConfigMap garden/fake-cm-name: fake", "")
			})

			It("references configmap without a policy key", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: gardenNs, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: gardenNs, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = corev1.ConfigMap{
						Data: nil,
					}
					return nil
				})
				test(admissionv1.Create, nil, garden, false, statusCodeInvalid, "error getting ConfigMap /: missing audit policy key in policy ConfigMap data", "")
			})

			It("references audit policy which breaks validation rules", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": invalidAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: gardenNs, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: gardenNs, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, garden, false, statusCodeInvalid, "Unsupported value: \"FakeLevel\"", "")
			})

			It("references audit policy with invalid structure", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": missingKeyAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: gardenNs, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: gardenNs, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, garden, false, statusCodeInvalid, "did not find expected key", "")
			})

			It("references a deprecated auditPolicy/v1alpha1", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": validAuditPolicyV1alpha1},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: gardenNs, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: gardenNs, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, garden, false, statusCodeInvalid, "no kind \"Policy\" is registered for version \"audit.k8s.io/v1alpha1\"", "")
			})
		})
	})

	Context("ConfigMaps", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
			request.Name = cmName
			request.Namespace = gardenNs
		})

		Context("Update", func() {
			Context("Allow", func() {
				It("is not referenced by any garden", func() {
					gardenNotReferencing := garden.DeepCopy()
					gardenNotReferencing.Spec.VirtualCluster.Gardener.APIServer = nil
					Expect(fakeClient.Create(ctx, gardenNotReferencing)).To(Succeed())

					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "config map is not referenced by garden resource, nothing to validate", "")
				})

				It("did not change policy field", func() {
					Expect(fakeClient.Create(ctx, garden)).To(Succeed())
					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "audit policy did not change", "")
				})

				It("should allow if the auditPolicy is changed to something valid", func() {
					Expect(fakeClient.Create(ctx, garden)).To(Succeed())
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = anotherValidAuditPolicy

					test(admissionv1.Update, cm, newCm, true, statusCodeAllowed, "referenced audit policy is valid", "")
				})
			})

			Context("Deny", func() {
				BeforeEach(func() {
					Expect(fakeClient.Create(ctx, garden)).To(Succeed())
				})

				It("has no data key", func() {
					newCm := cm.DeepCopy()
					newCm.Data = nil
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "error getting audit policy from ConfigMap garden/fake-cm-name: missing audit policy key in policy ConfigMap data", "")
				})

				It("has empty policy", func() {
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = ""
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "error getting audit policy from ConfigMap garden/fake-cm-name: audit policy in policy key is empty", "")
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
			})
		})
	})
})
