// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
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
		Expect(co.ErrorToleration).To(HaveLen(1))
	})

	It("should allow setting CleanOptions", func() {
		co := &CleanOptions{}
		(&CleanOptions{
			ListOptions:                []client.ListOption{client.InNamespace("ns"), client.MatchingLabels{"key": "value"}},
			DeleteOptions:              []client.DeleteOption{client.GracePeriodSeconds(42), client.DryRunAll},
			FinalizeGracePeriodSeconds: ptr.To[int64](42),
			ErrorToleration:            []TolerateErrorFunc{apierrors.IsConflict},
		}).ApplyToClean(co)
		Expect(co.ListOptions).To(Equal([]client.ListOption{client.InNamespace("ns"), client.MatchingLabels{"key": "value"}}))
		Expect(co.DeleteOptions).To(Equal([]client.DeleteOption{client.GracePeriodSeconds(42), client.DryRunAll}))
		gp := int64(42)
		Expect(co.FinalizeGracePeriodSeconds).To(Equal(&gp))
		Expect(co.ErrorToleration).To(HaveLen(1))
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
