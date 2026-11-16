// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package cel_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/cel"
)

var _ = Describe("Compatibility", func() {
	var (
		ctx      = context.Background()
		compiler *Compiler
		shoot    *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		var err error
		compiler, err = NewCompiler()
		Expect(err).NotTo(HaveOccurred())

		shoot = &gardencorev1beta1.Shoot{}
		shoot.Spec.Kubernetes.Version = "1.32.3"
	})

	Describe("#Compile", func() {
		It("should compile a valid boolean rule", func() {
			rule, err := compiler.Compile("shoot.spec.kubernetes.version.matches('^1\\\\.3[2-9]')")
			Expect(err).NotTo(HaveOccurred())
			Expect(rule).NotTo(BeNil())
		})

		It("should reject a rule with a syntax error", func() {
			_, err := compiler.Compile("shoot.spec.kubernetes.version ===")
			Expect(err).To(HaveOccurred())
		})

		It("should reject a rule that does not return a boolean", func() {
			_, err := compiler.Compile("shoot.spec.kubernetes.version")
			Expect(err).To(MatchError(ContainSubstring("must evaluate to a boolean")))
		})

		It("should return the cached program for a repeated rule", func() {
			first, err := compiler.Compile("1 == 1")
			Expect(err).NotTo(HaveOccurred())
			second, err := compiler.Compile("1 == 1")
			Expect(err).NotTo(HaveOccurred())
			Expect(second).To(BeIdenticalTo(first))
		})
	})

	Describe("#EvalBool", func() {
		It("should evaluate to true when the shoot satisfies the rule", func() {
			rule, err := compiler.Compile("shoot.spec.kubernetes.version.matches('^1\\\\.3[2-9]')")
			Expect(err).NotTo(HaveOccurred())

			result, err := rule.EvalBool(ctx, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeTrue())
		})

		It("should evaluate to false when the shoot violates the rule", func() {
			shoot.Spec.Kubernetes.Version = "1.30.0"
			rule, err := compiler.Compile("shoot.spec.kubernetes.version.matches('^1\\\\.3[2-9]')")
			Expect(err).NotTo(HaveOccurred())

			result, err := rule.EvalBool(ctx, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return an error when the rule references a non-existent field", func() {
			rule, err := compiler.Compile("shoot.spec.doesNotExist == true")
			Expect(err).NotTo(HaveOccurred())

			_, err = rule.EvalBool(ctx, shoot)
			Expect(err).To(HaveOccurred())
		})
	})
})
