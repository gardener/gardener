// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package references_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestReferences(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ResourceManager Controller GarbageCollector References Suite")
}
