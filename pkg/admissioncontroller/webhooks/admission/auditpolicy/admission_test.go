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

package auditpolicy_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/auditpolicy"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("handler", func() {
	var (
		ctx    = context.TODO()
		logger logr.Logger

		request admission.Request
		decoder *admission.Decoder
		handler admission.Handler

		ctrl       *gomock.Controller
		mockReader *mockclient.MockReader

		statusCodeAllowed       int32 = http.StatusOK
		statusCodeInvalid       int32 = http.StatusUnprocessableEntity
		statusCodeInternalError int32 = http.StatusInternalServerError

		testEncoder runtime.Encoder

		cmName         = "fake-cm-name"
		cmNamespace    = "fake-cm-namespace"
		shootName      = "fake-shoot-name"
		shootNamespace = "fake-shoot-namespace"

		cm    *v1.ConfigMap
		shoot *gardencorev1beta1.Shoot

		validAuditPolicy = `
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
		anotherValidAuditPolicy = `
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
      resources: ["pods/log"]
`
		missingKeyAuditPolicy = `
---
apiVersion: audit.k8s.io/v1beta1
kind: Policy
rules:
  - level: RequestResponse
    resources:
    - group: "
      resources: ["pods"]
`
		invalidAuditPolicy = `
---
apiVersion: audit.k8s.io/v1beta1
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
		v1AuditPolicy = `
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
	)

	BeforeEach(func() {
		logger = logzap.New(logzap.WriteTo(GinkgoWriter))
		testEncoder = &json.Serializer{}

		ctrl = gomock.NewController(GinkgoT())
		mockReader = mockclient.NewMockReader(ctrl)

		var err error
		decoder, err = admission.NewDecoder(kubernetes.GardenScheme)
		Expect(err).NotTo(HaveOccurred())

		handler = auditpolicy.New(logger)
		Expect(inject.APIReaderInto(mockReader, handler)).To(BeTrue())
		Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

		request = admission.Request{}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#ValidateAuditPolicyApiGroupVersionKind", func() {
		var kind = "Policy"

		It("should return false without error because of version incompatibility", func() {
			incompatibilityMatrix := map[string][]schema.GroupVersionKind{
				"1.10.0": {
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.11.0": {
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
			}

			for shootVersion, gvks := range incompatibilityMatrix {
				for _, gvk := range gvks {
					ok, err := auditpolicy.IsValidAuditPolicyVersion(shootVersion, &gvk)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeFalse())
				}
			}
		})

		It("should return true without error because of version compatibility", func() {
			compatibilityMatrix := map[string][]schema.GroupVersionKind{
				"1.10.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
				},
				"1.11.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
				},
				"1.12.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.13.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.14.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.15.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
			}

			for shootVersion, gvks := range compatibilityMatrix {
				for _, gvk := range gvks {
					ok, err := auditpolicy.IsValidAuditPolicyVersion(shootVersion, &gvk)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeTrue())
				}
			}
		})

		It("should return true without error because of a valid semver version with dev tag", func() {
			shootVersion := "1.12.3-dev"
			gvks := []schema.GroupVersionKind{
				auditv1alpha1.SchemeGroupVersion.WithKind(kind),
				auditv1beta1.SchemeGroupVersion.WithKind(kind),
				auditv1.SchemeGroupVersion.WithKind(kind),
			}

			for _, gvk := range gvks {
				ok, err := auditpolicy.IsValidAuditPolicyVersion(shootVersion, &gvk)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			}
		})

		It("should return false with error because of not valid semver version", func() {
			shootVersion := "1.ab.0"
			gvk := auditv1.SchemeGroupVersion.WithKind(kind)

			ok, err := auditpolicy.IsValidAuditPolicyVersion(shootVersion, &gvk)
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	test := func(op admissionv1.Operation, oldObj runtime.Object, obj runtime.Object, expectedAllowed bool, expectedStatusCode int32, expectedMsg string) {
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
	}

	Context("Shoots", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "Shoot"}
			shoot = &gardencorev1beta1.Shoot{
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
									ConfigMapRef: &v1.ObjectReference{
										Name: cmName,
									},
								},
							},
						},
					},
				},
			}
		})

		It("should ignore other operations than CREATE or UPDATE", func() {
			test(admissionv1.Delete, shoot, nil, true, statusCodeAllowed, "operation is not Create or Update")
			test(admissionv1.Connect, shoot, nil, true, statusCodeAllowed, "operation is not Create or Update")
		})

		Context("Allow", func() {

			It("has no KubeAPIServer config", func() {
				shoot.Spec.Kubernetes.KubeAPIServer = nil
				test(admissionv1.Create, nil, shoot, true, statusCodeAllowed, "shoot resource is not specifying any audit policy")
				test(admissionv1.Update, shoot, shoot, true, statusCodeAllowed, "shoot resource is not specifying any audit policy")
			})

			It("has no AuditConfig", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig = nil
				test(admissionv1.Create, nil, shoot, true, statusCodeAllowed, "shoot resource is not specifying any audit policy")
			})

			It("has no audit policy cm Ref", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef = nil
				test(admissionv1.Create, nil, shoot, true, statusCodeAllowed, "shoot resource is not specifying any audit policy")
			})

			It("cm Ref did not change", func() {
				newShoot := shoot.DeepCopy()
				newShoot.Labels = map[string]string{
					"foo": "bar",
				}
				test(admissionv1.Update, shoot, newShoot, true, statusCodeAllowed, "audit policy reference did not change in Shoot resource")
			})

			It("references a valid auditPolicy", func() {
				returnedCm := v1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Data:       map[string]string{"policy": validAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), kutil.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&v1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *v1.ConfigMap) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shoot, true, statusCodeAllowed, "referenced audit policy is valid")
			})

		})

		Context("Deny", func() {

			It("references a configmap that does not exist", func() {
				mockReader.EXPECT().Get(gomock.Any(), kutil.Key(shootNamespace, cmName), &v1.ConfigMap{}).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *v1.ConfigMap) error {
					return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, cmName)
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInvalid, "referenced audit policy does not exist")
			})

			It("fails getting cm", func() {
				mockReader.EXPECT().Get(gomock.Any(), kutil.Key(shootNamespace, cmName), &v1.ConfigMap{}).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *v1.ConfigMap) error {
					return fmt.Errorf("fake")
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInternalError, "could not retrieve config map: fake")
			})

			It("references configmap without a policy key", func() {
				mockReader.EXPECT().Get(gomock.Any(), kutil.Key(shootNamespace, cmName), &v1.ConfigMap{}).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *v1.ConfigMap) error {
					*cm = v1.ConfigMap{
						Data: nil,
					}
					return nil
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInvalid, "missing '.data.policy' in audit policy configmap")
			})

			It("references audit policy which breaks validation rules", func() {
				returnedCm := v1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Data:       map[string]string{"policy": invalidAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), kutil.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&v1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *v1.ConfigMap) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInvalid, "Unsupported value: \"FakeLevel\"")
			})

			It("references audit policy with invalid structure", func() {
				returnedCm := v1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Data:       map[string]string{"policy": missingKeyAuditPolicy},
				}
				mockReader.EXPECT().Get(gomock.Any(), kutil.Key(shootNamespace, cmName), gomock.AssignableToTypeOf(&v1.ConfigMap{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, cm *v1.ConfigMap) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInvalid, "did not find expected key")
			})
		})
	})

	Context("Configmaps", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}

			cm = &v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: cmNamespace,
					Finalizers: []string{
						"gardener.cloud/reference-protection",
					},
				},
				Data: map[string]string{
					"policy": validAuditPolicy,
				},
			}
		})

		Context("ignored requests", func() {

			It("should ignore other operations than UPDATE", func() {
				test(admissionv1.Create, cm, cm, true, statusCodeAllowed, "operation is not update")
				test(admissionv1.Connect, cm, cm, true, statusCodeAllowed, "operation is not update")
				test(admissionv1.Delete, cm, cm, true, statusCodeAllowed, "operation is not update")
			})

			It("should ignore other resources than Configmaps", func() {
				request.Kind = metav1.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"}

				test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "resource is not core.gardener.cloud/v1beta1.shoot or v1.configmap")
			})
		})

		Context("Update", func() {

			BeforeEach(func() {
				request.Name = cmName
				request.Namespace = cmNamespace

				shoot = &gardencorev1beta1.Shoot{}
				shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
					AuditConfig: &gardencorev1beta1.AuditConfig{
						AuditPolicy: &gardencorev1beta1.AuditPolicy{
							ConfigMapRef: &v1.ObjectReference{Name: cmName},
						},
					},
				}
			})

			Context("Allow", func() {

				It("does not have a finalizer from a Shoot", func() {
					cm.ObjectMeta.Finalizers = nil
					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "configmap is not referenced by a Shoot")
				})
				It("did not change policy field", func() {
					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "audit policy not changed")
				})

				It("should allow if the auditPolicy is changed to something valid", func() {
					shoot.Spec.Kubernetes.Version = "1.15"
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = anotherValidAuditPolicy

					mockReader.EXPECT().List(gomock.Any(), &gardencorev1beta1.ShootList{}, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
						*list = gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{
							*shoot,
						}}
						return nil
					})
					test(admissionv1.Update, cm, newCm, true, statusCodeAllowed, "configmap change is valid")
				})

			})

			Context("Deny", func() {

				It("has no data key", func() {
					newCm := cm.DeepCopy()
					newCm.Data = nil
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "missing '.data.policy' in audit policy configmap")
				})

				It("has empty policy", func() {
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = ""
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "empty audit policy. Provide non-empty audit policy")
				})

				It("fails listing shoots", func() {
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = anotherValidAuditPolicy

					mockReader.EXPECT().List(gomock.Any(), &gardencorev1beta1.ShootList{}, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
						return fmt.Errorf("fake")
					})
					test(admissionv1.Update, cm, newCm, false, statusCodeInternalError, "")
				})

				It("should fail if shoot cluster version is incompatible with the audit policy version", func() {
					shoot.Spec.Kubernetes.Version = "1.10"
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = v1AuditPolicy

					mockReader.EXPECT().List(gomock.Any(), &gardencorev1beta1.ShootList{}, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
						*list = gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{
							*shoot,
						}}
						return nil
					})
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "your shoot cluster version")
				})

				It("holds audit policy which breaks validation rules", func() {
					cm.DeepCopy()
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = invalidAuditPolicy

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "Unsupported value: \"FakeLevel\"")
				})

				It("holds audit policy with invalid YAML structure", func() {
					cm.DeepCopy()
					newCm := cm.DeepCopy()
					newCm.Data["policy"] = missingKeyAuditPolicy

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "did not find expected key")
				})

			})
		})
	})
})
