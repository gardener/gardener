// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Extensions", func() {
	Describe("CheckExtensionObject", func() {
		DescribeTable("extension objects",
			func(obj client.Object, match types.GomegaMatcher) {
				Expect(health.CheckExtensionObject(obj)).To(match)
			},
			Entry("not an extensionsv1alpha1.Object",
				&corev1.Pod{},
				MatchError(ContainSubstring("expected extensionsv1alpha1.Object")),
			),
			Entry("healthy",
				&extensionsv1alpha1.Infrastructure{
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								State: gardencorev1beta1.LastOperationStateSucceeded,
							},
						},
					},
				},
				Succeed(),
			),
			Entry("generation outdated",
				&extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								State: gardencorev1beta1.LastOperationStateSucceeded,
							},
						},
					},
				},
				HaveOccurred(),
			),
			Entry("gardener operation ongoing",
				&extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						},
					},
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								State: gardencorev1beta1.LastOperationStateSucceeded,
							},
						},
					},
				},
				HaveOccurred(),
			),
			Entry("last error non-nil",
				&extensionsv1alpha1.Infrastructure{
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastError: &gardencorev1beta1.LastError{
								Description: "something happened",
							},
							LastOperation: &gardencorev1beta1.LastOperation{
								State: gardencorev1beta1.LastOperationStateSucceeded,
							},
						},
					},
				},
				HaveOccurred(),
			),
			Entry("no last operation",
				&extensionsv1alpha1.Infrastructure{},
				HaveOccurred(),
			),
			Entry("last operation not succeeded",
				&extensionsv1alpha1.Infrastructure{
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								State: gardencorev1beta1.LastOperationStateError,
							},
						},
					},
				},
				HaveOccurred(),
			),
		)
	})

	Describe("ExtensionOperationHasBeenUpdatedSince", func() {
		var (
			healthFunc health.Func
			now        metav1.Time
		)

		BeforeEach(func() {
			now = metav1.Now()
			healthFunc = health.ExtensionOperationHasBeenUpdatedSince(now)
		})

		It("should fail if object is not an extensionsv1alpha1.Object", func() {
			Expect(healthFunc(&corev1.Pod{})).To(MatchError(ContainSubstring("expected extensionsv1alpha1.Object")))
		})
		It("should fail if last operation is unset", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				Status: extensionsv1alpha1.InfrastructureStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						LastOperation: nil,
					},
				},
			})).NotTo(Succeed())
		})
		It("should fail if last operation update time has not changed", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				Status: extensionsv1alpha1.InfrastructureStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							LastUpdateTime: now,
						},
					},
				},
			})).NotTo(Succeed())
		})
		It("should fail if last operation update time was before given time", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				Status: extensionsv1alpha1.InfrastructureStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							LastUpdateTime: metav1.NewTime(now.Add(-time.Second)),
						},
					},
				},
			})).NotTo(Succeed())
		})
		It("should succeed if last operation update time is after given time", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				Status: extensionsv1alpha1.InfrastructureStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							LastUpdateTime: metav1.NewTime(now.Add(time.Second)),
						},
					},
				},
			})).To(Succeed())
		})
	})
})
