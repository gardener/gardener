// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crds_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/extensions/crds"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#CRDs", func() {
	var (
		err         error
		ctx         context.Context
		c           client.Client
		crdDeployer component.DeployWaiter
		applier     kubernetes.Applier
	)

	BeforeEach(func() {
		ctx = context.Background()

		s := runtime.NewScheme()
		Expect(apiextensionsv1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)

		applier = kubernetes.NewApplier(c, mapper)
	})

	JustBeforeEach(func() {
		Expect(crdDeployer.Deploy(ctx)).ToNot(HaveOccurred(), "extensions crds deploy succeeds")
	})

	When("shoot CRDs are included", func() {
		BeforeEach(func() {
			crdDeployer, err = crds.NewCRD(c, applier, true, true)
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("CRD is deployed",
			func(crdName string) {
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			},

			Entry("BackupBucket", "backupbuckets.extensions.gardener.cloud"),
			Entry("BackupEntry", "backupentries.extensions.gardener.cloud"),
			Entry("Bastion", "bastions.extensions.gardener.cloud"),
			Entry("Cluster", "clusters.extensions.gardener.cloud"),
			Entry("ContainerRuntime", "containerruntimes.extensions.gardener.cloud"),
			Entry("ControlPlane", "controlplanes.extensions.gardener.cloud"),
			Entry("DNSRecord", "dnsrecords.extensions.gardener.cloud"),
			Entry("Extension", "extensions.extensions.gardener.cloud"),
			Entry("Infrastructure", "infrastructures.extensions.gardener.cloud"),
			Entry("Network", "networks.extensions.gardener.cloud"),
			Entry("OperatingSystemConfig", "operatingsystemconfigs.extensions.gardener.cloud"),
			Entry("Worker", "workers.extensions.gardener.cloud"),
		)

		DescribeTable("should re-create CRD if it is deleted",
			func(crdName string) {
				Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
				Expect(crdDeployer.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			},

			Entry("BackupBucket", "backupbuckets.extensions.gardener.cloud"),
			Entry("BackupEntry", "backupentries.extensions.gardener.cloud"),
			Entry("Bastion", "bastions.extensions.gardener.cloud"),
			Entry("Cluster", "clusters.extensions.gardener.cloud"),
			Entry("ContainerRuntime", "containerruntimes.extensions.gardener.cloud"),
			Entry("ControlPlane", "controlplanes.extensions.gardener.cloud"),
			Entry("DNSRecord", "dnsrecords.extensions.gardener.cloud"),
			Entry("Extension", "extensions.extensions.gardener.cloud"),
			Entry("Infrastructure", "infrastructures.extensions.gardener.cloud"),
			Entry("Network", "networks.extensions.gardener.cloud"),
			Entry("OperatingSystemConfig", "operatingsystemconfigs.extensions.gardener.cloud"),
			Entry("Worker", "workers.extensions.gardener.cloud"),
		)
	})

	When("shoot CRDs are not included", func() {
		BeforeEach(func() {
			crdDeployer, err = crds.NewCRD(c, applier, true, false)
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("CRD is deployed",
			func(crdName string, matcher gomegatypes.GomegaMatcher) {
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(matcher)
			},

			Entry("BackupBucket", "backupbuckets.extensions.gardener.cloud", Succeed()),
			Entry("BackupEntry", "backupentries.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Bastion", "bastions.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Cluster", "clusters.extensions.gardener.cloud", BeNotFoundError()),
			Entry("ContainerRuntime", "containerruntimes.extensions.gardener.cloud", BeNotFoundError()),
			Entry("ControlPlane", "controlplanes.extensions.gardener.cloud", BeNotFoundError()),
			Entry("DNSRecord", "dnsrecords.extensions.gardener.cloud", Succeed()),
			Entry("Extension", "extensions.extensions.gardener.cloud", Succeed()),
			Entry("Infrastructure", "infrastructures.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Network", "networks.extensions.gardener.cloud", BeNotFoundError()),
			Entry("OperatingSystemConfig", "operatingsystemconfigs.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Worker", "workers.extensions.gardener.cloud", BeNotFoundError()),
		)

		DescribeTable("should re-create CRD if it is deleted",
			func(crdName string, matcher gomegatypes.GomegaMatcher) {
				Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).To(Or(Succeed(), BeNotFoundError()))
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
				Expect(crdDeployer.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(matcher)
			},

			Entry("BackupBucket", "backupbuckets.extensions.gardener.cloud", Succeed()),
			Entry("BackupEntry", "backupentries.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Bastion", "bastions.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Cluster", "clusters.extensions.gardener.cloud", BeNotFoundError()),
			Entry("ContainerRuntime", "containerruntimes.extensions.gardener.cloud", BeNotFoundError()),
			Entry("ControlPlane", "controlplanes.extensions.gardener.cloud", BeNotFoundError()),
			Entry("DNSRecord", "dnsrecords.extensions.gardener.cloud", Succeed()),
			Entry("Extension", "extensions.extensions.gardener.cloud", Succeed()),
			Entry("Infrastructure", "infrastructures.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Network", "networks.extensions.gardener.cloud", BeNotFoundError()),
			Entry("OperatingSystemConfig", "operatingsystemconfigs.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Worker", "workers.extensions.gardener.cloud", BeNotFoundError()),
		)
	})

	When("general CRDs are not included", func() {
		BeforeEach(func() {
			crdDeployer, err = crds.NewCRD(c, applier, false, true)
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("CRD is deployed",
			func(crdName string, matcher gomegatypes.GomegaMatcher) {
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(matcher)
			},

			Entry("BackupBucket", "backupbuckets.extensions.gardener.cloud", BeNotFoundError()),
			Entry("BackupEntry", "backupentries.extensions.gardener.cloud", Succeed()),
			Entry("Bastion", "bastions.extensions.gardener.cloud", Succeed()),
			Entry("Cluster", "clusters.extensions.gardener.cloud", Succeed()),
			Entry("ContainerRuntime", "containerruntimes.extensions.gardener.cloud", Succeed()),
			Entry("ControlPlane", "controlplanes.extensions.gardener.cloud", Succeed()),
			Entry("DNSRecord", "dnsrecords.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Extension", "extensions.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Infrastructure", "infrastructures.extensions.gardener.cloud", Succeed()),
			Entry("Network", "networks.extensions.gardener.cloud", Succeed()),
			Entry("OperatingSystemConfig", "operatingsystemconfigs.extensions.gardener.cloud", Succeed()),
			Entry("Worker", "workers.extensions.gardener.cloud", Succeed()),
		)

		DescribeTable("should re-create CRD if it is deleted",
			func(crdName string, matcher gomegatypes.GomegaMatcher) {
				Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).To(Or(Succeed(), BeNotFoundError()))
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
				Expect(crdDeployer.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(matcher)
			},

			Entry("BackupBucket", "backupbuckets.extensions.gardener.cloud", BeNotFoundError()),
			Entry("BackupEntry", "backupentries.extensions.gardener.cloud", Succeed()),
			Entry("Bastion", "bastions.extensions.gardener.cloud", Succeed()),
			Entry("Cluster", "clusters.extensions.gardener.cloud", Succeed()),
			Entry("ContainerRuntime", "containerruntimes.extensions.gardener.cloud", Succeed()),
			Entry("ControlPlane", "controlplanes.extensions.gardener.cloud", Succeed()),
			Entry("DNSRecord", "dnsrecords.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Extension", "extensions.extensions.gardener.cloud", BeNotFoundError()),
			Entry("Infrastructure", "infrastructures.extensions.gardener.cloud", Succeed()),
			Entry("Network", "networks.extensions.gardener.cloud", Succeed()),
			Entry("OperatingSystemConfig", "operatingsystemconfigs.extensions.gardener.cloud", Succeed()),
			Entry("Worker", "workers.extensions.gardener.cloud", Succeed()),
		)
	})

	When("deleting CRDs", func() {
		BeforeEach(func() {
			crdDeployer, err = crds.NewCRD(c, applier, true, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete all CRDs", func() {
			Expect(crdDeployer.Destroy(ctx)).To(Succeed(), "extensions crds deletion succeeds")

			Expect(c.Get(ctx, client.ObjectKey{Name: "backupbuckets.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "backupentries.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "bastions.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "clusters.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "containerruntimes.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "controlplanes.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "dnsrecords.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "extensions.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "infrastructures.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "networks.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "operatingsystemconfigs.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "workers.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
		})

		It("should delete shoot CRDs only", func() {
			crdDeployer, err = crds.NewCRD(c, applier, false, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(crdDeployer.Destroy(ctx)).To(Succeed(), "extensions crds deletion succeeds")

			Expect(c.Get(ctx, client.ObjectKey{Name: "backupbuckets.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "backupentries.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "bastions.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "clusters.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "containerruntimes.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "controlplanes.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "dnsrecords.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "extensions.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "infrastructures.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "networks.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "operatingsystemconfigs.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "workers.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
		})

		It("should delete general CRDs only", func() {
			crdDeployer, err = crds.NewCRD(c, applier, true, false)
			Expect(crdDeployer.Destroy(ctx)).To(Succeed(), "extensions crds deletion succeeds")

			Expect(c.Get(ctx, client.ObjectKey{Name: "backupbuckets.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "backupentries.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "bastions.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "clusters.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "containerruntimes.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "controlplanes.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "dnsrecords.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "extensions.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "infrastructures.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "networks.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "operatingsystemconfigs.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "workers.extensions.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
		})
	})
})
