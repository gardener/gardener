// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package machinecontrollermanager_test

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
	. "github.com/gardener/gardener/pkg/component/machinecontrollermanager"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		ctx         = context.TODO()
		fakeClient  client.Client
		crdDeployer component.Deployer
	)

	BeforeEach(func() {
		fakeClient = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
		applier := kubernetes.NewApplier(fakeClient, mapper)

		crdDeployer = NewCRD(fakeClient, applier)
	})

	Describe("#Deploy", func() {
		It("should deploy the CRDs", func() {
			Expect(crdDeployer.Deploy(ctx)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "alicloudmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "awsmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "azuremachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "gcpmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machinedeployments.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machines.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machinesets.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "openstackmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "packetmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should delete the CRDs", func() {
			Expect(crdDeployer.Destroy(ctx)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "alicloudmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "awsmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "azuremachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "gcpmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machinedeployments.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machines.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machinesets.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "openstackmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "packetmachineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
		})
	})
})
