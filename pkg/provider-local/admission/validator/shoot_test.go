// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/provider-local/admission/validator"
)

var _ = Describe("Shoot Validator", func() {
	var (
		ctx = context.Background()

		shootValidator extensionswebhook.Validator
		shoot          *core.Shoot
	)

	BeforeEach(func() {
		shootValidator = validator.NewShootValidator()
		shoot = &core.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-1",
				Namespace: "garden-dev",
			},
			Spec: core.ShootSpec{
				Networking: &core.Networking{},
				Provider: core.Provider{
					Workers: []core.Worker{{Name: "worker-1"}},
				},
			},
		}
	})

	Describe("#Validate", func() {
		It("should succeed for valid IPv4 nodes CIDR", func() {
			shoot.Spec.Networking.Nodes = ptr.To("10.0.0.0/24")
			Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
		})

		It("should fail for invalid IPv4 nodes CIDR", func() {
			shoot.Spec.Networking.Nodes = ptr.To("192.168.0.0/24")
			Expect(shootValidator.Validate(ctx, shoot, nil)).To(MatchError(ContainSubstring("nodes CIDR must be a subnet of 10.0.0.0/16")))
		})

		It("should succeed for valid IPv6 nodes CIDR", func() {
			shoot.Spec.Networking.Nodes = ptr.To("fd00:10:1:100::/64")
			Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
		})

		It("should fail for invalid IPv6 nodes CIDR", func() {
			shoot.Spec.Networking.Nodes = ptr.To("fd00:20:1:100::/64")
			Expect(shootValidator.Validate(ctx, shoot, nil)).To(MatchError(ContainSubstring("nodes CIDR must be a subnet of fd00:10:1:100::/56")))
		})

		It("should fail for empty nodes CIDR", func() {
			shoot.Spec.Networking.Nodes = nil
			Expect(shootValidator.Validate(ctx, shoot, nil)).To(MatchError(ContainSubstring("nodes CIDR must not be empty")))
		})

		It("should fail for invalid CIDR format", func() {
			shoot.Spec.Networking.Nodes = ptr.To("not-a-cidr")
			Expect(shootValidator.Validate(ctx, shoot, nil)).To(MatchError(ContainSubstring("nodes CIDR must be a valid CIDR")))
		})

		It("should succeed if nodes CIDR is added on update", func() {
			oldShoot := shoot.DeepCopy()
			shoot.Spec.Networking.Nodes = ptr.To("10.0.0.0/24")
			Expect(shootValidator.Validate(ctx, shoot, oldShoot)).To(Succeed())
		})

		It("should fail if invalid nodes CIDR is added on update", func() {
			oldShoot := shoot.DeepCopy()
			shoot.Spec.Networking.Nodes = ptr.To("192.168.0.0/24")
			Expect(shootValidator.Validate(ctx, shoot, oldShoot)).To(MatchError(ContainSubstring("nodes CIDR must be a subnet of 10.0.0.0/16")))
		})

		It("should succeed if nodes CIDR is unchanged on update", func() {
			shoot.Spec.Networking.Nodes = ptr.To("192.168.0.0/24")
			oldShoot := shoot.DeepCopy()
			Expect(shootValidator.Validate(ctx, shoot, oldShoot)).To(Succeed())
		})

		It("should succeed if nodes CIDR is changed to valid value on update", func() {
			oldShoot := shoot.DeepCopy()
			oldShoot.Spec.Networking.Nodes = ptr.To("10.0.0.0/24")
			shoot.Spec.Networking.Nodes = ptr.To("10.0.1.0/24")
			Expect(shootValidator.Validate(ctx, shoot, oldShoot)).To(Succeed())
		})

		It("should fail if nodes CIDR is changed to invalid value on update", func() {
			oldShoot := shoot.DeepCopy()
			oldShoot.Spec.Networking.Nodes = ptr.To("10.0.0.0/24")
			shoot.Spec.Networking.Nodes = ptr.To("192.168.0.0/24")
			Expect(shootValidator.Validate(ctx, shoot, oldShoot)).To(MatchError(ContainSubstring("nodes CIDR must be a subnet of 10.0.0.0/16")))
		})

		It("should succeed if Shoot is in deletion phase", func() {
			shoot.Spec.Networking.Nodes = ptr.To("192.168.0.0/24")
			shoot.DeletionTimestamp = ptr.To(metav1.Now())
			Expect(shootValidator.Validate(ctx, shoot, shoot.DeepCopy())).To(Succeed())
		})

		It("should accept empty nodes CIDR for workerless shoot", func() {
			shoot.Spec.Provider.Workers = nil
			shoot.Spec.Networking.Nodes = nil
			Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
		})
	})
})
