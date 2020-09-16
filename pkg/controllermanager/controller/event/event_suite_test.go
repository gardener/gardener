// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/util/workqueue"
)

func TestProject(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerManager Event Controller Suite")
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
