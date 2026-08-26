// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOpenTelemetryOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Observability OpenTelemetry Operator Suite")
}
