// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInPlaceUpdate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardenlet Controller Shoot InPlaceUpdate Suite")
}
