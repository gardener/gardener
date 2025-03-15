// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"

	. "github.com/onsi/gomega"
)

// Verifier does some assertions in different phases of the credentials rotation test.
type Verifier interface {
	// Before is called before the rotation is started.
	Before(ctx context.Context)
	// ExpectPreparingStatus is called while waiting for the Preparing status.
	ExpectPreparingStatus(g Gomega)
	// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
	ExpectPreparingWithoutWorkersRolloutStatus(g Gomega)
	// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
	ExpectWaitingForWorkersRolloutStatus(g Gomega)
	// AfterPrepared is called when the Shoot is in Prepared status.
	AfterPrepared(ctx context.Context)
	// ExpectCompletingStatus is called while waiting for the Completing status.
	ExpectCompletingStatus(g Gomega)
	// AfterCompleted is called when the Shoot is in Completed status.
	AfterCompleted(ctx context.Context)
}

// Verifiers combines multiple Verifier instances and calls them sequentially
type Verifiers []Verifier

var _ Verifier = Verifiers{}
var _ CleanupVerifier = Verifiers{}

// Before is called before the rotation is started.
func (v Verifiers) Before(ctx context.Context) {
	for _, vv := range v {
		vv.Before(ctx)
	}
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v Verifiers) ExpectPreparingStatus(g Gomega) {
	for _, vv := range v {
		vv.ExpectPreparingStatus(g)
	}
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v Verifiers) ExpectPreparingWithoutWorkersRolloutStatus(g Gomega) {
	for _, vv := range v {
		vv.ExpectPreparingWithoutWorkersRolloutStatus(g)
	}
}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v Verifiers) ExpectWaitingForWorkersRolloutStatus(g Gomega) {
	for _, vv := range v {
		vv.ExpectWaitingForWorkersRolloutStatus(g)
	}
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v Verifiers) AfterPrepared(ctx context.Context) {
	for _, vv := range v {
		vv.AfterPrepared(ctx)
	}
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v Verifiers) ExpectCompletingStatus(g Gomega) {
	for _, vv := range v {
		vv.ExpectCompletingStatus(g)
	}
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v Verifiers) AfterCompleted(ctx context.Context) {
	for _, vv := range v {
		vv.AfterCompleted(ctx)
	}
}

// CleanupVerifier can be implemented optionally to run cleanup code.
type CleanupVerifier interface {
	// Cleanup is passed to ginkgo.DeferCleanup.
	Cleanup(ctx context.Context)
}

// Cleanup is passed to ginkgo.DeferCleanup.
func (v Verifiers) Cleanup(ctx context.Context) {
	for _, vv := range v {
		if cleanup, ok := vv.(CleanupVerifier); ok {
			cleanup.Cleanup(ctx)
		}
	}
}
