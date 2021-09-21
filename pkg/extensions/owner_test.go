// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://wwr.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package extensions_test

import (
	"context"
	"errors"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/extensions"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	shootName = "foo"
	dnsName   = shootName + "-owner"
	namespace = "test"
	ownerName = "foo.example.com"
	ownerID   = "foo"
)

var _ = Describe("owner", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		ctx  context.Context
		dns  *extensionsv1alpha1.DNSRecord
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		ctx = context.TODO()

		dns = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dnsName,
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.DNSRecordSpec{
				Name:   ownerName,
				Values: []string{ownerID},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		expectGetDNSRecord = func(err error) {
			c.EXPECT().Get(ctx, kutil.Key(namespace, dnsName), gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.DNSRecord) error {
					if err == nil {
						*obj = *dns
					}
					return err
				},
			)
		}
	)

	Describe("GetOwnerNameAndID", func() {
		It("should return the correct owner name and ID if getting the owner DNSRecord succeeds", func() {
			expectGetDNSRecord(nil)

			name, id, err := GetOwnerNameAndID(ctx, c, namespace, shootName)
			Expect(err).To(Not(HaveOccurred()))
			Expect(name).To(Equal(ownerName))
			Expect(id).To(Equal(ownerID))
		})

		It("should return empty owner name and ID if the owner DNSRecord is not found", func() {
			expectGetDNSRecord(apierrors.NewNotFound(schema.GroupResource{}, dnsName))

			name, id, err := GetOwnerNameAndID(ctx, c, namespace, shootName)
			Expect(err).To(Not(HaveOccurred()))
			Expect(name).To(BeEmpty())
			Expect(id).To(BeEmpty())
		})

		It("should fail if getting the owner DNSRecord fails", func() {
			expectGetDNSRecord(errors.New("test"))

			_, _, err := GetOwnerNameAndID(ctx, c, namespace, shootName)
			Expect(err).To(HaveOccurred())
		})
	})
})
