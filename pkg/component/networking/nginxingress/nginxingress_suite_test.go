// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nginxingress_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNginxIngress(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Networking NginxIngress Suite")
}
