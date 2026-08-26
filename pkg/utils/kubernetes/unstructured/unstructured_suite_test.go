// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package unstructured_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUnstructured(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Kubernetes Unstructured Suite")
}
