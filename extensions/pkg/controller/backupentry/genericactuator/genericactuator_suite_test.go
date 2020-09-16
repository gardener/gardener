// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGenericactuator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BackupEntry Genericactuator Suite")
}
