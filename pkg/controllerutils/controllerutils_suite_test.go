// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestControllerutils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerUtils Suite")
}
