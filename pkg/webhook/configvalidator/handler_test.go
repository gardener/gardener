// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package configvalidator_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/webhook/configvalidator"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.Background()

		fakeClient client.Client
		encoder    runtime.Encoder
		decoder    admission.Decoder

		configMap *corev1.ConfigMap
		shoot     *gardencorev1beta1.Shoot
		garden    *operatorv1alpha1.Garden

		handler *Handler
		request admission.Request
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
		utilruntime.Must(gardencorev1beta1.AddToScheme(scheme))
		utilruntime.Must(corev1.AddToScheme(scheme))

		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		encoder = &jsonserializer.Serializer{}
		decoder = admission.NewDecoder(scheme)

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "config-name",
				Namespace: "shoot-namespace",
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-name",
				Namespace: configMap.Namespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.30.0",
				},
			},
		}
		garden = &operatorv1alpha1.Garden{
			TypeMeta: metav1.TypeMeta{
				APIVersion: operatorv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Garden",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden-name",
			},
			Spec: operatorv1alpha1.GardenSpec{
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.30.0",
					},
					Gardener: operatorv1alpha1.Gardener{
						APIServer: &operatorv1alpha1.GardenerAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: configMap.Name,
									},
								},
							},
						},
					},
				},
			},
		}

		handler = &Handler{
			APIReader: fakeClient,
			Client:    fakeClient,
			Decoder:   decoder,

			ConfigMapPurpose: "test config",
			ConfigMapDataKey: "config.yaml",
			GetConfigMapNameFromShoot: func(_ *core.Shoot) string {
				return configMap.Name
			},
			GetNamespace: func() string {
				return configMap.Namespace
			},
			GetConfigMapNameFromGarden: func(garden *operatorv1alpha1.Garden) map[string]string {
				configMapNames := map[string]string{}
				if garden.Spec.VirtualCluster.Gardener.APIServer != nil && garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig != nil &&
					garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy != nil && garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
					configMapNames["gardener-apiserver-audit-policy"] = garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name
				}

				if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer != nil && garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditConfig != nil &&
					garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy != nil && garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
					configMapNames["virtual-garden-kube-apiserver-audit-policy"] = garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name
				}
				return configMapNames
			},
		}
		request = admission.Request{}

		Expect(fakeClient.Create(ctx, configMap)).To(Succeed())
	})

	Context("Shoots", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "Shoot"}

			rawShoot, err := runtime.Encode(encoder, shoot)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawShoot
		})

		It("should return an error because it cannot decode the object", func() {
			request.Object = runtime.RawExtension{Raw: []byte(`{`)}

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(500)))
			Expect(response.Result.Message).To(ContainSubstring("couldn't get version/kind; json parse error: unexpected end of JSON input"))
		})

		It("should allow since the shoot has a deletion timestamp", func() {
			shoot.DeletionTimestamp = ptr.To(metav1.Now())

			rawShoot, err := runtime.Encode(encoder, shoot)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawShoot

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("shoot is already marked for deletion"))
		})

		It("should allow because the shoot does not specify a config map", func() {
			handler.GetConfigMapNameFromShoot = func(_ *core.Shoot) string { return "" }

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("Shoot resource does not specify any test config ConfigMap"))
		})

		It("should allow on update because the specification does not have relevant changes", func() {
			request.Operation = admissionv1.Update
			shoot.Spec.Region = "foo"

			rawShoot, err := runtime.Encode(encoder, shoot)
			Expect(err).NotTo(HaveOccurred())
			request.OldObject.Raw = rawShoot

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("Neither test config ConfigMap nor Kubernetes version or other relevant fields were changed"))
		})

		It("should allow on update because the custom skip-validation function returns true", func() {
			request.Operation = admissionv1.Update
			shoot.Spec.Region = "foo"

			handler.SkipValidationOnShootUpdate = func(_, _ *core.Shoot) bool { return true }

			rawShoot, err := runtime.Encode(encoder, shoot)
			Expect(err).NotTo(HaveOccurred())
			request.OldObject.Raw = rawShoot

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("Neither test config ConfigMap nor Kubernetes version or other relevant fields were changed"))
		})

		It("should return an error because the ConfigMap cannot be read", func() {
			Expect(fakeClient.Delete(ctx, configMap)).To(Succeed())

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("referenced test config ConfigMap shoot-namespace/config-name does not exist: configmaps \"config-name\" not found"))
		})

		It("should return an error because the ConfigMap does not contain the data key", func() {
			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("error getting test config from ConfigMap shoot-namespace/config-name: missing test config key in config.yaml ConfigMap data"))
		})

		It("should return an error because the data key in the ConfigMap is empty", func() {
			configMap.Data = map[string]string{"config.yaml": ""}
			Expect(fakeClient.Update(ctx, configMap)).To(Succeed())

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("error getting test config from ConfigMap shoot-namespace/config-name: test config in config.yaml key is empty"))
		})

		It("should return an error because the admit function returns an error", func() {
			configMap.Data = map[string]string{"config.yaml": "foo"}
			Expect(fakeClient.Update(ctx, configMap)).To(Succeed())

			handler.AdmitConfig = func(_ string, _ []*core.Shoot) (int32, error) { return 1337, fmt.Errorf("fake error") }

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(1337)))
			Expect(response.Result.Message).To(ContainSubstring("fake error"))
		})

		It("should allow because the admit function does not return an error", func() {
			configMap.Data = map[string]string{"config.yaml": "foo"}
			Expect(fakeClient.Update(ctx, configMap)).To(Succeed())

			handler.AdmitConfig = func(_ string, _ []*core.Shoot) (int32, error) { return 0, nil }

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("referenced test config is valid"))
		})
	})

	Context("ConfigMaps", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
			request.Operation = admissionv1.Update
			request.Name = configMap.Name

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap
		})

		It("should allow because the operation is not update", func() {
			request.Operation = admissionv1.Create

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("operation is not update, nothing to validate"))
		})

		It("should return an error because it cannot decode the object", func() {
			request.Object = runtime.RawExtension{Raw: []byte(`{`)}

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(500)))
			Expect(response.Result.Message).To(ContainSubstring("couldn't get version/kind; json parse error: unexpected end of JSON input"))
		})

		It("should allow because the ConfigMap is not referenced by any shoots", func() {
			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("ConfigMap is not referenced by a Shoot"))
		})

		It("should return an error because the ConfigMap does not contain the data key", func() {
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("error getting test config from ConfigMap shoot-namespace/config-name: missing test config key in config.yaml ConfigMap data"))
		})

		It("should return an error because the data key in the ConfigMap is empty", func() {
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": ""}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("error getting test config from ConfigMap shoot-namespace/config-name: test config in config.yaml key is empty"))
		})

		It("should return an error because it cannot decode the old object", func() {
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": "foo"}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap

			request.OldObject = runtime.RawExtension{Raw: []byte(`{`)}

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(500)))
			Expect(response.Result.Message).To(ContainSubstring("couldn't get version/kind; json parse error: unexpected end of JSON input"))
		})

		It("should allow because the config did not change", func() {
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": "foo"}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap
			request.OldObject.Raw = rawConfigMap

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("test config did not change"))
		})

		It("should return an error because the admit function returns an error", func() {
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": "foo"}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap

			configMap.Data = map[string]string{"config.yaml": "bar"}

			rawConfigMap, err = runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.OldObject.Raw = rawConfigMap

			handler.AdmitConfig = func(_ string, _ []*core.Shoot) (int32, error) { return 1337, fmt.Errorf("fake error") }

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(1337)))
			Expect(response.Result.Message).To(ContainSubstring("fake error"))
		})

		It("should allow because the admit function does not return an error", func() {
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": "foo"}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap

			configMap.Data = map[string]string{"config.yaml": "bar"}

			rawConfigMap, err = runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.OldObject.Raw = rawConfigMap

			handler.AdmitConfig = func(_ string, _ []*core.Shoot) (int32, error) { return 0, nil }

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("referenced test config is valid"))
		})
	})

	Context("Gardens", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "operator.gardener.cloud", Version: "v1alpha1", Kind: "Garden"}

			rawGarden, err := runtime.Encode(encoder, garden)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawGarden

			// Set up handler for Garden resources
			handler.GetConfigMapNameFromShoot = nil // Clear shoot handler
			handler.AdmitGardenConfig = func(_ string) (int32, error) { return 0, nil }
		})

		It("should return an error because it cannot decode the object", func() {
			request.Object = runtime.RawExtension{Raw: []byte(`{`)}

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(500)))
			Expect(response.Result.Message).To(ContainSubstring("couldn't get version/kind; json parse error: unexpected end of JSON input"))
		})

		It("should allow since the garden has a deletion timestamp", func() {
			garden.DeletionTimestamp = ptr.To(metav1.Now())

			rawGarden, err := runtime.Encode(encoder, garden)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawGarden

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("garden is already marked for deletion"))
		})

		It("should allow because the garden does not specify a config map", func() {
			garden.Spec.VirtualCluster.Gardener.APIServer = nil

			rawGarden, err := runtime.Encode(encoder, garden)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawGarden

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("no audit policy config map reference found in garden spec"))
		})

		It("should allow on update because the specification was not changed", func() {
			request.Operation = admissionv1.Update

			rawGarden, err := runtime.Encode(encoder, garden)
			Expect(err).NotTo(HaveOccurred())
			request.OldObject.Raw = rawGarden

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("garden spec was not changed"))
		})

		It("should allow on update because config map reference and kubernetes version were not changed", func() {
			request.Operation = admissionv1.Update

			oldGarden := garden.DeepCopy()
			garden.Labels = map[string]string{"foo": "bar"} // Change only labels

			rawOldGarden, err := runtime.Encode(encoder, oldGarden)
			Expect(err).NotTo(HaveOccurred())
			request.OldObject.Raw = rawOldGarden

			rawGarden, err := runtime.Encode(encoder, garden)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawGarden

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("garden spec was not changed"))
		})

		It("should return an error because the ConfigMap cannot be read", func() {
			Expect(fakeClient.Delete(ctx, configMap)).To(Succeed())

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("referenced ConfigMap shoot-namespace/config-name does not exist: configmaps \"config-name\" not found"))
		})

		It("should return an error because the ConfigMap does not contain the data key", func() {
			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("error getting ConfigMap shoot-namespace/config-name: missing test config key in config.yaml ConfigMap data"))
		})

		It("should return an error because the data key in the ConfigMap is empty", func() {
			configMap.Data = map[string]string{"config.yaml": ""}
			Expect(fakeClient.Update(ctx, configMap)).To(Succeed())

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("error getting ConfigMap shoot-namespace/config-name: test config in config.yaml key is empty"))
		})

		It("should return an error because the admit function returns an error", func() {
			configMap.Data = map[string]string{"config.yaml": "foo"}
			Expect(fakeClient.Update(ctx, configMap)).To(Succeed())

			handler.AdmitGardenConfig = func(_ string) (int32, error) { return 1337, fmt.Errorf("fake error") }

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(1337)))
			Expect(response.Result.Message).To(ContainSubstring("fake error"))
		})

		It("should allow because the admit function does not return an error", func() {
			configMap.Data = map[string]string{"config.yaml": "foo"}
			Expect(fakeClient.Update(ctx, configMap)).To(Succeed())

			handler.AdmitGardenConfig = func(_ string) (int32, error) { return 0, nil }

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("all referenced configMaps are valid"))
		})

		It("should validate multiple ConfigMaps when both gardener and kube-apiserver audit policies are configured", func() {
			// Create second ConfigMap for kube-apiserver
			kubeAPIServerConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-apiserver-audit-policy",
					Namespace: configMap.Namespace,
				},
				Data: map[string]string{"config.yaml": "kube-apiserver-config"},
			}
			Expect(fakeClient.Create(ctx, kubeAPIServerConfigMap)).To(Succeed())

			// Update garden to reference both ConfigMaps
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{
				KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{
					AuditConfig: &gardencorev1beta1.AuditConfig{
						AuditPolicy: &gardencorev1beta1.AuditPolicy{
							ConfigMapRef: &corev1.ObjectReference{
								Name: kubeAPIServerConfigMap.Name,
							},
						},
					},
				},
			}

			configMap.Data = map[string]string{"config.yaml": "gardener-apiserver-config"}
			Expect(fakeClient.Update(ctx, configMap)).To(Succeed())

			rawGarden, err := runtime.Encode(encoder, garden)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawGarden

			validationCalls := 0
			handler.AdmitGardenConfig = func(configRaw string) (int32, error) {
				validationCalls++
				if configRaw != "gardener-apiserver-config" && configRaw != "kube-apiserver-config" {
					return 1337, fmt.Errorf("unexpected config content: %s", configRaw)
				}
				return 0, nil
			}

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("all referenced configMaps are valid"))
			Expect(validationCalls).To(Equal(2))
		})
	})

	Context("ConfigMaps for Gardens", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
			request.Operation = admissionv1.Update
			request.Name = configMap.Name
			request.Namespace = configMap.Namespace

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap

			// Set up handler for Garden ConfigMap resources
			handler.GetConfigMapNameFromShoot = nil // Clear shoot handler
			handler.AdmitGardenConfig = func(_ string) (int32, error) { return 0, nil }
		})

		It("should allow because the operation is not update", func() {
			request.Operation = admissionv1.Create

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("operation is not update, nothing to validate"))
		})

		It("should return an error because it cannot decode the object", func() {
			request.Object = runtime.RawExtension{Raw: []byte(`{`)}

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(500)))
			Expect(response.Result.Message).To(ContainSubstring("couldn't get version/kind; json parse error: unexpected end of JSON input"))
		})

		It("should allow because no garden resources found", func() {
			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("no garden resources found, nothing to validate"))
		})

		It("should allow because the ConfigMap is not referenced by garden", func() {
			// Create a garden that doesn't reference this ConfigMap
			gardenNotReferencing := garden.DeepCopy()
			gardenNotReferencing.Spec.VirtualCluster.Gardener.APIServer = nil
			Expect(fakeClient.Create(ctx, gardenNotReferencing)).To(Succeed())

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("config map is not referenced by garden resource, nothing to validate"))
		})

		It("should return an error because the ConfigMap does not contain the data key", func() {
			Expect(fakeClient.Create(ctx, garden)).To(Succeed())

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("error getting test config from ConfigMap shoot-namespace/config-name: missing test config key in config.yaml ConfigMap data"))
		})

		It("should return an error because the data key in the ConfigMap is empty", func() {
			Expect(fakeClient.Create(ctx, garden)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": ""}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(422)))
			Expect(response.Result.Message).To(ContainSubstring("error getting test config from ConfigMap shoot-namespace/config-name: test config in config.yaml key is empty"))
		})

		It("should return an error because it cannot decode the old object", func() {
			Expect(fakeClient.Create(ctx, garden)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": "foo"}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap

			request.OldObject = runtime.RawExtension{Raw: []byte(`{`)}

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(500)))
			Expect(response.Result.Message).To(ContainSubstring("couldn't get version/kind; json parse error: unexpected end of JSON input"))
		})

		It("should allow because the config did not change", func() {
			Expect(fakeClient.Create(ctx, garden)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": "foo"}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap
			request.OldObject.Raw = rawConfigMap

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("test config did not change"))
		})

		It("should return an error because the admit function returns an error", func() {
			Expect(fakeClient.Create(ctx, garden)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": "foo"}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap

			configMap.Data = map[string]string{"config.yaml": "bar"}

			rawConfigMap, err = runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.OldObject.Raw = rawConfigMap

			handler.AdmitGardenConfig = func(_ string) (int32, error) { return 1337, fmt.Errorf("fake error") }

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(1337)))
			Expect(response.Result.Message).To(ContainSubstring("fake error"))
		})

		It("should allow because the admit function does not return an error", func() {
			Expect(fakeClient.Create(ctx, garden)).To(Succeed())

			configMap.Data = map[string]string{"config.yaml": "foo"}

			rawConfigMap, err := runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = rawConfigMap

			configMap.Data = map[string]string{"config.yaml": "bar"}

			rawConfigMap, err = runtime.Encode(encoder, configMap)
			Expect(err).NotTo(HaveOccurred())
			request.OldObject.Raw = rawConfigMap

			handler.AdmitGardenConfig = func(_ string) (int32, error) { return 0, nil }

			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("referenced test config is valid"))
		})
	})

	Context("unrelated resource", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}
		})

		It("should allow since the resource is not handled", func() {
			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("resource is neither of type core.gardener.cloud/v1beta1.Shoot, operator.gardener.cloud/v1alpha1.Garden, nor corev1.ConfigMap"))
		})
	})
})
