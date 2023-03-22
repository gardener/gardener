// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
var _ cleanupVerifier = Verifiers{}

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

// cleanupVerifier can be implemented optionally to run cleanup code.
type cleanupVerifier interface {
	// Cleanup is passed to ginkgo.DeferCleanup.
	Cleanup(ctx context.Context)
}

// Cleanup is passed to ginkgo.DeferCleanup.
func (v Verifiers) Cleanup(ctx context.Context) {
	for _, vv := range v {
		if cleanup, ok := vv.(cleanupVerifier); ok {
			cleanup.Cleanup(ctx)
		}
	}
}
