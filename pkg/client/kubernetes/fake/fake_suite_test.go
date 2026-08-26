// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package fake_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFake(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Kubernetes Fake Suite")
}
