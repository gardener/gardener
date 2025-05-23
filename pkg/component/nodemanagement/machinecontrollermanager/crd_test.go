// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	. "github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		ctx         = context.TODO()
		fakeClient  client.Client
		crdDeployer component.DeployWaiter
	)

	BeforeEach(func() {
		var err error
		fakeClient = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
		applier := kubernetes.NewApplier(fakeClient, mapper)

		crdDeployer, err = NewCRD(fakeClient, applier)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("#Deploy", func() {
		It("should deploy the CRDs", func() {
			Expect(crdDeployer.Deploy(ctx)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machinedeployments.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machines.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machinesets.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should delete the CRDs", func() {
			Expect(crdDeployer.Destroy(ctx)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machineclasses.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machinedeployments.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machines.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "machinesets.machine.sapcloud.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
		})
	})
})
