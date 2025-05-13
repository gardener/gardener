// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"encoding/base64"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Resources", func() {
	Describe("#PrepareReferencedResourcesForSeedCopy", func() {
		var (
			ctx             context.Context
			fakeClient      client.Client
			resources       []gardencorev1beta1.NamedResourceReference
			sourceNamespace string
			targetNamespace string

			annotations map[string]string
			labels      map[string]string

			secret    *corev1.Secret
			configMap *corev1.ConfigMap
		)

		BeforeEach(func() {
			ctx = context.Background()
			fakeClient = fake.NewClientBuilder().Build()
			sourceNamespace = "garden"
			targetNamespace = "garden--project--shoot"

			annotations = map[string]string{
				"annotationKey": "annotationValue",
				"kubectl.kubernetes.io/last-applied-configuration": "last-applied-configuration",
			}

			labels = map[string]string{
				"labelKey": "labelValue",
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-secret",
					Namespace:   sourceNamespace,
					Annotations: annotations,
					Labels:      labels,
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-configmap",
					Namespace:   sourceNamespace,
					Finalizers:  []string{"finalizer"},
					Annotations: annotations,
					Labels:      labels,
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			Expect(fakeClient.Create(ctx, configMap)).To(Succeed())

			resources = []gardencorev1beta1.NamedResourceReference{
				{
					Name: "referenced-test-secret",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       secret.Name,
					},
				},
				{
					Name: "referenced-test-configmap",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Name:       configMap.Name,
					},
				},
			}
		})

		It("should prepare unstructured objects correctly", func() {
			unstructuredObjs, err := PrepareReferencedResourcesForSeedCopy(ctx, fakeClient, resources, sourceNamespace, targetNamespace)

			Expect(unstructuredObjs).To(HaveLen(2))
			Expect(err).NotTo(HaveOccurred())

			for _, unstructuredObj := range unstructuredObjs {
				Expect(unstructuredObj.GetNamespace()).To(Equal(targetNamespace), unstructuredObj.GetName()+" should have target namespace "+targetNamespace)
				Expect(unstructuredObj.GetAnnotations()).To(BeEmpty(), unstructuredObj.GetName()+" should have no annotations")
				Expect(unstructuredObj.GetLabels()).To(BeEmpty(), unstructuredObj.GetName()+" should have no labels")
				Expect(unstructuredObj.GetFinalizers()).To(BeEmpty(), unstructuredObj.GetName()+" should have no finalizers")
				Expect(unstructuredObj.Object).To(HaveKey("data"), unstructuredObj.GetName()+" should have data field")

				switch unstructuredObj.GetKind() {
				case "Secret":
					Expect(unstructuredObj.GetName()).To(Equal("ref-" + secret.Name))
					Expect(unstructuredObj.Object["data"]).To(WithTransform(func(data interface{}) (map[string][]byte, error) {
						byteMap := make(map[string][]byte)
						for k, v := range data.(map[string]interface{}) {
							byteMap[k], err = base64.StdEncoding.DecodeString(v.(string))
							if err != nil {
								return nil, err
							}
						}
						return byteMap, nil
					}, Equal(secret.Data)))
				case "ConfigMap":
					Expect(unstructuredObj.GetName()).To(Equal("ref-" + configMap.Name))
					Expect(unstructuredObj.Object["data"]).To(WithTransform(func(data interface{}) map[string]string {
						strMap := make(map[string]string)
						for k, v := range data.(map[string]interface{}) {
							strMap[k] = v.(string)
						}
						return strMap
					}, Equal(configMap.Data)))
				default:
					Fail(unstructuredObj.GetName() + " has unexpected kind")
				}
			}
		})

		It("should return an error if the referenced object is not found", func() {
			resources[0].ResourceRef.Name = "non-existing-secret"

			unstructuredObjs, err := PrepareReferencedResourcesForSeedCopy(ctx, fakeClient, resources, sourceNamespace, targetNamespace)
			Expect(unstructuredObjs).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("object not found")))
		})
	})
})
