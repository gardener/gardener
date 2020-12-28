// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes_test

import (
	"sort"
	"time"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("event", func() {
	Context("SortableEvents", func() {
		It("should sort events", func() {
			var (
				event1 = corev1.Event{
					Source:         corev1.EventSource{Component: "kubelet"},
					Message:        "Item 1",
					FirstTimestamp: metav1.NewTime(time.Date(2014, time.January, 15, 0, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(2014, time.January, 15, 0, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeNormal,
				}
				event2 = corev1.Event{
					Source:         corev1.EventSource{Component: "scheduler"},
					Message:        "Item 2",
					FirstTimestamp: metav1.NewTime(time.Date(1987, time.June, 17, 0, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(1987, time.June, 17, 0, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeNormal,
				}
				event3 = corev1.Event{
					Source:         corev1.EventSource{Component: "kubelet"},
					Message:        "Item 3",
					FirstTimestamp: metav1.NewTime(time.Date(2002, time.December, 25, 0, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(2002, time.December, 25, 0, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeNormal,
				}
				events = []corev1.Event{
					event1,
					event2,
					event3,
				}
			)

			sort.Sort(kutil.SortableEvents(events))

			Expect(events).To(Equal([]corev1.Event{event2, event3, event1}))
		})
	})
})
