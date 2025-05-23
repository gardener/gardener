// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ConditionBuilder build a Condition.
type ConditionBuilder interface {
	WithOldCondition(old gardencorev1beta1.Condition) ConditionBuilder
	WithStatus(status gardencorev1beta1.ConditionStatus) ConditionBuilder
	WithReason(reason string) ConditionBuilder
	WithMessage(message string) ConditionBuilder
	WithCodes(codes ...gardencorev1beta1.ErrorCode) ConditionBuilder
	WithClock(clock clock.Clock) ConditionBuilder
	Build() (new gardencorev1beta1.Condition, updated bool)
}

// defaultConditionBuilder build a Condition.
type defaultConditionBuilder struct {
	old           gardencorev1beta1.Condition
	status        gardencorev1beta1.ConditionStatus
	conditionType gardencorev1beta1.ConditionType
	reason        *string
	message       *string
	codes         []gardencorev1beta1.ErrorCode
	clock         clock.Clock
}

// NewConditionBuilder returns a ConditionBuilder for a specific condition.
func NewConditionBuilder(conditionType gardencorev1beta1.ConditionType) (ConditionBuilder, error) {
	if conditionType == "" {
		return nil, errors.New("conditionType cannot be empty")
	}

	return &defaultConditionBuilder{
		conditionType: conditionType,
		clock:         clock.RealClock{},
	}, nil
}

// WithOldCondition sets the old condition. It can be used to prodive default values.
// The old's condition type is overridden to the one specified in the builder.
func (b *defaultConditionBuilder) WithOldCondition(old gardencorev1beta1.Condition) ConditionBuilder {
	old.Type = b.conditionType
	b.old = old

	return b
}

// WithStatus sets the status of the condition.
func (b *defaultConditionBuilder) WithStatus(status gardencorev1beta1.ConditionStatus) ConditionBuilder {
	b.status = status
	return b
}

// WithReason sets the reason of the condition.
func (b *defaultConditionBuilder) WithReason(reason string) ConditionBuilder {
	b.reason = &reason
	return b
}

// WithMessage sets the message of the condition.
func (b *defaultConditionBuilder) WithMessage(message string) ConditionBuilder {
	b.message = &message
	return b
}

// WithCodes sets the codes of the condition.
func (b *defaultConditionBuilder) WithCodes(codes ...gardencorev1beta1.ErrorCode) ConditionBuilder {
	b.codes = codes
	return b
}

// WithClock sets a `clock.Clock` which is used for getting the current time
func (b *defaultConditionBuilder) WithClock(clock clock.Clock) ConditionBuilder {
	b.clock = clock

	return b
}

// Build creates the condition and returns if there are modifications with the OldCondition.
// If OldCondition is provided:
// - Any changes to status set the `LastTransitionTime`
// - Any updates to the message, reason or the codes cause set `LastUpdateTime` to the current time.
// - The error codes will not be transferred from the old to the new condition
func (b *defaultConditionBuilder) Build() (c gardencorev1beta1.Condition, updated bool) {
	var (
		now       = metav1.Time{Time: b.clock.Now()}
		emptyTime = metav1.Time{}
	)

	c = *b.old.DeepCopy()

	if c.LastTransitionTime == emptyTime {
		c.LastTransitionTime = now
	}

	if c.LastUpdateTime == emptyTime {
		c.LastUpdateTime = now
	}

	c.Type = b.conditionType

	if b.status != "" {
		c.Status = b.status
	} else if b.old.Status == "" {
		c.Status = gardencorev1beta1.ConditionUnknown
	}

	c.Reason = b.buildReason()

	c.Message = b.buildMessage()

	c.Codes = b.codes

	if c.Status != b.old.Status {
		c.LastTransitionTime = now
	}

	if c.Reason != b.old.Reason ||
		c.Message != b.old.Message ||
		!apiequality.Semantic.DeepEqual(c.Codes, b.old.Codes) {
		c.LastUpdateTime = now
	}

	return c, !apiequality.Semantic.DeepEqual(c, b.old)
}

func (b *defaultConditionBuilder) buildMessage() string {
	if message := b.message; message != nil {
		if *message != "" {
			return *message
		}
		// We need to set a condition message in this case because when the condition is updated the next time
		// without specifying a message we want to retain this message instead of toggling to `b.old.Message == ""`.
		return "No message given."
	}

	if b.old.Message == "" {
		return "The condition has been initialized but its semantic check has not been performed yet."
	}
	return b.old.Message
}

func (b *defaultConditionBuilder) buildReason() string {
	if reason := b.reason; reason != nil {
		if *reason != "" {
			return *reason
		}
		// We need to set a condition reason in this case because when the condition is updated the next time
		// without specifying a reason we want to retain this reason instead of toggling to `b.old.Reason == ""`.
		return "Unspecified"
	}
	if b.old.Reason == "" {
		return "ConditionInitialized"
	}
	return b.old.Reason
}
