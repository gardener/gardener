// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/etcd/bootstrap"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Etcd", func() {
	var (
		c         client.Client
		sm        secretsmanager.Interface
		etcd      component.Deployer
		ctx       = context.Background()
		namespace = "shoot--foo--bar"
		image     = "some.registry.io/etcd:v3.5.10"
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
		etcd = New(c, namespace, sm, Values{Image: image})
	})

	Describe("#Deploy", func() {
		It("should successfully deploy bootstrap etcd", func() {
			Expect(etcd.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(etcd.Destroy(ctx)).To(Succeed())
		})
	})
})
