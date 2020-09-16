// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

type fakeControllerInstallationLister struct {
	gardencorelisters.ControllerInstallationLister

	listResult []*gardencorev1beta1.ControllerInstallation
	listErr    error
}

func newFakeControllerInstallationLister(controllerInstallationLister gardencorelisters.ControllerInstallationLister, listResult []*gardencorev1beta1.ControllerInstallation, listErr error) *fakeControllerInstallationLister {
	return &fakeControllerInstallationLister{
		ControllerInstallationLister: controllerInstallationLister,

		listResult: listResult,
		listErr:    listErr,
	}
}

func (c *fakeControllerInstallationLister) List(labels.Selector) ([]*gardencorev1beta1.ControllerInstallation, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	return c.listResult, nil
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
