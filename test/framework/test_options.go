// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
)

// TestOptions contains options to add additional functionality
// or cleanup handlers to a testcase.
type TestOptions struct {
	// afterTests holds a list of all registered AfterTest functions
	// that are executed when the test has finished.
	AfterTests afterTests

	// CAfterTests holds a list of all registered contextified AfterTest functions
	// that are executed when the test has finished.
	CAfterTests []cAfterTestOption
}

// ApplyOptions applies the given test options on these options.
func (o *TestOptions) ApplyOptions(opts []TestOption) *TestOptions {
	for _, opt := range opts {
		opt.ApplyToTestOptions(o)
	}
	return o
}

// Complete registers all test options that are configured.
// it should be a function that configures a ginkgo test case
// This should get called when all options are applied.
func (o *TestOptions) Complete(it func()) {
	if len(o.AfterTests) == 0 && len(o.CAfterTests) == 0 {
		it()
		return
	}

	// Create a new context so that the afterTests function only runs once after this one testcase.
	// Otherwise the after tests would run after every test case in the outer context.
	ginkgo.Context("", func() {
		it()

		for _, aftertest := range o.AfterTests {
			// register afterTest to global aftersuite in case the test interrupts
			var h CleanupActionHandle
			ginkgo.BeforeEach(func() {
				h = AddCleanupAction(aftertest)
			})
			ginkgo.AfterEach(func() {
				RemoveCleanupAction(h)
				aftertest()
			})
		}

		for _, caftertest := range o.CAfterTests {
			// register afterTest to global aftersuite in case the test interrupts
			var h CleanupActionHandle
			ginkgo.BeforeEach(func() {
				h = AddCleanupAction(func() {
					contextify(caftertest.Body, caftertest.Timeout)()
				})
			})
			CAfterEach(func(ctx context.Context) {
				RemoveCleanupAction(h)
				caftertest.Body(ctx)
			}, caftertest.Timeout)
		}
	})
}

// cAfterTestOption contains options for contextified after test function.
type cAfterTestOption struct {
	Body    func(ctx context.Context)
	Timeout time.Duration
}

// ApplyToTestOptions adds contextified after test functions to test options
func (at *cAfterTestOption) ApplyToTestOptions(opts *TestOptions) {
	opts.CAfterTests = append(opts.CAfterTests, *at)
}

// TestOption is some configuration that modifies options for testcase.
type TestOption interface {
	// ApplyToTestOptions applies this configuration to the given test options.
	ApplyToTestOptions(*TestOptions)
}

// afterTests are functions that should run when a test has finished
type afterTests []func()

// ApplyToTestOptions adds after test functions to test options
func (at afterTests) ApplyToTestOptions(opts *TestOptions) {
	opts.AfterTests = append(opts.AfterTests, at...)
}

// WithAfterTests adds functions to the current test that are called
// when the test has finished
func WithAfterTests(funcs ...func()) TestOption {
	return afterTests(funcs)
}

// WithCAfterTest adds contextified functions to the current test that are called
// when the test has finished
func WithCAfterTest(body func(ctx context.Context), timeout time.Duration) TestOption {
	return &cAfterTestOption{
		Body:    body,
		Timeout: timeout,
	}
}
