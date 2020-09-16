// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client_test

import (
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("CleanOptions", func() {
	It("should allow setting ListWith", func() {
		co := &utilclient.CleanOptions{}
		utilclient.ListWith{client.InNamespace("ns"), client.MatchingLabels{"key": "value"}}.ApplyToClean(co)
		Expect(co.ListOptions).To(Equal([]client.ListOption{client.InNamespace("ns"), client.MatchingLabels{"key": "value"}}))
	})

	It("should allow setting DeleteWith", func() {
		co := &utilclient.CleanOptions{}
		utilclient.DeleteWith{client.GracePeriodSeconds(42), client.DryRunAll}.ApplyToClean(co)
		Expect(co.DeleteOptions).To(Equal([]client.DeleteOption{client.GracePeriodSeconds(42), client.DryRunAll}))
	})

	It("should allow setting FinalizeGracePeriodSeconds", func() {
		co := &utilclient.CleanOptions{}
		utilclient.FinalizeGracePeriodSeconds(42).ApplyToClean(co)
		gp := int64(42)
		Expect(co.FinalizeGracePeriodSeconds).To(Equal(&gp))
	})

	It("should allow setting ErrorToleration", func() {
		co := &utilclient.CleanOptions{}
		utilclient.TolerateErrors{apierrors.IsConflict}.ApplyToClean(co)
		Expect(len(co.ErrorToleration)).To(Equal(1))
	})

	It("should merge multiple options together", func() {
		gp := int64(7)
		co := &utilclient.CleanOptions{}
		co.ApplyOptions([]utilclient.CleanOption{
			utilclient.ListWith{client.InNamespace("ns")},
			utilclient.FinalizeGracePeriodSeconds(gp),
		})
		Expect(co.ListOptions).To(Equal([]client.ListOption{client.InNamespace("ns")}))
		Expect(co.FinalizeGracePeriodSeconds).To(Equal(&gp))
	})
})
