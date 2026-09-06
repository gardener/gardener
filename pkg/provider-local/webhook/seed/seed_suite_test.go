// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSeedWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider-Local Webhook Seed Suite")
}
