// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretbinding_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apiserver/features"
)

func TestSecretBinding(t *testing.T) {
	features.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "APIServer Registry Core SecretBinding Suite")
}
