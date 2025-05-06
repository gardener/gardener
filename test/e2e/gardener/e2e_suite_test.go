// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"flag"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/test/e2e"
	_ "github.com/gardener/gardener/test/e2e/gardener/managedseed"
	_ "github.com/gardener/gardener/test/e2e/gardener/project"
	_ "github.com/gardener/gardener/test/e2e/gardener/seed"
	"github.com/gardener/gardener/test/e2e/gardener/shoot"
	_ "github.com/gardener/gardener/test/e2e/gardener/shoot/gardenerupgrade"
)

func TestMain(m *testing.M) {
	shoot.RegisterShootFlags()
	flag.Parse()
	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	if os.Getenv("USE_PROVIDER_LOCAL_COREDNS_SERVER") == "true" {
		e2e.UseProviderLocalCoreDNSServer()
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test E2E Gardener Suite")
}
