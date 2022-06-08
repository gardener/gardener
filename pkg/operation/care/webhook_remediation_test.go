// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package care_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/care"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("WebhookRemediation", func() {
	var (
		ctx = context.TODO()

		fakeClient              client.Client
		fakeKubernetesInterface kubernetes.Interface
		shootClientInit         func() (kubernetes.Interface, bool, error)

		shoot *gardencorev1beta1.Shoot
		op    *operation.Operation

		remediator *WebhookRemediation
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeKubernetesInterface = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		shootClientInit = func() (kubernetes.Interface, bool, error) {
			return fakeKubernetesInterface, true, nil
		}

		shoot = &gardencorev1beta1.Shoot{}
		op = &operation.Operation{
			Logger: logger.NewNopLogger(),
			Shoot:  &shootpkg.Shoot{},
		}
		op.Shoot.SetInfo(shoot)

		remediator = NewWebhookRemediation(op, shootClientInit)
	})

	Describe("#Remediate", func() {
		It("should remediate the problematic webhooks", func() {
			Expect(remediator.Remediate(ctx)).To(Succeed())
		})
	})
})
