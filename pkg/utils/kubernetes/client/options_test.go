// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/utils/kubernetes/client"
)

var _ = Describe("CleanOptions", func() {
	It("should allow setting ListWith", func() {
		co := &CleanOptions{}
		ListWith{client.InNamespace("ns"), client.MatchingLabels{"key": "value"}}.ApplyToClean(co)
		Expect(co.ListOptions).To(Equal([]client.ListOption{client.InNamespace("ns"), client.MatchingLabels{"key": "value"}}))
	})

	It("should allow setting DeleteWith", func() {
		co := &CleanOptions{}
		DeleteWith{client.GracePeriodSeconds(42), client.DryRunAll}.ApplyToClean(co)
		Expect(co.DeleteOptions).To(Equal([]client.DeleteOption{client.GracePeriodSeconds(42), client.DryRunAll}))
	})

	It("should allow setting FinalizeGracePeriodSeconds", func() {
		co := &CleanOptions{}
		FinalizeGracePeriodSeconds(42).ApplyToClean(co)
		gp := int64(42)
		Expect(co.FinalizeGracePeriodSeconds).To(Equal(&gp))
	})

	It("should allow setting ErrorToleration", func() {
		co := &CleanOptions{}
		TolerateErrors{apierrors.IsConflict}.ApplyToClean(co)
		Expect(len(co.ErrorToleration)).To(Equal(1))
	})

	It("should allow setting CleanOptions", func() {
		co := &CleanOptions{}
		(&CleanOptions{
			ListOptions:                []client.ListOption{client.InNamespace("ns"), client.MatchingLabels{"key": "value"}},
			DeleteOptions:              []client.DeleteOption{client.GracePeriodSeconds(42), client.DryRunAll},
			FinalizeGracePeriodSeconds: pointer.Int64(42),
			ErrorToleration:            []TolerateErrorFunc{apierrors.IsConflict},
		}).ApplyToClean(co)
		Expect(co.ListOptions).To(Equal([]client.ListOption{client.InNamespace("ns"), client.MatchingLabels{"key": "value"}}))
		Expect(co.DeleteOptions).To(Equal([]client.DeleteOption{client.GracePeriodSeconds(42), client.DryRunAll}))
		gp := int64(42)
		Expect(co.FinalizeGracePeriodSeconds).To(Equal(&gp))
		Expect(len(co.ErrorToleration)).To(Equal(1))
	})

	It("should merge multiple options together", func() {
		gp := int64(7)
		co := &CleanOptions{}
		co.ApplyOptions([]CleanOption{
			ListWith{client.InNamespace("ns")},
			FinalizeGracePeriodSeconds(gp),
		})
		Expect(co.ListOptions).To(Equal([]client.ListOption{client.InNamespace("ns")}))
		Expect(co.FinalizeGracePeriodSeconds).To(Equal(&gp))
	})
})
