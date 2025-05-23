// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garbagecollector_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGarbageCollector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ResourceManager Controller GarbageCollector Suite")
}
