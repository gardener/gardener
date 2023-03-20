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

package vpa_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		ctx         = context.TODO()
		crdDeployer component.Deployer
	)

	JustBeforeEach(func() {
		Expect(crdDeployer.Deploy(ctx)).To(Succeed(), "VPA CRD deployment succeeds")
	})

	Context("with applier", func() {
		var c client.Client

		BeforeEach(func() {
			c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
			mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
			applier := kubernetes.NewApplier(c, mapper)

			crdDeployer = NewCRD(applier, nil)
		})

		DescribeTable("CRD is deployed",
			func(crdName string) {
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			},

			Entry("VerticalPodAutoscaler", "verticalpodautoscalers.autoscaling.k8s.io"),
			Entry("VerticalPodAutoscalerCheckpoints", "verticalpodautoscalercheckpoints.autoscaling.k8s.io"),
		)

		DescribeTable("should re-create CRD if it is deleted",
			func(crdName string) {
				Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
				Expect(crdDeployer.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			},

			Entry("VerticalPodAutoscaler", "verticalpodautoscalers.autoscaling.k8s.io"),
			Entry("VerticalPodAutoscalerCheckpoints", "verticalpodautoscalercheckpoints.autoscaling.k8s.io"),
		)
	})

	Context("with registry", func() {
		var registry *managedresources.Registry

		BeforeEach(func() {
			registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

			crdDeployer = NewCRD(nil, registry)
		})

		DescribeTable("CRD is added to registry",
			func(filename string) {
				Expect(registry.SerializedObjects()).To(HaveKeyWithValue(filename, Not(BeEmpty())))
			},

			Entry("VerticalPodAutoscaler", "crd-verticalpodautoscalers.yaml"),
			Entry("VerticalPodAutoscalerCheckpoints", "crd-verticalpodautoscalercheckpoints.yaml"),
		)
	})
})
