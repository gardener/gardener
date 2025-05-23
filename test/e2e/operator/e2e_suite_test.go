// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/test/e2e"
	_ "github.com/gardener/gardener/test/e2e/operator/garden"
)

func TestE2E(t *testing.T) {
	if os.Getenv("USE_PROVIDER_LOCAL_COREDNS_SERVER") == "true" {
		e2e.UseProviderLocalCoreDNSServer()
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test E2E Operator Suite")
}
