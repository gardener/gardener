// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package customverbauthorizer_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCustomVerbAuthorizer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission CustomVerbAuthorizer Suite")
}
