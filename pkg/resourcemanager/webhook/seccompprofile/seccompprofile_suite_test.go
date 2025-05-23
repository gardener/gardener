// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seccompprofile_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSeccompProfile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ResourceManager Webhook SeccompProfile Suite")
}
