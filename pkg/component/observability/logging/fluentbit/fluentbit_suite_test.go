// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentbit_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFluentBit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Observability Logging FluentBit Suite")
}
