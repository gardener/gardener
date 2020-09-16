// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootsecrets_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestShootSeecrets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shoot Secrets Suite")
}
