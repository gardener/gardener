// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoots

import (
	"context"
	"time"

	"github.com/gardener/gardener/test/framework"
	"github.com/onsi/ginkgo"
)

// CIt  contextifies Gingko's It
func CIt(text string, body func(context.Context), timeout time.Duration) {
	ginkgo.It(text, contextify(body, timeout), timeout.Seconds())
}

// FCIt contextifies Gingko's FIt
func FCIt(text string, body func(context.Context), timeout time.Duration) {
	ginkgo.FIt(text, contextify(body, timeout), timeout.Seconds())
}

// CAfterSuite contextifies Gingko's FIt
func CAfterSuite(body func(context.Context), timeout time.Duration) {
	ginkgo.AfterSuite(contextify(body, timeout), timeout.Seconds())
}

// CAfterEach contextifies Gingko's AfterEach
func CAfterEach(body func(context.Context), timeout time.Duration) {
	ginkgo.AfterEach(contextify(body, timeout), timeout.Seconds())
}

// CBeforeSuite contextifies Gingko's FIt
func CBeforeSuite(body func(context.Context), timeout time.Duration) {
	ginkgo.BeforeSuite(contextify(body, timeout), timeout.Seconds())
}

// CBeforeEach contextifies Gingko's BeforeEach
func CBeforeEach(body func(ctx context.Context), timeout time.Duration) {
	ginkgo.BeforeEach(contextify(body, timeout), timeout.Seconds())
}

// CJustBeforeEach contextifies Gingko's JustBeforeEach
func CJustBeforeEach(body func(ctx context.Context), timeout time.Duration) {
	ginkgo.JustBeforeEach(contextify(body, timeout), timeout.Seconds())
}

func contextify(body func(context.Context), timeout time.Duration) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		body(ctx)
	}
}

// StringSet checks if a string is set
func StringSet(s string) bool {
	return len(s) != 0
}

// FileExists Checks if a file path exists and fail otherwise
func FileExists(kc string) bool {
	ok, err := framework.Exists(kc)
	if err != nil {
		ginkgo.Fail(err.Error())
	}
	return ok
}
