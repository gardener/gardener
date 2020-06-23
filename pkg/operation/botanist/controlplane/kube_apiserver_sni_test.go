// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controlplane_test

import (
	"context"

	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("#KubeAPIServerSNI", func() {
	const (
		deployNS   = "test-chart-namespace"
		deployName = "test-deploy"
	)
	var (
		ca               kubernetes.ChartApplier
		ctx              context.Context
		c                client.Client
		defaultDepWaiter component.DeployWaiter
	)

	BeforeEach(func() {

		ctx = context.TODO()

		s := runtime.NewScheme()

		c = fake.NewFakeClientWithScheme(s)

		renderer := cr.NewWithServerVersion(&version.Info{})
		ca = kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))

		defaultDepWaiter = NewKubeAPIServerSNI(&KubeAPIServerSNIValues{
			Hosts:                 []string{"foo.bar"},
			ApiserverClusterIP:    "1.1.1.1",
			IstioIngressNamespace: "istio-foo",
			Name:                  deployName,
			NamespaceUID:          types.UID("123456"),
		}, deployNS, ca, chartsRoot(), c)
	})

	It("deploys succeeds", func() {
		Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())
	})

	It("destroy succeeds", func() {
		Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())
		Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
	})

	It("wait succeeds", func() {
		Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())
		Expect(defaultDepWaiter.Wait(ctx)).ToNot(HaveOccurred())
	})

	It("destroy succeeds", func() {
		Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())
		Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		Expect(defaultDepWaiter.WaitCleanup(ctx)).ToNot(HaveOccurred())
	})
})
