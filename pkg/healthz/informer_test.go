// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthz_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/healthz"
)

var _ = Describe("Informer", func() {
	Describe("#NewCacheSyncHealthz", func() {
		It("should succeed if all informers sync", func() {
			checker := NewCacheSyncHealthz(&fakeSyncWaiter{true})
			Expect(checker(nil)).To(Succeed())
		})

		It("should fail if informers don't sync", func() {
			checker := NewCacheSyncHealthz(&fakeSyncWaiter{false})
			Expect(checker(nil)).To(MatchError(ContainSubstring("not synced")))
		})
	})
})

type fakeSyncWaiter struct {
	value bool
}

func (f *fakeSyncWaiter) WaitForCacheSync(_ context.Context) bool { return f.value }
