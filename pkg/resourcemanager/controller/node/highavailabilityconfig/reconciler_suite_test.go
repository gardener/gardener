// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package highavailabilityconfig

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHighAvailabilityConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ResourceManager Node HighAvailabilityConfig Suite")
}
