// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authenticationconfig_test

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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/authenticationconfig"
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

		validAuthenticationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: https://foo.com
    audiences:
    - example-client-id
  claimMappings:
    username:
      claim: username
      prefix: "foo:"
`
		anotherValidAuthenticationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: https://bar.com
    audiences:
    - example-client-id
  certificateAuthority: /foo/bar
  claimMappings:
    username:
      claim: username
      prefix: "foo:"
    groups:
      claim: groups
      prefix: "foo:"
  claimValidationRules:
  - claim: hd
    requiredValue: "foo.com"
  - claim: admin
`
		missingKeyConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: "https://foo.com
    audiences:
    - example-client-id
  claimMappings:
    username:
      claim: username
      prefix: "foo:"
`
		invalidAuthenticationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: https://foo.com
  claimMappings:
    username:
      claim: username
      prefix: "foo:"
`

		invalidIssuerUrl = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: https://abc.com
    audiences:
    - example-client-id
  claimMappings:
    username:
      claim: username
      prefix: "abc:"
`

		anonymousAuthenticationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
  conditions:
  - path: /livez
  - path: /readyz
  - path: /healthz
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
					Version: "1.30.0",
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
							ConfigMapName: cmName,
						},
						ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
							Issuer: ptr.To("https://xyz.com"),
							AcceptedIssuers: []string{
								"https://abc.com",
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
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "Shoot resource does not specify any authentication configuration ConfigMap", "")
			})

			It("has no Structured Authentication", func() {
				shootv1beta1.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication = nil
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "Shoot resource does not specify any authentication configuration ConfigMap", "")
			})

			It("has no authentication configuration cm Ref", func() {
				shootv1beta1.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication.ConfigMapName = ""
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "Shoot resource does not specify any authentication configuration ConfigMap", "")
			})

			It("references a valid authenticationConfiguration (CREATE)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
					Data:       map[string]string{"config.yaml": validAuthenticationConfiguration},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("authenticationConfiguration name was added (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"config.yaml": validAuthenticationConfiguration},
				}
				apiServerConfig := shootv1beta1.Spec.Kubernetes.KubeAPIServer.DeepCopy()
				shootv1beta1.Spec.Kubernetes.KubeAPIServer = nil
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer = apiServerConfig
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("serviceAccountConfig is nil (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"config.yaml": validAuthenticationConfiguration},
				}
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig = nil
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("referenced authenticationConfiguration name was changed (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"config.yaml": validAuthenticationConfiguration},
				}
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication.ConfigMapName = cmNameOther
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmNameOther}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("referenced authenticationConfiguration name was removed (UPDATE)", func() {
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer = nil
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "Shoot resource does not specify any authentication configuration ConfigMap", "")
			})

			It("should not validate authenticationConfiguration if already marked for deletion (UPDATE)", func() {
				now := metav1.Now()
				shootv1beta1.DeletionTimestamp = &now
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Labels = map[string]string{
					"foo": "bar",
				}
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "marked for deletion", "")
			})

			It("should not validate authenticationConfiguration if spec wasn't changed (UPDATE)", func() {
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Labels = map[string]string{
					"foo": "bar",
				}
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "shoot spec was not changed", "")
			})

			It("should allow enabling the legacy anonymous authentication on the kube-apiserver when not using (structured) anonymous authentication configuration", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"config.yaml": validAuthenticationConfiguration}, // does not set `anonymous.enabled: true`
				}
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication = ptr.To(true)
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})
		})

		Context("Deny", func() {
			It("references a configmap that does not exist", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ *corev1.ConfigMap, _ ...client.GetOption) error {
					return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, cmName)
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "referenced authentication configuration ConfigMap fake-cm-namespace/fake-cm-name does not exist: configmaps \"fake-cm-name\" not found", "")
			})

			It("fails getting cm", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ *corev1.ConfigMap, _ ...client.GetOption) error {
					return errors.New("fake")
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInternalError, "could not retrieve authentication configuration ConfigMap fake-cm-namespace/fake-cm-name: fake", "")
			})

			It("references configmap without a config.yaml key", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = corev1.ConfigMap{
						Data: nil,
					}
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "error getting authentication configuration from ConfigMap /: missing authentication configuration key in config.yaml ConfigMap data", "")
			})

			It("references authentication configuration which breaks validation rules", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"config.yaml": invalidAuthenticationConfiguration},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.audiences: Required value: at least one jwt[0].issuer.audiences is required]", "")
			})

			It("references authentication configuration with invalid structure", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"config.yaml": missingKeyConfiguration},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "did not find expected key", "")
			})

			It("contains disallowed issuers in the service account config", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"config.yaml": invalidIssuerUrl},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.url: Invalid value: \"https://abc.com\": URL must not overlap with disallowed issuers:", "")
			})

			It("enables legacy anonymous authentication on the kube-apiserver when anonymous authentication configuration is already present", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"config.yaml": anonymousAuthenticationConfiguration},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: cmName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication = ptr.To(true)
				test(admissionv1.Create, shootv1beta1, newShoot, false, statusCodeInvalid, "cannot use anonymous authentication configuration when the following shoots have the legacy configuration enabled: fake-shoot-name", "")
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
					"config.yaml": validAuthenticationConfiguration,
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

					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "ConfigMap is not referenced by a Shoot", "")
				})

				It("did not change config.yaml field", func() {
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())
					test(admissionv1.Update, cm, cm, true, statusCodeAllowed, "authentication configuration did not change", "")
				})

				It("should allow if the authenticationConfiguration is changed to something valid", func() {
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())
					shootv1beta1.Spec.Kubernetes.Version = "1.30.1"
					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = anotherValidAuthenticationConfiguration

					test(admissionv1.Update, cm, newCm, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
				})

				It("should allow anonymous authentication when the legacy kube-apiserver setting is disabled", func() {
					shootv1beta1.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication = ptr.To(false)
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())

					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = anonymousAuthenticationConfiguration

					test(admissionv1.Update, cm, newCm, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
				})
			})

			Context("Deny", func() {
				BeforeEach(func() {
					Expect(fakeClient.Create(ctx, shootv1beta1)).To(Succeed())
				})

				It("has no data key", func() {
					newCm := cm.DeepCopy()
					newCm.Data = nil
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "error getting authentication configuration from ConfigMap fake-cm-namespace/fake-cm-name: missing authentication configuration key in config.yaml ConfigMap data", "")
				})

				It("has empty config.yaml", func() {
					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = ""
					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "error getting authentication configuration from ConfigMap fake-cm-namespace/fake-cm-name: authentication configuration in config.yaml key is empty", "")
				})

				It("holds authentication configuration which breaks validation rules", func() {
					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = invalidAuthenticationConfiguration

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.audiences: Required value: at least one jwt[0].issuer.audiences is required]", "")
				})

				It("holds authentication configuration with invalid YAML structure", func() {
					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = missingKeyConfiguration

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "did not find expected key", "")
				})

				It("contains disallowed issuers in the issuer url", func() {
					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = invalidIssuerUrl

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.url: Invalid value: \"https://abc.com\": URL must not overlap with disallowed issuers:", "")
				})

				It("uses anonymous authentication and has the legacy kube-apiserver setting already enabled", func() {
					shootv1beta1.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication = ptr.To(true)
					Expect(fakeClient.Update(ctx, shootv1beta1)).To(Succeed())

					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = anonymousAuthenticationConfiguration

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "cannot use anonymous authentication configuration when the following shoots have the legacy configuration enabled: fake-shoot-name", "")
				})
			})
		})
	})
})
