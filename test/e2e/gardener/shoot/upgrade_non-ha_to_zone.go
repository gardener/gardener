// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
)

var _ = Describe("Shoot Tests", Label("Shoot", "high-availability", "upgrade-to-zone"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = e2e.DefaultShoot("e2e-update-zone")
	f.Shoot.Spec.ControlPlane = nil

	It("Create, Upgrade (non-HA to HA with failure tolerance type 'zone') and Delete Shoot", func() {
		setupDNSForTest()
		DeferCleanup(tearDownDNSForTest)

		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
		defer cancel()

		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		By("Upgrade Shoot (non-HA to HA with failure tolerance type 'zone')")
		ctx, cancel = context.WithTimeout(parentCtx, 30*time.Minute)
		defer cancel()
		highavailability.UpgradeAndVerify(ctx, f.ShootFramework, v1beta1.FailureToleranceTypeZone)

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})

var defaultResolver *net.Resolver

func setupDNSForTest() {
	defaultResolver = net.DefaultResolver
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := net.Dialer{
				Timeout: time.Duration(5) * time.Second,
			}
			// We use tcp to distinguish easily in-cluster requests (done via udp) and requests from
			// the tests (using tcp). The result for cluster api names differ depending on the source.
			return dialer.DialContext(ctx, "tcp", "127.0.0.1:5353")
		},
	}
}

func tearDownDNSForTest() {
	net.DefaultResolver = defaultResolver
}
