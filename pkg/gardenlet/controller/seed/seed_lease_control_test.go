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

package seed

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorescheme "github.com/gardener/gardener/pkg/client/core/clientset/versioned/scheme"
	gardencorev1beta1lister "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	mock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	mockrest "github.com/gardener/gardener/pkg/mock/client-go/rest"
	mockio "github.com/gardener/gardener/pkg/mock/go/io"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	fakerestclient "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("SeedLeaseControlReconcile", func() {
	var (
		ctrl            *gomock.Controller
		k8sGardenClient *mock.MockInterface
		k8sClient       client.Client
		seedRESTClient  *mockrest.MockInterface
		httpMockClient  *mockrest.MockHTTPClient
		body            *mockio.MockReadCloser
		indexer         cache.Indexer
		lister          gardencorev1beta1lister.SeedLister
		response        *http.Response

		seedName        = "test"
		seed            *gardencorev1beta1.Seed
		seedConditions  []gardencorev1beta1.Condition
		seedLeaseQueue  workqueue.RateLimitingInterface
		gardenletConfig *config.GardenletConfiguration
		controller      Controller
		syncFail        error
	)

	BeforeEach(func() {
		logger.Logger = logger.NewNopLogger()

		ctrl = gomock.NewController(GinkgoT())
		k8sGardenClient = mock.NewMockInterface(ctrl)
		seedRESTClient = mockrest.NewMockInterface(ctrl)
		httpMockClient = mockrest.NewMockHTTPClient(ctrl)

		seedLeaseQueue = workqueue.NewRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter())
		seedLeaseQueue.Add(seedName)

		k8sGardenClient.EXPECT().RESTClient().Return(seedRESTClient)

		body = mockio.NewMockReadCloser(ctrl)
		response = &http.Response{StatusCode: http.StatusOK, Body: body}
		fakeHTTPClient := fakerestclient.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			return response, nil
		})
		request := rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, fakeHTTPClient)
		seedRESTClient.EXPECT().Get().Return(request)
		body.EXPECT().Read(gomock.Any()).Return(0, io.EOF).AnyTimes()
		body.EXPECT().Close().AnyTimes()

		indexer = cache.NewIndexer(
			cache.MetaNamespaceKeyFunc,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

		gardenletConfig = &config.GardenletConfiguration{SeedClientConnection: &config.SeedClientConnection{}}
		lister = gardencorev1beta1lister.NewSeedLister(indexer)
	})

	JustBeforeEach(func() {
		httpMockClient.EXPECT().Do(gomock.Any()).Return(response, nil)

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: seedName},
			Status:     gardencorev1beta1.SeedStatus{Conditions: seedConditions},
		}

		k8sClient = fake.NewClientBuilder().WithScheme(gardencorescheme.Scheme).WithObjects(seed).Build()
		k8sGardenClient.EXPECT().Client().Return(k8sClient).AnyTimes()

		seedLeaseControl := &fakeLeaseController{
			err: syncFail,
		}

		healthManager := healthz.NewDefaultHealthz()
		healthManager.Start()

		clientMap := fakeclientmap.NewClientMap().
			AddClient(keys.ForGarden(), k8sGardenClient).
			AddClient(keys.ForSeed(seed), k8sGardenClient)
		controller = Controller{
			clientMap:        clientMap,
			seedLister:       lister,
			seedLeaseQueue:   seedLeaseQueue,
			config:           gardenletConfig,
			seedLeaseControl: seedLeaseControl,
			healthManager:    healthManager,
		}
		Expect(indexer.Add(seed)).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		seedLeaseQueue.ShutDown()
		syncFail = nil
	})

	Context("seed retrieval", func() {
		BeforeEach(func() {
			lister = &failingSeedLister{}
		})

		It("propagates the error when list request fails", func() {
			err := controller.reconcileSeedLeaseKey(seedName)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("#checkSeedConnection", func() {
		BeforeEach(func() {
			response = &http.Response{StatusCode: http.StatusInternalServerError, Body: body}
		})

		It("returns error if connection to Seed is unsuccessful", func() {
			err := controller.reconcileSeedLeaseKey(seedName)
			Expect(err).To(HaveOccurred())
			Expect(controller.healthManager.Get()).To(BeFalse())
		})
	})

	Context("#leaseSync fails", func() {
		BeforeEach(func() {
			syncFail = fmt.Errorf("dummy error")
		})

		It("propagates the error when lease fails to update", func() {
			err := controller.reconcileSeedLeaseKey(seedName)
			Expect(err).To(HaveOccurred())
			Expect(controller.healthManager.Get()).To(BeFalse())
		})
	})

	Context("#leaseSync", func() {
		It("updates the lease without error", func() {
			err := controller.reconcileSeedLeaseKey(seedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.healthManager.Get()).To(BeTrue())
		})
	})

	Context("#updateSeedCondition", func() {
		BeforeEach(func() {
			seedConditions = []gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.SeedGardenletReady,
					Status: gardencorev1beta1.ConditionUnknown,
				},
			}
		})

		It("updated seed Condition if already exists", func() {
			err := controller.reconcileSeedLeaseKey(seedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.healthManager.Get()).To(BeTrue())
		})
	})
})

type failingSeedLister struct{}

func (f *failingSeedLister) List(selector labels.Selector) (ret []*gardencorev1beta1.Seed, err error) {
	return nil, fmt.Errorf("dummy error")
}

func (f *failingSeedLister) Get(name string) (*gardencorev1beta1.Seed, error) {
	return nil, fmt.Errorf("dummy error")
}

type fakeLeaseController struct {
	err error
}

func (l *fakeLeaseController) Sync(holderIdentity string, ownerRef ...metav1.OwnerReference) error {
	return l.err
}
