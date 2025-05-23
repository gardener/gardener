// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hostnameresolver

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func TestHostnameResolver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardenlet Controller NetworkPolicy HostnameResolver Suite")
}

type fakeLookup struct {
	addrs []string
	err   error
	lock  sync.Mutex
}

func (f *fakeLookup) LookupHost(_ context.Context, _ string) ([]string, error) {
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
			log:           logr.Discard(),
			onUpdate: func() {
				updateCount++
			},
		}
	})

	It("should return correct subset", func() {
		go func() {
			Eventually(func() bool {
				return r.HasSynced()
			}).Should(BeTrue(), "HasSync should be true after start")
			Eventually(func() uint8 {
				return updateCount
			}).Should(BeEquivalentTo(1), "update should be called once")

			Expect(r.Subset()).To(ConsistOf(corev1.EndpointSubset{
				Addresses: []corev1.EndpointAddress{
					{IP: "1.2.3.4"}, {IP: "5.6.7.8"},
				},
				Ports: []corev1.EndpointPort{{Protocol: corev1.ProtocolTCP, Port: 1234}},
			}))

			cancelFunc()
		}()
		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false before starting")
		Expect(r.Start(ctx)).To(Succeed())
	})

	It("should not return that it has synced because it was not started", func() {
		Expect(updateCount).To(BeEquivalentTo(0), "update should not be called")
		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false")
	})

	It("should not return that it has synced if error occurs", func() {
		go func() {
			Consistently(func() bool {
				return r.HasSynced()
			}).Should(BeFalse(), "HasSync should always be false")
			Consistently(func() uint8 {
				return updateCount
			}).Should(BeZero(), "update should never be called")

			cancelFunc()
		}()
		f.setAddrs(nil)
		f.setErr(errors.New("some-error"))
		Expect(r.Start(ctx)).To(Succeed())
	})

	It("should return correct subset after resync", func() {
		go func() {
			Eventually(func() bool {
				return r.HasSynced()
			}).Should(BeTrue(), "HasSync should be true after start")

			Eventually(func() uint8 {
				return updateCount
			}).Should(BeEquivalentTo(1), "update should be called")

			Consistently(func() []corev1.EndpointSubset {
				return r.Subset()
			}).Should(ConsistOf(corev1.EndpointSubset{
				Addresses: []corev1.EndpointAddress{
					{IP: "1.2.3.4"}, {IP: "5.6.7.8"},
				},
				Ports: []corev1.EndpointPort{{Protocol: corev1.ProtocolTCP, Port: 1234}},
			}))

			f.setAddrs([]string{"5.6.7.8"})

			Eventually(func() []corev1.EndpointSubset {
				return r.Subset()
			}).Should(ConsistOf(corev1.EndpointSubset{
				Addresses: []corev1.EndpointAddress{{IP: "5.6.7.8"}},
				Ports:     []corev1.EndpointPort{{Protocol: corev1.ProtocolTCP, Port: 1234}},
			}))

			Expect(updateCount).To(BeEquivalentTo(2), "update should be called twice")

			cancelFunc()
		}()
		Expect(r.HasSynced()).To(BeFalse(), "HasSync should be false before starting")
		Expect(r.Start(ctx)).To(Succeed())
	})
})

var _ = Describe("CreateForCluster", func() {
	It("uses client host", func() {
		p, err := CreateForCluster(&rest.Config{Host: "https://foo.bar:1234"}, logr.Discard())
		Expect(err).NotTo(HaveOccurred())
		Expect(p).NotTo(BeNil())

		v, ok := p.(*resolver)
		Expect(ok).To(BeTrue(), "cast to resolver succeeds")
		Expect(v.upstreamFQDN).To(Equal("foo.bar"))
		Expect(v.upstreamPort).To(BeEquivalentTo(1234))
	})

	It("uses environment variable", func() {
		var (
			restConfig                 = &rest.Config{Host: "https://1.2.3.4:1234"}
			existingHost, existingPort = os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		)

		Expect(os.Setenv("KUBERNETES_SERVICE_HOST", "baz.bar")).To(Succeed())
		Expect(os.Setenv("KUBERNETES_SERVICE_PORT", "4321")).To(Succeed())

		defer func() {
			if existingHost != "" {
				Expect(os.Setenv("KUBERNETES_SERVICE_HOST", existingHost)).To(Succeed())
			}
			if existingPort != "" {
				Expect(os.Setenv("KUBERNETES_SERVICE_PORT", existingPort)).To(Succeed())
			}
		}()

		p, err := CreateForCluster(restConfig, logr.Discard())
		Expect(err).NotTo(HaveOccurred())
		Expect(p).NotTo(BeNil())

		v, ok := p.(*resolver)
		Expect(ok).To(BeTrue(), "cast to resolver succeeds")
		Expect(v.upstreamFQDN).To(Equal("baz.bar"))
		Expect(v.upstreamPort).To(BeEquivalentTo(4321))
	})

	It("does nothing", func() {
		var (
			restConfig                 = &rest.Config{Host: "https://1.2.3.4:1234"}
			existingHost, existingPort = os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		)

		Expect(os.Setenv("KUBERNETES_SERVICE_HOST", "5.6.7.8")).To(Succeed())
		Expect(os.Setenv("KUBERNETES_SERVICE_PORT", "4321")).To(Succeed())

		defer func() {
			if existingHost != "" {
				Expect(os.Setenv("KUBERNETES_SERVICE_HOST", existingHost)).To(Succeed())
			}
			if existingPort != "" {
				Expect(os.Setenv("KUBERNETES_SERVICE_PORT", existingPort)).To(Succeed())
			}
		}()

		p, err := CreateForCluster(restConfig, logr.Discard())
		Expect(err).NotTo(HaveOccurred())
		Expect(p).NotTo(BeNil())

		_, ok := p.(*noOpResolver)
		Expect(ok).To(BeTrue(), "cast to noOpResolver succeeds")
	})
})
