// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"bytes"
	"context"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/shoot"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Resources", func() {
	var (
		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		botanist *Botanist

		ctx             = context.TODO()
		gardenNamespace = "garden-foo"
		seedNamespace   = "shoot--foo--bar"

		resource *corev1.Secret
	)

	BeforeEach(func() {
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		seedClientSet = kubernetesfake.NewClientSetBuilder().WithClient(seedClient).Build()

		botanist = &Botanist{Operation: &operation.Operation{
			GardenClient:  gardenClient,
			SeedClientSet: seedClientSet,
			Shoot:         &shoot.Shoot{SeedNamespace: seedNamespace},
		}}

		resource = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo",
				Namespace:   gardenNamespace,
				Finalizers:  []string{"some-finalizer1", "some-finalizer2"},
				Annotations: map[string]string{"foo": "bar"},
				Labels:      map[string]string{"bar": "foo"},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"some": []byte("data")},
		}

		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: gardenNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Resources: []gardencorev1beta1.NamedResourceReference{{
					Name: "resource",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       resource.Name,
					},
				}},
			},
		})
	})

	Describe("#DeployReferencedResources", func() {
		It("should deploy the managed resource and its secret for the referenced resources", func() {
			Expect(gardenClient.Create(ctx, resource)).To(Succeed())

			Expect(botanist.DeployReferencedResources(ctx)).To(Succeed())

			managedResource := &resourcesv1alpha1.ManagedResource{}
			Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, "referenced-resources"), managedResource)).To(Succeed())
			Expect(managedResource.Spec.Class).To(PointTo(Equal("seed")))
			Expect(managedResource.Spec.ForceOverwriteAnnotations).To(PointTo(BeFalse()))
			Expect(managedResource.Spec.KeepObjects).To(PointTo(BeFalse()))
			Expect(managedResource.Spec.SecretRefs).To(HaveLen(1))

			managedResourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResource.Spec.SecretRefs[0].Name,
					Namespace: managedResource.Namespace,
				},
			}
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Data).To(HaveKey("referenced-resources"))

			var (
				decoder    = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(managedResourceSecret.Data["referenced-resources"]), 1024)
				decodedObj map[string]interface{}
			)

			for indexInFile := 0; true; indexInFile++ {
				err := decoder.Decode(&decodedObj)
				if err == io.EOF {
					break
				}
				Expect(err).NotTo(HaveOccurred())

				if decodedObj == nil {
					continue
				}

				secret := &corev1.Secret{}
				Expect(kubernetesscheme.Scheme.Convert(&unstructured.Unstructured{Object: decodedObj}, secret, nil)).To(Succeed())

				Expect(secret).To(Equal(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "ref-" + resource.Name,
						Namespace:   seedNamespace,
						Labels:      resource.Labels,
						Annotations: resource.Annotations,
					},
					Type: resource.Type,
					Data: resource.Data,
				}))
			}
		})
	})

	Describe("#DestroyReferencedResources", func() {
		It("should destroy the managed resource and its secret for the referenced resources", func() {
			managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: seedNamespace, Name: "referenced-resources"}}
			Expect(seedClient.Create(ctx, managedResource)).To(Succeed())

			managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: seedNamespace, Name: "referenced-resources"}}
			Expect(seedClient.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(botanist.DestroyReferencedResources(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(managedResource), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), &corev1.Secret{})).To(BeNotFoundError())
		})
	})
})
