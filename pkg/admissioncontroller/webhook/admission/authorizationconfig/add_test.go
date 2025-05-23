// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authorizationconfig_test

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

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/authorizationconfig"
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

		configMapName      = "fake-cm-name"
		configMapNameOther = "fake-cm-name-other"
		configMapNamespace = "fake-cm-namespace"
		shootName          = "fake-shoot-name"
		shootNamespace     = configMapNamespace

		configMap *corev1.ConfigMap
		shoot     *gardencorev1beta1.Shoot

		validAuthorizationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthorizationConfiguration
authorizers:
- type: Webhook
  name: webhook
  webhook:
    timeout: 3s
    subjectAccessReviewVersion: v1
    matchConditionSubjectAccessReviewVersion: v1
    failurePolicy: Deny
    matchConditions:
    - expression: request.resourceAttributes.namespace == 'kube-system'
`
		anotherValidAuthorizationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthorizationConfiguration
authorizers:
- type: Webhook
  name: webhook
  webhook:
    timeout: 5s
    subjectAccessReviewVersion: v1
    matchConditionSubjectAccessReviewVersion: v1
    failurePolicy: Deny
`
		missingKeyConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthorizationConfiguration
authorizers:
- type: Webhook
  name: webhook
  webhook:
    timeout: 5s
    matchConditionSubjectAccessReviewVersion: v1
    failurePolicy: Deny
`
		invalidAuthorizationConfiguration = `
