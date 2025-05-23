// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTokenRequestorWorkloadIdentity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardenlet Controller TokenRequestor WorkloadIdentity Suite")
}
