// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package exposureclass_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestExposureClass(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AdmissionPlugin Shoot ExposureClass Suite")
}
