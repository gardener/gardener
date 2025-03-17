// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/etcd/etcd"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		c           client.Client
		ctx         = context.TODO()
		crdDeployer CRDAccess
		k8sVersion  = semver.MustParse("1.30")
	)

	BeforeEach(func() {
		c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
		applier := kubernetes.NewApplier(c, mapper)
		var err error
		crdDeployer, err = NewCRD(c, applier, k8sVersion)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		Expect(crdDeployer.Deploy(ctx)).To(Succeed(), "Etcd/EtcdCopyBackupsTask CRD deployment succeeds")
	})

	DescribeTable("CRD is deployed",
		func(crdName string) {
			verifyDeployedCRD(ctx, crdName, c)
		},

		Entry("Etcd", "etcds.druid.gardener.cloud"),
		Entry("EtcdCopyBackupsTask", "etcdcopybackupstasks.druid.gardener.cloud"),
	)

	DescribeTable("should re-create CRD if it is deleted",
		func(crdName string) {
			Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(crdDeployer.Deploy(ctx)).To(Succeed())
			verifyDeployedCRD(ctx, crdName, c)
		},

		Entry("Etcd", "etcds.druid.gardener.cloud"),
		Entry("EtcdCopyBackupsTask", "etcdcopybackupstasks.druid.gardener.cloud"),
	)

	Describe("CRD is destroyed", func() {
		JustBeforeEach(func() {
			Expect(crdDeployer.Deploy(ctx)).To(Succeed())
		})

		DescribeTable("CRD is deleted",
			func(crdName string) {
				Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			},

			Entry("Etcd", "etcds.druid.gardener.cloud"),
			Entry("EtcdCopyBackupsTask", "etcdcopybackupstasks.druid.gardener.cloud"),
		)
	})

	DescribeTable("Get CRD",
		func(crdName string) {
			crd, err := crdDeployer.GetCRD(crdName)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd).NotTo(BeNil())
		},

		Entry("Etcd", "etcds.druid.gardener.cloud"),
		Entry("EtcdCopyBackupsTask", "etcdcopybackupstasks.druid.gardener.cloud"),
	)

	DescribeTable("Get CRD YAML",
		func(crdName string) {
			crd, err := crdDeployer.GetCRDYaml(crdName)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd).NotTo(BeNil())
		},

		Entry("Etcd", "etcds.druid.gardener.cloud"),
		Entry("EtcdCopyBackupsTask", "etcdcopybackupstasks.druid.gardener.cloud"),
	)

})

func verifyDeployedCRD(ctx context.Context, crdName string, c client.Client) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, crd)).To(Succeed())
	Expect(crd.Labels).To(HaveKeyWithValue("gardener.cloud/deletion-protected", "true"))
}
