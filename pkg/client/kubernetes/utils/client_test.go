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

package utils_test

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/utils"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("SpecificallyCachedReader", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme

		ctrl                      *gomock.Controller
		cacheReader, clientReader *mockclient.MockReader
		reader                    client.Reader
		restMapper                meta.RESTMapper

		key                            client.ObjectKey
		specifiedObj, unspecifiedObj   runtime.Object
		specifiedList, unspecifiedList runtime.Object
	)

	BeforeEach(func() {
		var err error
		ctx = context.TODO()
		scheme = runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(scheme))

		ctrl = gomock.NewController(GinkgoT())
		cacheReader = mockclient.NewMockReader(ctrl)
		clientReader = mockclient.NewMockReader(ctrl)
		restMapper, err = apiutil.NewDynamicRESTMapper(restConfig, apiutil.WithLazyDiscovery)
		Expect(err).NotTo(HaveOccurred())

		key = client.ObjectKey{Namespace: "default", Name: "bar"}
		specifiedObj = &corev1.ConfigMap{ObjectMeta: kubernetes.ObjectMetaFromKey(key)}
		specifiedList = &corev1.ConfigMapList{}
		unspecifiedObj = &corev1.Secret{ObjectMeta: kubernetes.ObjectMetaFromKey(key)}
		unspecifiedList = &corev1.SecretList{}

		By("creating empty objects in local control plane") // for testing read against real client
		c, err := client.New(restConfig, client.Options{Scheme: scheme, Mapper: restMapper})
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Create(ctx, specifiedObj)).To(Or(BeAlreadyExistsError(), Succeed()))
		Expect(c.Create(ctx, unspecifiedObj)).To(Or(BeAlreadyExistsError(), Succeed()))
		Expect(c.Get(ctx, key, specifiedObj)).To(Succeed())
		Expect(c.Get(ctx, key, unspecifiedObj)).To(Succeed())
		Expect(c.List(ctx, specifiedList)).To(Succeed())
		Expect(c.List(ctx, unspecifiedList)).To(Succeed())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	testReadDirectly := func(obj, list runtime.Object) {
		clientReader.EXPECT().Get(ctx, key, obj)
		err := reader.Get(ctx, key, obj)
		Expect(err).NotTo(HaveOccurred())

		clientReader.EXPECT().List(ctx, list)
		err = reader.List(ctx, list)
		Expect(err).NotTo(HaveOccurred())
	}
	testReadFromCache := func(obj, list runtime.Object) {
		cacheReader.EXPECT().Get(ctx, key, obj)
		err := reader.Get(ctx, key, obj)
		Expect(err).NotTo(HaveOccurred())

		cacheReader.EXPECT().List(ctx, list)
		err = reader.List(ctx, list)
		Expect(err).NotTo(HaveOccurred())
	}
	testReadFromLocalControlPlane := func(obj, list runtime.Object) {
		objFromAPI := obj.DeepCopyObject()
		err := reader.Get(ctx, key, objFromAPI)
		Expect(err).NotTo(HaveOccurred())
		Expect(objFromAPI).To(DeepDerivativeEqual(obj))

		listFromAPI := list.DeepCopyObject()
		err = reader.List(ctx, listFromAPI)
		Expect(err).NotTo(HaveOccurred())
		Expect(listFromAPI).To(DeepDerivativeEqual(list))
	}

	Describe("common behaviour", func() {
		It("should fail, if object is not registered in scheme", func() {
			_, err := utils.NewSpecificallyCachedReaderFor(cacheReader, clientReader, scheme, false, &appsv1.Deployment{})
			Expect(err).To(MatchError(ContainSubstring("no kind is registered for the type")))
		})
		It("should default scheme to scheme.Scheme", func() {
			_, err := utils.NewSpecificallyCachedReaderFor(cacheReader, clientReader, nil, false, &appsv1.Deployment{})
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.NewSpecificallyCachedReaderFor(cacheReader, clientReader, nil, false, &gardencorev1beta1.Shoot{})
			Expect(err).To(MatchError(ContainSubstring("no kind is registered for the type")))
		})
		Describe("#Get", func() {
			It("should fail, if object is not registered in scheme", func() {
				var err error
				reader, err = utils.NewSpecificallyCachedReaderFor(cacheReader, clientReader, scheme, false, specifiedObj)
				Expect(err).NotTo(HaveOccurred())

				err = reader.Get(ctx, key, &appsv1.Deployment{})
				Expect(err).To(MatchError(ContainSubstring("no kind is registered for the type")))
			})
		})
		Describe("#List", func() {
			It("should fail, if object is not registered in scheme", func() {
				var err error
				reader, err = utils.NewSpecificallyCachedReaderFor(cacheReader, clientReader, scheme, false, specifiedObj)
				Expect(err).NotTo(HaveOccurred())

				err = reader.List(ctx, &appsv1.DeploymentList{})
				Expect(err).To(MatchError(ContainSubstring("no kind is registered for the type")))
			})
		})
	})

	Describe("#NewReaderWithDisabledCacheFor", func() {
		BeforeEach(func() {
			var err error
			reader, err = utils.NewReaderWithDisabledCacheFor(cacheReader, clientReader, scheme, specifiedObj)
			Expect(err).NotTo(HaveOccurred())
			Expect(reader).NotTo(BeNil())
		})

		It("should read specified objects directly", func() {
			testReadDirectly(specifiedObj, specifiedList)
		})

		It("should read unspecified objects from cache", func() {
			testReadFromCache(unspecifiedObj, unspecifiedList)
		})
	})

	Describe("#NewReaderWithEnabledCacheFor", func() {
		BeforeEach(func() {
			var err error
			reader, err = utils.NewReaderWithEnabledCacheFor(cacheReader, clientReader, scheme, specifiedObj)
			Expect(err).NotTo(HaveOccurred())
			Expect(reader).NotTo(BeNil())
		})

		It("should read specified objects from cache", func() {
			testReadFromCache(specifiedObj, specifiedList)
		})

		It("should read unspecified objects directly", func() {
			testReadDirectly(unspecifiedObj, unspecifiedList)
		})
	})

	Describe("#NewClientWithSpecificallyCachedReader", func() {
		It("should fail, if client can't be created", func() {
			_, err := utils.NewClientWithSpecificallyCachedReader(nil, nil, client.Options{Scheme: scheme}, false)
			Expect(err).To(MatchError(ContainSubstring("must provide non-nil rest.Config to client.New")))
		})
		It("should fail, if SpecificallyCachedReader can't be created", func() {
			_, err := utils.NewClientWithSpecificallyCachedReader(fakeCache{Reader: cacheReader}, restConfig, client.Options{Scheme: scheme}, false, &appsv1.Deployment{})
			Expect(err).To(MatchError(ContainSubstring("no kind is registered for the type")))
		})
	})

	Describe("#NewClientFuncWithDisabledCacheFor", func() {
		BeforeEach(func() {
			c, err := utils.NewClientFuncWithDisabledCacheFor(specifiedObj)(fakeCache{Reader: cacheReader}, restConfig, client.Options{
				Scheme: scheme,
				Mapper: restMapper,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(c).NotTo(BeNil())
			reader = c
		})

		It("produced client should read specified objects directly", func() {
			testReadFromLocalControlPlane(specifiedObj, specifiedList)
		})

		It("produced client should read unspecified objects from cache", func() {
			testReadFromCache(unspecifiedObj, unspecifiedList)
		})
	})

	Describe("#NewClientFuncWithEnabledCacheFor", func() {
		BeforeEach(func() {
			c, err := utils.NewClientFuncWithEnabledCacheFor(specifiedObj)(fakeCache{Reader: cacheReader}, restConfig, client.Options{
				Scheme: scheme,
				Mapper: restMapper,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(c).NotTo(BeNil())
			reader = c
		})

		It("produced client should read specified objects from cache", func() {
			testReadFromCache(specifiedObj, specifiedList)
		})

		It("produced client should read unspecified objects directly", func() {
			testReadFromLocalControlPlane(unspecifiedObj, unspecifiedList)
		})
	})

})

type fakeCache struct {
	client.Reader
	cache.Informers
}
