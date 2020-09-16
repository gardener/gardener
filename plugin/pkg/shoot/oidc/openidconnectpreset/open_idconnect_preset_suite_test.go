// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openidconnectpreset_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOpenIDConnectPreset(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission OpenIDConnectPreset Suite")
}
