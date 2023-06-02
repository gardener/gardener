// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package crds_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		ctx         context.Context
		c           client.Client
		crdDeployer component.DeployWaiter
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(apiextensionsv1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)

		applier := kubernetes.NewApplier(c, mapper)

		crdDeployer = crds.NewCRD(applier)
	})

	JustBeforeEach(func() {
		Expect(crdDeployer.Deploy(ctx)).ToNot(HaveOccurred(), "extensions crds deploy succeeds")
	})

	DescribeTable("CRD is deployed",
		func(crdName string) {
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).ToNot(HaveOccurred())
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
			Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).ToNot(HaveOccurred())
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(crdDeployer.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).ToNot(HaveOccurred())
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
