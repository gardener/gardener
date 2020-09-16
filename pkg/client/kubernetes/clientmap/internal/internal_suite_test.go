// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestClientmap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Kubernetes ClientMap Internal Suite")
}
