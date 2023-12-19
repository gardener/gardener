// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package gardenerscheduler_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGardenerScheduler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component GardenerScheduler Suite")
}
