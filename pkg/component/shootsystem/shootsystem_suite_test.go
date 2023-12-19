// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package shootsystem_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShootSystem(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component ShootSystem Suite")
}
