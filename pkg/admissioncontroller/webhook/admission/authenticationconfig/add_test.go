// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authenticationconfig_test

import (
	"context"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/authenticationconfig"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("handler", func() {
	var (
		ctx = context.TODO()

		request admission.Request
		decoder admission.Decoder
		handler admission.Handler

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

		managedIssuerAuthenticationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: https://managed.example.com/projects/my-project/shoots/some-uid/issuer
    audiences:
    - example-client-id
  claimMappings:
    username:
      claim: username
      prefix: "managed:"
`

		computedManagedIssuerAuthenticationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: https://issuer.example.com/projects/test-project/shoots/test-shoot-uid/issuer
    audiences:
    - example-client-id
  claimMappings:
    username:
      claim: username
      prefix: "managed:"
`

		defaultIssuerAuthenticationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: https://api.fake-shoot-name.test-project.internal.example.com
    audiences:
    - example-client-id
  claimMappings:
    username:
      claim: username
      prefix: "default:"
`

		externalIssuerAuthenticationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: https://api.fake-shoot-name.test-project.external.example.com
    audiences:
    - example-client-id
  claimMappings:
    username:
      claim: username
      prefix: "default:"
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
		testEncoder = &jsonserializer.Serializer{}

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		decoder = admission.NewDecoder(kubernetes.GardenScheme)

		handler = NewHandler(fakeClient, fakeClient, decoder)

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
					Version: "1.34.0",
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
							ConfigMapName: cmName,
						},
						ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
							Issuer: new("https://xyz.com"),
							AcceptedIssuers: []string{
								"https://abc.com",
							},
						},
					},
				},
			},
		}
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
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": validAuthenticationConfiguration},
				})).To(Succeed())
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("authenticationConfiguration name was added (UPDATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": validAuthenticationConfiguration},
				})).To(Succeed())
				apiServerConfig := shootv1beta1.Spec.Kubernetes.KubeAPIServer.DeepCopy()
				shootv1beta1.Spec.Kubernetes.KubeAPIServer = nil
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer = apiServerConfig
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("serviceAccountConfig is nil (UPDATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": validAuthenticationConfiguration},
				})).To(Succeed())
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig = nil
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("referenced authenticationConfiguration name was changed (UPDATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmNameOther, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": validAuthenticationConfiguration},
				})).To(Succeed())
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication.ConfigMapName = cmNameOther
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
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": validAuthenticationConfiguration}, // does not set `anonymous.enabled: true`
				})).To(Succeed())
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication = new(true)
				test(admissionv1.Update, shootv1beta1, newShoot, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("references a valid authenticationConfiguration when the service account issuer advertised address uses a different URL (CREATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": validAuthenticationConfiguration},
				})).To(Succeed())
				shootv1beta1.Status = gardencorev1beta1.ShootStatus{
					AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
						{
							Name: v1beta1constants.AdvertisedAddressServiceAccountIssuer,
							URL:  "https://api.my-shoot.my-project.example.com",
						},
					},
				}
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("allows auth config with managed issuer URL when shoot has no UID yet (CREATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   shootNamespace,
						Labels: map[string]string{v1beta1constants.ProjectName: "test-project"},
					},
				})).To(Succeed())
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-service-account-issuer",
						Namespace: v1beta1constants.GardenNamespace,
						Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShootServiceAccountIssuer},
					},
					Data: map[string][]byte{"hostname": []byte("issuer.example.com")},
				})).To(Succeed())
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": computedManagedIssuerAuthenticationConfiguration},
				})).To(Succeed())

				shootv1beta1.Annotations = map[string]string{v1beta1constants.AnnotationAuthenticationIssuer: v1beta1constants.AnnotationAuthenticationIssuerManaged}
				// No UID set - computed issuer should be skipped
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("allows auth config with different URL when shoot has managed issuer (CREATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   shootNamespace,
						Labels: map[string]string{v1beta1constants.ProjectName: "test-project"},
					},
				})).To(Succeed())
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-service-account-issuer",
						Namespace: v1beta1constants.GardenNamespace,
						Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShootServiceAccountIssuer},
					},
					Data: map[string][]byte{"hostname": []byte("issuer.example.com")},
				})).To(Succeed())
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": validAuthenticationConfiguration},
				})).To(Succeed())

				shootv1beta1.UID = types.UID("test-shoot-uid")
				shootv1beta1.Annotations = map[string]string{v1beta1constants.AnnotationAuthenticationIssuer: v1beta1constants.AnnotationAuthenticationIssuerManaged}
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})

			It("allows auth config when the default issuer URL is not used (CREATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": validAuthenticationConfiguration},
				})).To(Succeed())
				shootv1beta1.Status = gardencorev1beta1.ShootStatus{
					AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
						{
							Name: v1beta1constants.AdvertisedAddressInternal,
							URL:  "https://api.fake-shoot-name.test-project.internal.example.com",
						},
					},
				}
				test(admissionv1.Create, nil, shootv1beta1, true, statusCodeAllowed, "referenced authentication configuration is valid", "")
			})
		})

		Context("Deny", func() {
			It("references a configmap that does not exist", func() {
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "referenced authentication configuration ConfigMap fake-cm-namespace/fake-cm-name does not exist: configmaps \"fake-cm-name\" not found", "")
			})

			It("fails getting cm", func() {
				fakeErr := errors.New("fake")
				errClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*corev1.ConfigMap); ok {
							return fakeErr
						}
						return c.Get(ctx, key, obj, opts...)
					},
				}).Build()
				handler = NewHandler(errClient, fakeClient, decoder)
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInternalError, "could not retrieve authentication configuration ConfigMap fake-cm-namespace/fake-cm-name: fake", "")
			})

			It("references configmap without a config.yaml key", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       nil,
				})).To(Succeed())
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "error getting authentication configuration from ConfigMap fake-cm-namespace/fake-cm-name: missing authentication configuration key in config.yaml ConfigMap data", "")
			})

			It("references authentication configuration which breaks validation rules", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": invalidAuthenticationConfiguration},
				})).To(Succeed())
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.audiences: Required value: at least one jwt[0].issuer.audiences is required]", "")
			})

			It("references authentication configuration with invalid structure", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": missingKeyConfiguration},
				})).To(Succeed())
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "did not find expected key", "")
			})

			It("contains disallowed issuers in the service account config", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": invalidIssuerUrl},
				})).To(Succeed())
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.url: Invalid value: \"https://abc.com\": URL must not overlap with disallowed issuers:", "")
			})

			It("contains service account issuer from status advertised addresses in the authentication configuration", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": managedIssuerAuthenticationConfiguration},
				})).To(Succeed())
				shootv1beta1.Status = gardencorev1beta1.ShootStatus{
					AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
						{
							Name: v1beta1constants.AdvertisedAddressServiceAccountIssuer,
							URL:  "https://managed.example.com/projects/my-project/shoots/some-uid/issuer",
						},
					},
				}
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.url: Invalid value: \"https://managed.example.com/projects/my-project/shoots/some-uid/issuer\": URL must not overlap with disallowed issuers:", "")
			})

			It("denies authentication configuration containing the computed managed issuer URL (CREATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   shootNamespace,
						Labels: map[string]string{v1beta1constants.ProjectName: "test-project"},
					},
				})).To(Succeed())
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-service-account-issuer",
						Namespace: v1beta1constants.GardenNamespace,
						Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShootServiceAccountIssuer},
					},
					Data: map[string][]byte{"hostname": []byte("issuer.example.com")},
				})).To(Succeed())
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": computedManagedIssuerAuthenticationConfiguration},
				})).To(Succeed())

				shootv1beta1.UID = types.UID("test-shoot-uid")
				shootv1beta1.Annotations = map[string]string{v1beta1constants.AnnotationAuthenticationIssuer: v1beta1constants.AnnotationAuthenticationIssuerManaged}
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.url: Invalid value: \"https://issuer.example.com/projects/test-project/shoots/test-shoot-uid/issuer\": URL must not overlap with disallowed issuers:", "")
			})

			It("denies authentication configuration containing the default issuer URL from internal advertised address (CREATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": defaultIssuerAuthenticationConfiguration},
				})).To(Succeed())
				shootv1beta1.Status = gardencorev1beta1.ShootStatus{
					AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
						{
							Name: v1beta1constants.AdvertisedAddressInternal,
							URL:  "https://api.fake-shoot-name.test-project.internal.example.com",
						},
					},
				}
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.url: Invalid value: \"https://api.fake-shoot-name.test-project.internal.example.com\": URL must not overlap with disallowed issuers:", "")
			})

			It("denies authentication configuration containing the default issuer URL from external advertised address (CREATE)", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": externalIssuerAuthenticationConfiguration},
				})).To(Succeed())
				shootv1beta1.Status = gardencorev1beta1.ShootStatus{
					AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
						{
							Name: v1beta1constants.AdvertisedAddressExternal,
							URL:  "https://api.fake-shoot-name.test-project.external.example.com",
						},
					},
				}
				test(admissionv1.Create, nil, shootv1beta1, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.url: Invalid value: \"https://api.fake-shoot-name.test-project.external.example.com\": URL must not overlap with disallowed issuers:", "")
			})

			It("enables legacy anonymous authentication on the kube-apiserver when anonymous authentication configuration is already present", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: shootNamespace},
					Data:       map[string]string{"config.yaml": anonymousAuthenticationConfiguration},
				})).To(Succeed())
				newShoot := shootv1beta1.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication = new(false)
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
					shootv1beta1.Spec.Kubernetes.Version = "1.34.1"
					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = anotherValidAuthenticationConfiguration

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

				It("contains service account issuer from status advertised addresses in the authentication configuration", func() {
					shootv1beta1.Status = gardencorev1beta1.ShootStatus{
						AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
							{
								Name: v1beta1constants.AdvertisedAddressServiceAccountIssuer,
								URL:  "https://managed.example.com/projects/my-project/shoots/some-uid/issuer",
							},
						},
					}
					Expect(fakeClient.Update(ctx, shootv1beta1)).To(Succeed())

					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = managedIssuerAuthenticationConfiguration

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.url: Invalid value: \"https://managed.example.com/projects/my-project/shoots/some-uid/issuer\": URL must not overlap with disallowed issuers:", "")
				})

				It("denies ConfigMap update with computed managed issuer URL", func() {
					Expect(fakeClient.Create(ctx, &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name:   shootNamespace,
							Labels: map[string]string{v1beta1constants.ProjectName: "test-project"},
						},
					})).To(Succeed())
					Expect(fakeClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shoot-service-account-issuer",
							Namespace: v1beta1constants.GardenNamespace,
							Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShootServiceAccountIssuer},
						},
						Data: map[string][]byte{"hostname": []byte("issuer.example.com")},
					})).To(Succeed())

					shootv1beta1.UID = types.UID("test-shoot-uid")
					shootv1beta1.Annotations = map[string]string{v1beta1constants.AnnotationAuthenticationIssuer: v1beta1constants.AnnotationAuthenticationIssuerManaged}
					Expect(fakeClient.Update(ctx, shootv1beta1)).To(Succeed())

					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = computedManagedIssuerAuthenticationConfiguration

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "provided invalid authentication configuration: [jwt[0].issuer.url: Invalid value: \"https://issuer.example.com/projects/test-project/shoots/test-shoot-uid/issuer\": URL must not overlap with disallowed issuers:", "")
				})

				It("uses anonymous authentication and has the legacy kube-apiserver setting already enabled", func() {
					shootv1beta1.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication = new(true)
					Expect(fakeClient.Update(ctx, shootv1beta1)).To(Succeed())

					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = anonymousAuthenticationConfiguration

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "cannot use anonymous authentication configuration when the following shoots have the legacy configuration enabled: fake-shoot-name", "")
				})

				It("uses anonymous authentication and has the legacy kube-apiserver setting already enabled", func() {
					shootv1beta1.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication = new(false)
					Expect(fakeClient.Update(ctx, shootv1beta1)).To(Succeed())

					newCm := cm.DeepCopy()
					newCm.Data["config.yaml"] = anonymousAuthenticationConfiguration

					test(admissionv1.Update, cm, newCm, false, statusCodeInvalid, "cannot use anonymous authentication configuration when the following shoots have the legacy configuration enabled: fake-shoot-name", "")
				})
			})
		})
	})
})
