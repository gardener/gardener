// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package resourcemanager

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

func TestGardenerResourceManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component ResourceManager Suite")
}

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))
})
