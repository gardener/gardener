// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"strings"
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
	DeferCleanup(test.WithVar(&Read, func(b []byte) (int, error) {
		copy(b, []byte(strings.Repeat("_", len(b))))
		return len(b), nil
	}))
	DeferCleanup(test.WithVar(&Clock, testclock.NewFakeClock(time.Time{})))
})
