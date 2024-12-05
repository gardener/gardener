// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/types/helm"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerDeployment roundtrip conversion", func() {
	var (
		scheme          *runtime.Scheme
		expectedV1      *gardencorev1.ControllerDeployment
		expectedV1beta1 *gardencorev1beta1.ControllerDeployment
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		gardencoreinstall.Install(scheme)
	})

	// convert converts in to out via the internal API version
	convert := func(scheme *runtime.Scheme, in, out runtime.Object) {
		GinkgoHelper()

		internal := &gardencore.ControllerDeployment{}
		Expect(scheme.Convert(in, internal, nil)).To(Succeed())
		Expect(scheme.Convert(internal, out, nil)).To(Succeed())
	}

	Context("helm type", func() {
		BeforeEach(func() {
			rawChart := []byte("foo")
			values := `{"foo":{"bar":"baz"},"boom":42}`
			valuesJSON := &helm.Values{
				Raw: []byte(values),
			}

			expectedV1 = &gardencorev1.ControllerDeployment{
				Helm: &gardencorev1.HelmControllerDeployment{
					RawChart: rawChart,
					Values:   valuesJSON.DeepCopy(),
				},
			}

			expectedV1beta1 = &gardencorev1beta1.ControllerDeployment{
				Type: "helm",
				ProviderConfig: runtime.RawExtension{
					Raw: []byte(`{"chart":"` + base64.StdEncoding.EncodeToString(rawChart) + `","values":` + values + `}`),
				},
			}
		})

		It("should perform lossless roundtrip conversion", func() {
			By("converting from v1 to v1beta1")
			actualV1beta1 := &gardencorev1beta1.ControllerDeployment{}
			convert(scheme, expectedV1, actualV1beta1)
			Expect(actualV1beta1).To(DeepEqual(expectedV1beta1))

			By("converting from v1beta1 to v1")
			actualV1 := &gardencorev1.ControllerDeployment{}
			convert(scheme, actualV1beta1, actualV1)
			Expect(actualV1).To(DeepEqual(expectedV1))
		})
	})

	Context("helm type with OCI", func() {
		BeforeEach(func() {
			values := `{"foo":{"bar":"baz"},"boom":42}`
			valuesJSON := &helm.Values{
				Raw: []byte(values),
			}

			expectedV1 = &gardencorev1.ControllerDeployment{
				Helm: &gardencorev1.HelmControllerDeployment{
					Values: valuesJSON.DeepCopy(),
					OCIRepository: &gardencorev1.OCIRepository{
						Ref: ptr.To("foo:1.0.0"),
					},
				},
			}

			expectedV1beta1 = &gardencorev1beta1.ControllerDeployment{
				Type: "helm",
				ProviderConfig: runtime.RawExtension{
					Raw: []byte(`{"values":` + values + `,"ociRepository":{"ref":"foo:1.0.0"}}`),
				},
			}
		})

		It("should perform lossless roundtrip conversion", func() {
			By("converting from v1 to v1beta1")
			actualV1beta1 := &gardencorev1beta1.ControllerDeployment{}
			convert(scheme, expectedV1, actualV1beta1)
			Expect(actualV1beta1).To(DeepEqual(expectedV1beta1))

			By("converting from v1beta1 to v1")
			actualV1 := &gardencorev1.ControllerDeployment{}
			convert(scheme, actualV1beta1, actualV1)
			Expect(actualV1).To(DeepEqual(expectedV1))
		})
	})

	Context("custom type", func() {
		BeforeEach(func() {
			deploymentType := "custom"
			providerConfig := `{"foo":{"bar":"baz"},"boom":42}`

			expectedV1 = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"migration.controllerdeployment.gardener.cloud/type":           deploymentType,
						"migration.controllerdeployment.gardener.cloud/providerConfig": providerConfig,
					},
				},
			}

			expectedV1beta1 = &gardencorev1beta1.ControllerDeployment{
				Type: deploymentType,
				ProviderConfig: runtime.RawExtension{
					Raw: []byte(providerConfig),
				},
			}
		})

		It("should perform lossless roundtrip conversion", func() {
			By("converting from v1 to v1beta1")
			actualV1beta1 := &gardencorev1beta1.ControllerDeployment{}
			convert(scheme, expectedV1, actualV1beta1)
			Expect(actualV1beta1).To(DeepEqual(expectedV1beta1))

			By("converting from v1beta1 to v1")
			actualV1 := &gardencorev1.ControllerDeployment{}
			convert(scheme, actualV1beta1, actualV1)
			Expect(actualV1).To(DeepEqual(expectedV1))
		})
	})
})
