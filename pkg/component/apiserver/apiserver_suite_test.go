// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver_test

import (
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	testclock "k8s.io/utils/clock/testing"

	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

func TestAPIServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component APIServer Suite")
}

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVar(&secretsutils.GenerateRandomString, secretsutils.FakeGenerateRandomString))
	DeferCleanup(test.WithVar(&secretsutils.Read, func(b []byte) (int, error) {
		copy(b, []byte(strings.Repeat("_", len(b))))
		return len(b), nil
	}))
	DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))
	DeferCleanup(test.WithVar(&secretsutils.Clock, testclock.NewFakeClock(time.Time{})))
})
