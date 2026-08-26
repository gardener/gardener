// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package staticpod_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStaticPod(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardenadm StaticPod Suite")
}
