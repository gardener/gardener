// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package configvalidator_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/webhook/configvalidator"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.Background()

		log        logr.Logger
		fakeClient client.Client
		encoder    runtime.Encoder
		decoder    admission.Decoder

		configMap *corev1.ConfigMap
		shoot     *gardencorev1beta1.Shoot

		handler *Handler
		request admission.Request
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		encoder = &jsonserializer.Serializer{}
		decoder = admission.NewDecoder(kubernetes.GardenScheme)

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

		handler = &Handler{
			Logger:    log,
			APIReader: fakeClient,
			Client:    fakeClient,
			Decoder:   decoder,

			ConfigMapKind:    "test config",
			ConfigMapDataKey: "config.yaml",
			GetConfigMapNameFromShoot: func(_ *core.Shoot) string {
				return configMap.Name
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

	Context("unrelated resource", func() {
		BeforeEach(func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}
		})

		It("should allow since the resource is not handled", func() {
			response := handler.Handle(ctx, request)
			Expect(response.Allowed).To(BeTrue())
			Expect(response.Result.Code).To(Equal(int32(200)))
			Expect(response.Result.Message).To(Equal("resource is neither of type *core.gardener.cloud/v1beta1.Shoot nor *corev1.ConfigMap"))
		})
	})
})
