// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dns_test

import (
	"errors"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dns"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var _ = Describe("#CheckDNSObject", func() {
	var (
		obj client.Object
		acc Object
	)

	It("should return error for non-dns object", func() {
		Expect(CheckDNSObject(&corev1.ConfigMap{})).To(HaveOccurred())
	})

	test := func() {
		It("should return error if observedGeneration is outdated", func() {
			acc.SetGeneration(1)
			acc.SetObservedGeneration(0)
			Expect(CheckDNSObject(obj)).To(MatchError(ContainSubstring("observed generation")))
		})

		It("should return error if state is not ready", func() {
			for _, state := range []string{
				dnsv1alpha1.STATE_PENDING,
				dnsv1alpha1.STATE_ERROR,
				dnsv1alpha1.STATE_INVALID,
				dnsv1alpha1.STATE_STALE,
				dnsv1alpha1.STATE_DELETING,
			} {
				acc.SetState(state)
				Expect(CheckDNSObject(obj)).To(MatchError(ContainSubstring(state)), "state: "+state)
			}
		})

		It("should include status.message in error message", func() {
			msg := "invalid credentials"
			acc.SetState(dnsv1alpha1.STATE_ERROR)
			acc.SetMessage(&msg)
			Expect(CheckDNSObject(obj)).To(MatchError(ContainSubstring(msg)))
		})

		It("should detect error code, if status is Error or Invalid", func() {
			for _, state := range []string{
				dnsv1alpha1.STATE_PENDING,
				dnsv1alpha1.STATE_ERROR,
				dnsv1alpha1.STATE_INVALID,
				dnsv1alpha1.STATE_STALE,
				dnsv1alpha1.STATE_DELETING,
			} {
				msg := "failed to call DNS API: Unauthorized"
				acc.SetState(state)
				acc.SetMessage(&msg)
				err := CheckDNSObject(obj)

				if state == dnsv1alpha1.STATE_ERROR || state == dnsv1alpha1.STATE_INVALID {
					Expect(retry.IsRetriable(err)).To(BeTrue(), "state: "+state)

					var errorWithCodes *helper.ErrorWithCodes
					Expect(errors.As(err, &errorWithCodes)).To(BeTrue(), "state: "+state)
					Expect(errorWithCodes.Codes()).To(ConsistOf(gardencorev1beta1.ErrorInfraUnauthorized), "state: "+state)
				} else {
					Expect(retry.IsRetriable(err)).To(BeFalse())
				}
			}
		})

		It("should return a retriable error, if status is Error or Invalid, even if no code is determined", func() {
			for _, state := range []string{
				dnsv1alpha1.STATE_ERROR,
				dnsv1alpha1.STATE_INVALID,
			} {
				msg := "unknown owner id 'foo--bar'"
				acc.SetState(state)
				acc.SetMessage(&msg)
				err := CheckDNSObject(obj)

				Expect(err).To(HaveOccurred(), "state: "+state)
				Expect(retry.IsRetriable(err)).To(BeTrue(), "state: "+state)
			}
		})

		It("should annotate error with state of dns object", func() {
			for _, state := range []string{
				dnsv1alpha1.STATE_ERROR,
				dnsv1alpha1.STATE_INVALID,
			} {
				msg := "unknown owner id 'foo--bar'"
				acc.SetState(state)
				acc.SetMessage(&msg)
				err := CheckDNSObject(obj)

				var errorWithState ErrorWithDNSState
				Expect(errors.As(err, &errorWithState)).To(BeTrue(), "state: "+state)
				Expect(errorWithState.DNSState()).To(Equal(state), "state: "+state)
			}
		})

		It("should not return error if object is ready", func() {
			acc.SetGeneration(1)
			acc.SetObservedGeneration(1)
			acc.SetState(dnsv1alpha1.STATE_READY)
			Expect(CheckDNSObject(obj)).To(Succeed())
		})
	}

	Context("#DNSProvider", func() {
		BeforeEach(func() {
			obj = &dnsv1alpha1.DNSProvider{}

			var err error
			acc, err = Accessor(obj)
			Expect(err).NotTo(HaveOccurred())
		})

		test()
	})

	Context("#DNSEntry", func() {
		BeforeEach(func() {
			obj = &dnsv1alpha1.DNSEntry{}

			var err error
			acc, err = Accessor(obj)
			Expect(err).NotTo(HaveOccurred())
		})

		test()
	})
})
