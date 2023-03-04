// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/machinecontrollermanager"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("MachineControllerManager", func() {
	var (
		ctx       = context.TODO()
		namespace = "shoot--foo--bar"

		image                    = "mcm-image:tag"
		runtimeKubernetesVersion = semver.MustParse("1.26.1")

		fakeClient client.Client
		sm         secretsmanager.Interface
		values     Values
		mcm        component.DeployWaiter
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)
		values = Values{
			Image:                    image,
			Replicas:                 1,
			RuntimeKubernetesVersion: runtimeKubernetesVersion,
		}
		mcm = New(fakeClient, namespace, sm, values)
	})

	Describe("#Deploy", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(mcm.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(mcm.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(mcm.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(mcm.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