---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthorizationConfiguration
authorizers:
- type: Webhook
  name: webhook
  webhook:
    timeout: 5s
    subjectAccessReviewVersion: v1
    matchConditionSubjectAccessReviewVersion: v1
    failurePolicy: Deny
    matchConditions:
    - expression: !('system:serviceaccounts:kube-system' in request.user.groups)
    connectionInfo:
      type: InCluster
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
					Version: "1.30.0",
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
							ConfigMapName: configMapName,
							Kubeconfigs:   []gardencorev1beta1.AuthorizerKubeconfigReference{{AuthorizerName: "webhook"}},
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
		if expectedMsg != "" {
			ExpectWithOffset(1, response.Result.Message).To(ContainSubstring(expectedMsg))
		}
		if expectedReason != "" {
			ExpectWithOffset(1, string(response.Result.Reason)).To(ContainSubstring(expectedReason))
		}
		ExpectWithOffset(1, response.Result.Code).To(Equal(expectedStatusCode))
		ExpectWithOffset(1, response.Allowed).To(Equal(expectedAllowed))
		ExpectWithOffset(1, response.Patches).To(BeEmpty())
	}

	Context("Shoots", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "Shoot"}
		})

		Context("Allow", func() {
			It("has no KubeAPIServer config", func() {
				shoot.Spec.Kubernetes.KubeAPIServer = nil
				test(admissionv1.Create, nil, shoot, true, statusCodeAllowed, "Shoot resource does not specify any authorization configuration ConfigMap", "")
			})

			It("has no Structured Authorization", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = nil
				test(admissionv1.Create, nil, shoot, true, statusCodeAllowed, "Shoot resource does not specify any authorization configuration ConfigMap", "")
			})

			It("has no authorization configuration ConfigMap Ref", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.ConfigMapName = ""
				test(admissionv1.Create, nil, shoot, true, statusCodeAllowed, "Shoot resource does not specify any authorization configuration ConfigMap", "")
			})

			It("references a valid AuthorizationConfig (CREATE)", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
					Data:       map[string]string{"config.yaml": validAuthorizationConfiguration},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shoot, true, statusCodeAllowed, "referenced authorization configuration is valid", "")
			})

			It("AuthorizationConfig name was added (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"config.yaml": validAuthorizationConfiguration},
				}
				apiServerConfig := shoot.Spec.Kubernetes.KubeAPIServer.DeepCopy()
				shoot.Spec.Kubernetes.KubeAPIServer = nil
				newShoot := shoot.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer = apiServerConfig
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shoot, newShoot, true, statusCodeAllowed, "referenced authorization configuration is valid", "")
			})

			It("referenced AuthorizationConfig name was changed (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"config.yaml": validAuthorizationConfiguration},
				}
				newShoot := shoot.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.ConfigMapName = configMapNameOther
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapNameOther}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shoot, newShoot, true, statusCodeAllowed, "referenced authorization configuration is valid", "")
			})

			It("referenced AuthorizationConfig name was removed (UPDATE)", func() {
				newShoot := shoot.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer = nil
				test(admissionv1.Update, shoot, newShoot, true, statusCodeAllowed, "Shoot resource does not specify any authorization configuration ConfigMap", "")
			})

			It("should not validate AuthorizationConfig if already marked for deletion (UPDATE)", func() {
				now := metav1.Now()
				shoot.DeletionTimestamp = &now
				newShoot := shoot.DeepCopy()
				newShoot.Labels = map[string]string{
					"foo": "bar",
				}
				test(admissionv1.Update, shoot, newShoot, true, statusCodeAllowed, "marked for deletion", "")
			})

			It("should not validate AuthorizationConfig if spec wasn't changed (UPDATE)", func() {
				newShoot := shoot.DeepCopy()
				newShoot.Labels = map[string]string{
					"foo": "bar",
				}
				test(admissionv1.Update, shoot, newShoot, true, statusCodeAllowed, "shoot spec was not changed", "")
			})
		})

		Context("Deny", func() {
			It("references a ConfigMap that does not exist", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ *corev1.ConfigMap, _ ...client.GetOption) error {
					return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, configMapName)
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInvalid, "referenced authorization configuration ConfigMap fake-cm-namespace/fake-cm-name does not exist: configmaps \"fake-cm-name\" not found", "")
			})

			It("fails getting ConfigMap", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ *corev1.ConfigMap, _ ...client.GetOption) error {
					return errors.New("fake")
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInternalError, "could not retrieve authorization configuration ConfigMap fake-cm-namespace/fake-cm-name: fake", "")
			})

			It("references ConfigMap without a config.yaml key", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = corev1.ConfigMap{
						Data: nil,
					}
					return nil
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInvalid, "error getting authorization configuration from ConfigMap /: missing authorization configuration key in config.yaml ConfigMap data", "")
			})

			It("references ConfigMap with a webhook for which no kubeconfig secret name is specified (CREATE)", func() {
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = corev1.ConfigMap{
						Data: map[string]string{"config.yaml": validAuthorizationConfiguration},
					}
					return nil
				})
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs = nil
				test(admissionv1.Create, nil, shoot, false, statusCodeInvalid, `provided invalid authorization configuration: must provide kubeconfig secret name reference for webhook authorizer "webhook"`, "")
			})

			It("references ConfigMap with a webhook for which no kubeconfig secret name is specified (UPDATE)", func() {
				returnedCm := corev1.ConfigMap{
					Data: map[string]string{"config.yaml": validAuthorizationConfiguration},
				}
				newShoot := shoot.DeepCopy()
				newShoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs[0].AuthorizerName = "does-not-exist"
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Update, shoot, newShoot, false, statusCodeInvalid, "provided invalid authorization configuration: must provide kubeconfig secret name reference for webhook authorizer \"webhook\" in shoot fake-cm-namespace/fake-shoot-name", "")
			})

			It("references authorization configuration which breaks validation rules", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"config.yaml": invalidAuthorizationConfiguration},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInvalid, "provided invalid authorization configuration: [authorizers[0].connectionInfo: Forbidden: connectionInfo is not allowed to be set]", "")
			})

			It("references authorization configuration with invalid structure", func() {
				returnedCm := corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
					Data:       map[string]string{"config.yaml": missingKeyConfiguration},
				}
				mockReader.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: shootNamespace, Name: configMapName}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = returnedCm
					return nil
				})
				test(admissionv1.Create, nil, shoot, false, statusCodeInvalid, "provided invalid authorization configuration: [[].authorizers[0].subjectAccessReviewVersion: Required value]", "")
			})
		})
	})

	Context("ConfigMaps", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}

			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: configMapNamespace,
				},
				Data: map[string]string{"config.yaml": validAuthorizationConfiguration},
			}
		})

		Context("Update", func() {
			BeforeEach(func() {
				request.Name = configMapName
				request.Namespace = configMapNamespace
			})

			Context("Allow", func() {
				It("is not reference by any shoot", func() {
					shootInSameNamespaceButNotReferencing := shoot.DeepCopy()
					shootInSameNamespaceButNotReferencing.Spec.Kubernetes.KubeAPIServer = nil
					Expect(fakeClient.Create(ctx, shootInSameNamespaceButNotReferencing)).To(Succeed())
					shootInDifferentNamespaceAndReferencing := shoot.DeepCopy()
					shootInDifferentNamespaceAndReferencing.Namespace = shootNamespace + "other"
					Expect(fakeClient.Create(ctx, shootInDifferentNamespaceAndReferencing)).To(Succeed())

					test(admissionv1.Update, configMap, configMap, true, statusCodeAllowed, "ConfigMap is not referenced by a Shoot", "")
				})

				It("did not change config.yaml field", func() {
					Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
					test(admissionv1.Update, configMap, configMap, true, statusCodeAllowed, "authorization configuration did not change", "")
				})

				It("should allow if the AuthorizationConfig is changed to something valid", func() {
					Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
					shoot.Spec.Kubernetes.Version = "1.30.1"
					newCm := configMap.DeepCopy()
					newCm.Data["config.yaml"] = anotherValidAuthorizationConfiguration

					test(admissionv1.Update, configMap, newCm, true, statusCodeAllowed, "referenced authorization configuration is valid", "")
				})

				It("should allow if the AuthorizationConfig is changed to something valid even when multiple shoots reference it", func() {
					newCm := configMap.DeepCopy()
					newCm.Data["config.yaml"] = anotherValidAuthorizationConfiguration

					shoot2 := shoot.DeepCopy()
					shoot2.Name = "shoot2"
					shoot2.ResourceVersion = ""
					Expect(fakeClient.Create(ctx, shoot2)).To(Succeed())

					test(admissionv1.Update, configMap, newCm, true, statusCodeAllowed, "referenced authorization configuration is valid", "")
				})
			})

			Context("Deny", func() {
				BeforeEach(func() {
					Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
				})

				It("has no data key", func() {
					newCm := configMap.DeepCopy()
					newCm.Data = nil
					test(admissionv1.Update, configMap, newCm, false, statusCodeInvalid, "error getting authorization configuration from ConfigMap fake-cm-namespace/fake-cm-name: missing authorization configuration key in config.yaml ConfigMap data", "")
				})

				It("has empty config.yaml", func() {
					newCm := configMap.DeepCopy()
					newCm.Data["config.yaml"] = ""
					test(admissionv1.Update, configMap, newCm, false, statusCodeInvalid, "error getting authorization configuration from ConfigMap fake-cm-namespace/fake-cm-name: authorization configuration in config.yaml key is empty", "")
				})

				It("holds authorization configuration which breaks validation rules", func() {
					newCm := configMap.DeepCopy()
					newCm.Data["config.yaml"] = invalidAuthorizationConfiguration

					test(admissionv1.Update, configMap, newCm, false, statusCodeInvalid, "provided invalid authorization configuration: [authorizers[0].connectionInfo: Forbidden: connectionInfo is not allowed to be set]", "")
				})

				It("holds authorization configuration with invalid YAML structure", func() {
					newCm := configMap.DeepCopy()
					newCm.Data["config.yaml"] = missingKeyConfiguration

					test(admissionv1.Update, configMap, newCm, false, statusCodeInvalid, "provided invalid authorization configuration: [[].authorizers[0].subjectAccessReviewVersion: Required value]", "")
				})

				It("references ConfigMap with a webhook for which no kubeconfig secret name is specified", func() {
					newCm := configMap.DeepCopy()
					newCm.Data["config.yaml"] = anotherValidAuthorizationConfiguration

					shoot2 := shoot.DeepCopy()
					shoot2.Name = "shoot2"
					shoot2.ResourceVersion = ""
					shoot2.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs = nil
					Expect(fakeClient.Create(ctx, shoot2)).To(Succeed())

					test(admissionv1.Update, configMap, newCm, false, statusCodeInvalid, "provided invalid authorization configuration: must provide kubeconfig secret name reference for webhook authorizer \"webhook\" in shoot fake-cm-namespace/shoot2", "")
				})
			})
		})
	})
})
