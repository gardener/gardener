// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package prometheusoperator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
	"github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		ctx      = context.TODO()
		c        client.Client
		deployer component.Deployer
	)

	BeforeEach(func() {
		c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
		applier := kubernetes.NewApplier(c, mapper)

		deployer = NewCRDs(applier)
	})

	Describe("#Deploy", func() {
		It("should deploy the CRD", func() {
			Expect(deployer.Deploy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "alertmanagerconfigs.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "alertmanagers.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "podmonitors.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "probes.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "prometheusagents.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "prometheuses.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "prometheusrules.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "scrapeconfigs.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "servicemonitors.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "thanosrulers.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should delete the CRD", func() {
			Expect(deployer.Destroy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "alertmanagerconfigs.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "alertmanagers.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "podmonitors.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "probes.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "prometheusagents.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "prometheuses.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "prometheusrules.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "scrapeconfigs.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "servicemonitors.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "thanosrulers.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
		})
	})
})
