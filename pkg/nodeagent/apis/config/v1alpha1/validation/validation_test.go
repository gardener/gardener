// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	. "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1/validation"
)

var _ = Describe("#ValidateNodeAgentConfiguration", func() {
	var config *NodeAgentConfiguration

	BeforeEach(func() {
		config = &NodeAgentConfiguration{
			Controllers: ControllerConfiguration{
				OperatingSystemConfig: OperatingSystemConfigControllerConfig{
					SecretName:        "osc-secret",
					SyncPeriod:        &metav1.Duration{Duration: time.Minute},
					KubernetesVersion: semver.MustParse("v1.27.0"),
				},
				Token: TokenControllerConfig{
					SyncConfigs: []TokenSecretSyncConfig{{
						SecretName: "gardener-node-agent",
						Path:       "/var/lib/gardener-node-agent/credentials/token",
					}},
					SyncPeriod: &metav1.Duration{Duration: time.Hour},
				},
			},
		}
	})

	It("should pass because all necessary fields is specified", func() {
		Expect(ValidateNodeAgentConfiguration(config)).To(BeEmpty())
	})

	Context("client connection configuration", func() {
		var (
			clientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration
			fldPath          *field.Path
		)

		BeforeEach(func() {
			SetObjectDefaults_NodeAgentConfiguration(config)

			clientConnection = &config.ClientConnection
			fldPath = field.NewPath("clientConnection")
		})

		It("should allow default client connection configuration", func() {
			Expect(ValidateNodeAgentConfiguration(config)).To(BeEmpty())
		})

		It("should return errors because some values are invalid", func() {
			clientConnection.Burst = -1

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fldPath.Child("burst").String()),
				})),
			))
		})
	})

	Context("Operating System Config Controller", func() {
		It("should fail because kubernetes version is empty", func() {
			config.Controllers.OperatingSystemConfig.KubernetesVersion = nil

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controllers.operatingSystemConfig.kubernetesVersion"),
				})),
			))
		})

		It("should fail because operating system config secret name is not specified", func() {
			config.Controllers.OperatingSystemConfig.SecretName = ""

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controllers.operatingSystemConfig.secretName"),
				})),
			))
		})

		It("should fail because sync period is too small", func() {
			config.Controllers.OperatingSystemConfig.SyncPeriod.Duration = 10 * time.Second

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllers.operatingSystemConfig.syncPeriod"),
				})),
			))
		})
	})

	Context("Token Controller", func() {
		It("should fail because access token secret name is not specified", func() {
			config.Controllers.Token.SyncConfigs = append(config.Controllers.Token.SyncConfigs, TokenSecretSyncConfig{
				Path: "/some/path",
			})

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controllers.token.syncConfigs[1].secretName"),
				})),
			))
		})

		It("should fail because path is not specified", func() {
			config.Controllers.Token.SyncConfigs = append(config.Controllers.Token.SyncConfigs, TokenSecretSyncConfig{
				SecretName: "/some/secret",
			})

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controllers.token.syncConfigs[1].path"),
				})),
			))
		})

		It("should fail because path is duplicated", func() {
			config.Controllers.Token.SyncConfigs = append(config.Controllers.Token.SyncConfigs, TokenSecretSyncConfig{
				SecretName: "other-secret",
				Path:       "/var/lib/gardener-node-agent/credentials/token",
			})

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("controllers.token.syncConfigs[1].path"),
				})),
			))
		})

		It("should fail because sync period is too small", func() {
			config.Controllers.Token.SyncPeriod.Duration = 10 * time.Second

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllers.token.syncPeriod"),
				})),
			))
		})
	})
})
