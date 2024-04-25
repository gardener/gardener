// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

func TestEtcd(t *testing.T) {
	gardenletfeatures.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Etcd Suite")
}

const (
	testNamespace = "shoot--test--test"
	testRole      = "main"
	testROLE      = "Main"
)

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))
})
