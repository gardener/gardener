// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestShoot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardenlet Shoot Controller Suite")
}
