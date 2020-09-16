// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resources_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDeletionConfirmation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission ControllerRegistration Resources Suite")
}
