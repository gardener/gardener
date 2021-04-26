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

package controllerregistration

import (
	"testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/workqueue"
)

func TestControllerRegistration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerManager ControllerRegistration Controller Suite")
}

type fakeQueue struct {
	workqueue.RateLimitingInterface
	items []string
}

func (f *fakeQueue) Add(item interface{}) {
	f.items = append(f.items, item.(string))
}

func (f *fakeQueue) Len() int {
	return len(f.items)
}

type fakeSeedLister struct {
	gardencorelisters.SeedLister

	getResult  *gardencorev1beta1.Seed
	listResult []*gardencorev1beta1.Seed
	err        error
}

func newFakeSeedLister(seedLister gardencorelisters.SeedLister, getResult *gardencorev1beta1.Seed, listResult []*gardencorev1beta1.Seed, err error) *fakeSeedLister {
	return &fakeSeedLister{
		SeedLister: seedLister,

		getResult:  getResult,
		listResult: listResult,
		err:        err,
	}
}

func (c *fakeSeedLister) Get(string) (*gardencorev1beta1.Seed, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.getResult, nil
}

func (c *fakeSeedLister) List(labels.Selector) ([]*gardencorev1beta1.Seed, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.listResult, nil
}
