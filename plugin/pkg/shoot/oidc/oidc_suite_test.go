// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oidc_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOidc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission OIDC Suite")
}
