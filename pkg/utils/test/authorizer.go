// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"fmt"

	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// FakeAuthorizer is a test authorizer that delegates to Fn.
type FakeAuthorizer struct {
	// Fn is the function called by Authorize and ConditionsAwareAuthorize.
	Fn func(context.Context, authorizer.Attributes) (authorizer.Decision, string, error)
}

// Authorize implements authorizer.Authorizer.
func (a *FakeAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	return a.Fn(ctx, attrs)
}

// ConditionsAwareAuthorize implements authorizer.ConditionsAwareAuthorizer.
func (a *FakeAuthorizer) ConditionsAwareAuthorize(ctx context.Context, attrs authorizer.Attributes) authorizer.ConditionsAwareDecision {
	return authorizer.ConditionsAwareDecisionFromParts(a.Authorize(ctx, attrs))
}

// EvaluateConditions implements authorizer.ConditionsAwareAuthorizer.
func (a *FakeAuthorizer) EvaluateConditions(_ context.Context, _ authorizer.ConditionsAwareDecision, _ authorizer.ConditionsData) (authorizer.Decision, string, error) {
	return authorizer.DecisionDeny, "", authorizer.ErrorConditionEvaluationNotSupported
}

// Allow is an authorizer function that always returns DecisionAllow.
func Allow(_ context.Context, _ authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionAllow, "", nil
}

// Deny is an authorizer function that always returns DecisionDeny.
func Deny(_ context.Context, _ authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionDeny, "deny", nil
}

// NoOpinion is an authorizer function that always returns DecisionNoOpinion.
func NoOpinion(_ context.Context, _ authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionNoOpinion, "noopinion", nil
}

// Err is an authorizer function that always returns an error.
func Err(_ context.Context, _ authorizer.Attributes) (authorizer.Decision, string, error) {
	return -1, "", fmt.Errorf("fake-err")
}

// UnexpectedDecision is an authorizer function that returns an unrecognized decision value.
func UnexpectedDecision(_ context.Context, _ authorizer.Attributes) (authorizer.Decision, string, error) {
	return -1, "", nil
}

// Timeout is an authorizer function that blocks until the context is cancelled.
func Timeout(ctx context.Context, _ authorizer.Attributes) (authorizer.Decision, string, error) {
	<-ctx.Done()
	return 0, "", ctx.Err()
}

// AllowForUser is an authorizer function that allows the "allowed-user" username and denies all others.
func AllowForUser(_ context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
	username := a.GetUser().GetName()

	if username == "allowed-user" {
		return authorizer.DecisionAllow, "", nil
	}

	return authorizer.DecisionDeny, "", nil
}
