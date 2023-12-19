// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	testclock "k8s.io/utils/clock/testing"

	. "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

func TestSecrets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Secrets Suite")
}

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVar(&GenerateRandomString, FakeGenerateRandomString))
	DeferCleanup(test.WithVar(&Clock, testclock.NewFakeClock(time.Time{})))
})
