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

package dnsrecord

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("reconciler", func() {
	DescribeTable("#updateCreatedCondition", func(old v1beta1.ConditionStatus, f controller.UpdaterFunc, expected v1beta1.ConditionStatus) {
		var err error
		status := &extensionsv1alpha1.DefaultStatus{}
		switch old {
		case v1beta1.ConditionUnknown:
		case v1beta1.ConditionTrue:
			err = addCreatedConditionTrue(status)
			Expect(err).To(BeNil())
		case v1beta1.ConditionFalse:
			err = addCreatedConditionFalse(status)
			Expect(err).To(BeNil())
		}
		oldTime := time.Now().Add(-1 * time.Second)
		if len(status.Conditions) > 0 {
			status.Conditions[0].LastUpdateTime.Time = oldTime
			status.Conditions[0].LastUpdateTime.Time = oldTime
		}

		err = f(status)
		Expect(err).To(BeNil())
		Expect(len(status.Conditions)).To(Equal(1))
		Expect(status.Conditions[0].Status).To(Equal(expected))
		if old != expected {
			Expect(status.Conditions[0].LastUpdateTime.Time).NotTo(Equal(oldTime))
		} else {
			Expect(status.Conditions[0].LastUpdateTime.Time).To(Equal(oldTime))
		}
	},

		Entry("empty -> success => true", v1beta1.ConditionUnknown, addCreatedConditionTrue, v1beta1.ConditionTrue),
		Entry("empty -> error => false", v1beta1.ConditionUnknown, addCreatedConditionFalse, v1beta1.ConditionFalse),
		Entry("success -> success => true", v1beta1.ConditionTrue, addCreatedConditionTrue, v1beta1.ConditionTrue),
		Entry("success -> error => true", v1beta1.ConditionTrue, addCreatedConditionFalse, v1beta1.ConditionTrue),
		Entry("error -> success => false", v1beta1.ConditionFalse, addCreatedConditionTrue, v1beta1.ConditionTrue),
		Entry("error -> error => false", v1beta1.ConditionFalse, addCreatedConditionFalse, v1beta1.ConditionFalse),
	)
})
