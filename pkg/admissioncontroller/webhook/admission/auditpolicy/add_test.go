// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package auditpolicy_test

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-logr/logr"
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
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/auditpolicy"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("handler", func() {
	var (
		ctx = context.TODO()
		log logr.Logger

		request admission.Request
		decoder admission.Decoder
		handler admission.Handler

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

		cm           *corev1.ConfigMap
		shootv1beta1 *gardencorev1beta1.Shoot

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
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		testEncoder = &jsonserializer.Serializer{}

		ctrl = gomock.NewController(GinkgoT())
		mockReader = mockclient.NewMockReader(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		decoder = admission.NewDecoder(kubernetes.GardenScheme)

		handler = NewHandler(log, mockReader, fakeClient, decoder)

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
					Version: "1.31.1",
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
		ExpectWithOffset(1, response).To(Not(BeNil()))
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

	Context("Shoots", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "Shoot"}
		})

		Context("Allow", func() {
			It("has no KubeAPIServer config", func() {
				shootv1beta1.Spec.Kubernetes.KubeAPIServer = nil
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "Shoot resource does not specify any audit policy ConfigMap", "")
			})

			It("has no AuditConfig", func() {
				shootv1beta1.Spec.Kubernetes.KubeAPIServer.AuditConfig = nil
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "Shoot resource does not specify any audit policy ConfigMap", "")
			})

			It("has no audit policy cm Ref", func() {
				shootv1beta1.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef = nil
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "Shoot resource does not specify any audit policy ConfigMap", "")
			})

			It("references a valid auditPolicy (CREATE)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
					Data:       map[string]string{"policy": validAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "referenced audit policy is valid", "")
			})

			It("referenced auditPolicy name was not changed (UPDATE)", func() {
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{Name: "some-plugin"}}
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "Neither audit policy ConfigMap nor Kubernetes version or other relevant fields were changed", "")
			})

			It("auditPolicy name was added (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"policy": validAuditPolicy},
				}
				apiServerConfig := shootv1beta1.Spec.Kubernetes.KubeAPIServer.DeepCopy()
				shootv1beta1.Spec.Kubernetes.KubeAPIServer = nil
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer = apiServerConfig
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
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
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmNameOther}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced audit policy is valid", "")
			})

			It("referenced auditPolicy name was removed (UPDATE)", func() {
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer = nil
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "Shoot resource does not specify any audit policy ConfigMap", "")
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
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ *corev1.ConfigMap, _ ...client.GetOption) error {
					return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, cmName)
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, `referenced audit policy ConfigMap fake-cm-namespace/fake-cm-name does not exist: configmaps "fake-cm-name" not found`, "")
			})

			It("fails getting cm", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ *corev1.ConfigMap, _ ...client.GetOption) error {
					return errors.New("fake")
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInternalError, "could not retrieve audit policy ConfigMap fake-cm-namespace/fake-cm-name: fake", "")
			})

			It("references configmap without a policy key", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = corev1.ConfigMap{
						Data: nil,
					}
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "error getting audit policy from ConfigMap /: missing audit policy key in policy ConfigMap data", "")
			})

			It("references audit policy which breaks validation rules", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": invalidAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
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
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "did not find expected key", "")
			})

			It("references a deprecated auditPolicy/v1alpha1", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"policy": validAuditPolicyV1alpha1},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: cmName}}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "no kind \"Policy\" is registered for version \"audit.k8s.io/v1alpha1\"", "")
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
				It("is not referenced by any shoot", func() {
					shootInSameNamespaceButNotReferencing := shootv1beta1.DeepCopy()
					shootInSameNamespaceButNotReferencing.Spec.Kubernetes.KubeAPIServer = nil
					Expect(fakeClient.Create(ctx, shootInSameNamespaceButNotReferencing)).To(Succeed())
					shootInDifferentNamespaceAndReferencing := shootv1beta1.DeepCopy()
					shootInDifferentNamespaceAndReferencing.Namespace = shootNamespace + "other"
					Expect(fakeClient.Create(ctx, shootInDifferentNamespaceAndReferencing)).To(Succeed())

					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "ConfigMap is not referenced by a Shoot", "")
				})

				It("did not change policy field", func() {
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())
					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "audit policy did not change", "")
				})

				It("should allow if the auditPolicy is changed to something valid", func() {
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = anotherValidAuditPolicy

					test(admissionv1.Update, cm, newCm, true, statusCodeAllowed, "referenced audit policy is valid", "")
				})
			})

			Context("Deny", func() {
				BeforeEach(func() {
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())
				})

				It("has no data key", func() {
					newCm := cm.DeepCopy()
					newCm.Data = nil
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "error getting audit policy from ConfigMap fake-cm-namespace/fake-cm-name: missing audit policy key in policy ConfigMap data", "")
				})

				It("has empty policy", func() {
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = ""
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "error getting audit policy from ConfigMap fake-cm-namespace/fake-cm-name: audit policy in policy key is empty", "")
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
