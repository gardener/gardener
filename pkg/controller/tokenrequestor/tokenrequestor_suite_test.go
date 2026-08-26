// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package tokenrequestor_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTokenRequestor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller TokenRequestor Suite")
}
