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

package hostnameresolver

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func TestHostnameresolver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hostnameresolver Suite")
}

type fakeLookup struct {
	addrs []string
	err   error
	lock  sync.Mutex
}

func (f *fakeLookup) LookupHost(ctx context.Context, host string) ([]string, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.addrs, f.err
}

func (f *fakeLookup) setAddrs(addrs []string) {
	f.lock.Lock()
	f.addrs = addrs
	f.lock.Unlock()
}

func (f *fakeLookup) setErr(err error) {
	f.lock.Lock()
	f.err = err
	f.lock.Unlock()
}

var _ = Describe("resolver", func() {
	var (
		updateCount uint8
		r           *resolver
		ctx         context.Context
		cancelFunc  context.CancelFunc
		f           *fakeLookup
	)
	BeforeEach(func() {
		ctx, cancelFunc = context.WithCancel(context.Background())
		updateCount = 0
		f = &fakeLookup{
			addrs: []string{"5.6.7.8", "1.2.3.4"}, // sorting check
		}
		r = &resolver{
			lookup:        f,
			upstreamPort:  1234,
			refreshTicker: time.NewTicker(time.Millisecond),
			log:           logger.NewNopLogger(),
			onUpdate: func() {
				updateCount++
			},
		}
	})

	It("should return correct subset", func(done Done) {
		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false before starting")

		go r.Start(ctx)

		Eventually(func() bool { //nolint:unlambda
			return r.HasSynced()
		}).Should(BeTrue(), "HasSync should be true after start")
		Eventually(func() uint8 { //nolint:unlambda
			return updateCount
		}).Should(BeEquivalentTo(1), "update should be called once")

		Expect(r.Subset()).To(ConsistOf(corev1.EndpointSubset{
			Addresses: []corev1.EndpointAddress{
				{IP: "1.2.3.4"}, {IP: "5.6.7.8"},
			},
			Ports: []corev1.EndpointPort{{Protocol: corev1.ProtocolTCP, Port: 1234}},
		}))

		cancelFunc()
		close(done)
	})

	It("should not return that it has synced because it was not started", func(done Done) {
		Expect(updateCount).To(BeEquivalentTo(0), "update should not be called")
		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false")

		cancelFunc()
		close(done)
	})

	It("should not return that it has synced if error occurs", func(done Done) {
		f.setAddrs(nil)
		f.setErr(errors.New("some-error"))

		go r.Start(ctx)
		cancelFunc()

		Eventually(func() bool { //nolint:unlambda
			return r.HasSynced()
		}).Should(BeFalse(), "HasSync should always be false")
		Eventually(func() uint8 { //nolint:unlambda
			return updateCount
		}).Should(BeZero(), "update should never be called")

		close(done)
	})

	It("should return correct subset after resync", func(done Done) {
		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false before starting")

		go r.Start(ctx)

		Eventually(func() bool { //nolint:unlambda
			return r.HasSynced()
		}).Should(BeTrue(), "HasSync should be true after start")

		Eventually(func() uint8 { //nolint:unlambda
			return updateCount
		}).Should(BeEquivalentTo(1), "update should be called")

		Consistently(func() []corev1.EndpointSubset { //nolint:unlambda
			return r.Subset()
		}).Should(ConsistOf(corev1.EndpointSubset{
			Addresses: []corev1.EndpointAddress{
				{IP: "1.2.3.4"}, {IP: "5.6.7.8"},
			},
			Ports: []corev1.EndpointPort{{Protocol: corev1.ProtocolTCP, Port: 1234}},
		}))

		f.setAddrs([]string{"5.6.7.8"})

		Eventually(func() []corev1.EndpointSubset { //nolint:unlambda
			return r.Subset()
		}).Should(ConsistOf(corev1.EndpointSubset{
			Addresses: []corev1.EndpointAddress{{IP: "5.6.7.8"}},
			Ports:     []corev1.EndpointPort{{Protocol: corev1.ProtocolTCP, Port: 1234}},
		}))

		Expect(updateCount).To(BeEquivalentTo(2), "update should be called twice")

		cancelFunc()
		close(done)
	})
})

var _ = Describe("CreateForCluster", func() {
	It("uses client host", func() {
		c := fake.NewClientSetBuilder().WithRESTConfig(&rest.Config{
			Host: "https://foo.bar:1234",
		}).Build()

		p, err := CreateForCluster(c, logger.NewNopLogger())
		Expect(err).NotTo(HaveOccurred())
		Expect(p).NotTo(BeNil())

		v, ok := p.(*resolver)
		Expect(ok).To(BeTrue(), "cast to resolver succeeds")
		Expect(v.upstreamFQDN).To(Equal("foo.bar"))
		Expect(v.upstreamPort).To(BeEquivalentTo(1234))
	})

	It("uses environment variable", func() {
		var (
			c = fake.NewClientSetBuilder().WithRESTConfig(&rest.Config{
				Host: "https://1.2.3.4:1234",
			}).Build()
			existingHost, existingPort = os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		)

		os.Setenv("KUBERNETES_SERVICE_HOST", "baz.bar")
		os.Setenv("KUBERNETES_SERVICE_PORT", "4321")

		defer func() {
			if existingHost != "" {
				Expect(os.Setenv("KUBERNETES_SERVICE_HOST", existingHost)).NotTo(HaveOccurred())
			}
			if existingPort != "" {
				Expect(os.Setenv("KUBERNETES_SERVICE_PORT", existingPort)).NotTo(HaveOccurred())
			}
		}()

		p, err := CreateForCluster(c, logger.NewNopLogger())
		Expect(err).NotTo(HaveOccurred())
		Expect(p).NotTo(BeNil())

		v, ok := p.(*resolver)
		Expect(ok).To(BeTrue(), "cast to resolver succeeds")
		Expect(v.upstreamFQDN).To(Equal("baz.bar"))
		Expect(v.upstreamPort).To(BeEquivalentTo(4321))
	})

	It("does nothing", func() {
		var (
			c = fake.NewClientSetBuilder().WithRESTConfig(&rest.Config{
				Host: "https://1.2.3.4:1234",
			}).Build()
			existingHost, existingPort = os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		)

		os.Setenv("KUBERNETES_SERVICE_HOST", "5.6.7.8")
		os.Setenv("KUBERNETES_SERVICE_PORT", "4321")

		defer func() {
			if existingHost != "" {
				Expect(os.Setenv("KUBERNETES_SERVICE_HOST", existingHost)).NotTo(HaveOccurred())
			}
			if existingPort != "" {
				Expect(os.Setenv("KUBERNETES_SERVICE_PORT", existingPort)).NotTo(HaveOccurred())
			}
		}()

		p, err := CreateForCluster(c, logger.NewNopLogger())
		Expect(err).NotTo(HaveOccurred())
		Expect(p).NotTo(BeNil())

		_, ok := p.(*noOpResover)
		Expect(ok).To(BeTrue(), "cast to noOpResover succeeds")
	})
})
