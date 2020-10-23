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

package botanist_test

import (
	"context"
	"time"

	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("VolumeAttachments", func() {

	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		log  *logrus.Entry
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		log = logrus.NewEntry(logger.NewNopLogger())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#WaitUntilVolumeAttachmentsDeleted", func() {
		It("should return nil when there are no VolumeAttachments", func() {
			c.EXPECT().List(context.TODO(), gomock.AssignableToTypeOf(&storagev1beta1.VolumeAttachmentList{})).Return(nil)

			err := botanist.WaitUntilVolumeAttachmentsDeleted(context.TODO(), c, log)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return err when the context is cancelled", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&storagev1beta1.VolumeAttachmentList{})).DoAndReturn(func(_ context.Context, list *storagev1beta1.VolumeAttachmentList, _ ...client.ListOption) error {
				items := []storagev1beta1.VolumeAttachment{
					{ObjectMeta: metav1.ObjectMeta{Name: "csi-a8d88f2004683df6a875c361481931bbf033f6af92e39e60acf14f891d9c0731"}},
				}
				*list = storagev1beta1.VolumeAttachmentList{Items: items}
				return nil
			})

			err := botanist.WaitUntilVolumeAttachmentsDeleted(ctx, c, log)
			Expect(err).To(HaveOccurred())
		})
	})
})
