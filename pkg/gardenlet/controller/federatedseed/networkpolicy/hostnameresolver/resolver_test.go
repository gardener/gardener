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
		updateCalled bool
		r            *resolver
		ctx          context.Context
		cancelFunc   context.CancelFunc
		f            *fakeLookup
		lock         sync.Mutex
	)
	BeforeEach(func() {
		updateCalled = false
		lock = sync.Mutex{}
		f = &fakeLookup{
			addrs: []string{"5.6.7.8", "1.2.3.4"}, // sorting check
		}
		r = &resolver{
			lookup:        f,
			upstreamPort:  1234,
			refreshTicker: time.NewTicker(time.Millisecond * 3),
			log:           logger.NewNopLogger(),
			onUpdate: func() {
				lock.Lock()
				updateCalled = true
				cancelFunc()
				lock.Unlock()
			},
		}
	})

	It("should return correct subset", func(done Done) {
		ctx, cancelFunc = context.WithTimeout(context.Background(), time.Millisecond*2)

		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false before starting")

		r.Start(ctx)

		Expect(r.HasSynced()).To(BeTrue(), "HasSync should be true after start")
		Expect(updateCalled).To(BeTrue(), "update should be called")

		Expect(r.Subset()).To(ConsistOf(corev1.EndpointSubset{
			Addresses: []corev1.EndpointAddress{
				{IP: "1.2.3.4"}, {IP: "5.6.7.8"},
			},
			Ports: []corev1.EndpointPort{{Protocol: corev1.ProtocolTCP, Port: 1234}},
		}))

		close(done)
	}, 0.2)

	It("should not return that it has synced", func(done Done) {
		ctx, cancelFunc = context.WithTimeout(context.Background(), time.Millisecond*2)

		Expect(updateCalled).To(BeFalse(), "update should not be called")
		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false")

		close(done)
	}, 0.2)

	It("should not return that it has synced if error occurs", func(done Done) {
		ctx, cancelFunc = context.WithTimeout(context.Background(), time.Millisecond*2)

		f.setAddrs(nil)
		f.setErr(errors.New("some-error"))

		r.Start(ctx)

		Expect(updateCalled).To(BeFalse(), "update should not be called")
		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false")

		close(done)
	}, 0.2)

	It("should return correct subset after resync", func(done Done) {
		ctx, cancelFunc = context.WithTimeout(context.Background(), time.Millisecond*10)
		r.onUpdate = func() {
			lock.Lock()
			updateCalled = true
			lock.Unlock()
		}

		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false before starting")

		go r.Start(ctx)

		Eventually(func() bool { //nolint:unlambda
			return r.HasSynced()
		}, time.Millisecond*3, time.Millisecond).Should(BeTrue(), "HasSync should be true after start")

		Eventually(func() bool { //nolint:unlambda
			lock.Lock()
			defer lock.Unlock()
			return updateCalled
		}, time.Millisecond*3, time.Millisecond).Should(BeTrue(), "update should be called")

		Consistently(func() []corev1.EndpointSubset { //nolint:unlambda
			return r.Subset()
		}, time.Millisecond*3, time.Millisecond).Should(ConsistOf(corev1.EndpointSubset{
			Addresses: []corev1.EndpointAddress{
				{IP: "1.2.3.4"}, {IP: "5.6.7.8"},
			},
			Ports: []corev1.EndpointPort{{Protocol: corev1.ProtocolTCP, Port: 1234}},
		}))

		f.setAddrs([]string{"5.6.7.8"})

		Eventually(func() []corev1.EndpointSubset { //nolint:unlambda
			return r.Subset()
		}, time.Millisecond*3, time.Millisecond).Should(ConsistOf(corev1.EndpointSubset{
			Addresses: []corev1.EndpointAddress{{IP: "5.6.7.8"}},
			Ports:     []corev1.EndpointPort{{Protocol: corev1.ProtocolTCP, Port: 1234}},
		}))

		cancelFunc()
		close(done)
	}, 0.2)
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
