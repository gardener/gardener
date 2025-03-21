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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/etcd/etcd"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		c           client.Client
		ctx         = context.TODO()
		crdDeployer component.Deployer
		k8sVersion  = semver.MustParse("1.30")
	)

	Describe("Deployer", func() {
		BeforeEach(func() {
			c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			var err error
			crdDeployer, err = NewCRD(c, k8sVersion)
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
	})

	Describe("Getter", func() {
		var (
			crdGetter CRDGetter
			err       error
		)
		crdGetter, err = NewCRDGetter(k8sVersion)
		Expect(err).NotTo(HaveOccurred())

		DescribeTable("Get CRD",
			func(crdName string) {
				crd, err := crdGetter.GetCRD(crdName)
				Expect(err).NotTo(HaveOccurred())
				Expect(crd).NotTo(BeNil())
			},

			Entry("Etcd", "etcds.druid.gardener.cloud"),
			Entry("EtcdCopyBackupsTask", "etcdcopybackupstasks.druid.gardener.cloud"),
		)

		Describe("Get all CRDs", func() {
			allCRDs := crdGetter.GetAllCRDs()
			Expect(allCRDs).To(HaveLen(2))
			Expect(allCRDs).To(HaveKey("etcds.druid.gardener.cloud"))
			Expect(allCRDs).To(HaveKey("etcdcopybackupstasks.druid.gardener.cloud"))
		})
	})
})

func verifyDeployedCRD(ctx context.Context, crdName string, c client.Client) {
	crdObj := &apiextensionsv1.CustomResourceDefinition{}
	Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, crdObj)).To(Succeed())
	Expect(crdObj.Labels).To(HaveKeyWithValue("gardener.cloud/deletion-protected", "true"))
}
