// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeDurationPointer(d time.Duration) *metav1.Duration {
	return &metav1.Duration{Duration: d}
}
