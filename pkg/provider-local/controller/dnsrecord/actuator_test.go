// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsdnsrecordcontroller "github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/provider-local/controller/dnsrecord"
)

// mockDNSClient captures DNS messages sent via Exchange and returns configurable responses.
type mockDNSClient struct {
	messages []*dns.Msg
	resp     *dns.Msg
	err      error
}

func (m *mockDNSClient) Exchange(msg *dns.Msg, _ string) (*dns.Msg, time.Duration, error) {
	m.messages = append(m.messages, msg.Copy())
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.resp, 0, nil
}

func successResponse() *dns.Msg {
	return &dns.Msg{MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess}}
}

var _ = Describe("Actuator", func() {
	var (
		log  logr.Logger
		mock *mockDNSClient

		actuator extensionsdnsrecordcontroller.Actuator

		cluster      *extensionscontroller.Cluster
		apiDNSRecord *extensionsv1alpha1.DNSRecord
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter))
		mock = &mockDNSClient{resp: successResponse()}

		actuator = &Actuator{
			DNSClient: mock,
		}

		cluster = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{},
			Seed:  &gardencorev1beta1.Seed{},
		}

		apiDNSRecord = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
			},
			Spec: extensionsv1alpha1.DNSRecordSpec{
				Name:       "api.something.local.gardener.cloud",
				RecordType: extensionsv1alpha1.DNSRecordTypeA,
				Values:     []string{"1.2.3.4", "5.6.7.8"},
				TTL:        ptr.To[int64](123),
			},
		}
	})

	Describe("Reconcile", func() {
		It("should send DNS UPDATE with RemoveRRset and Insert for A records", func(ctx SpecContext) {
			Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).To(Succeed())

			Expect(mock.messages).To(HaveLen(1))
			msg := mock.messages[0]

			// Zone section
			Expect(msg.Question).To(HaveLen(1))
			Expect(msg.Question[0].Name).To(Equal("local.gardener.cloud."))

			// Ns section contains the RemoveRRset + Insert RRs
			// RemoveRRset: 1 ANY RR for removing the RRset
			// Insert: 2 A RRs
			Expect(msg.Ns).To(HaveLen(3))

			// First RR: RemoveRRset (class ANY, type A)
			Expect(msg.Ns[0].Header().Name).To(Equal("api.something.local.gardener.cloud."))
			Expect(msg.Ns[0].Header().Rrtype).To(Equal(dns.TypeA))
			Expect(msg.Ns[0].Header().Class).To(Equal(uint16(dns.ClassANY)))

			// Second and third RRs: Insert (class INET, type A)
			for i, expectedIP := range []string{"1.2.3.4", "5.6.7.8"} {
				rr := msg.Ns[1+i]
				Expect(rr.Header().Name).To(Equal("api.something.local.gardener.cloud."))
				Expect(rr.Header().Rrtype).To(Equal(dns.TypeA))
				Expect(rr.Header().Class).To(Equal(uint16(dns.ClassINET)))
				Expect(rr.Header().Ttl).To(Equal(uint32(123)))
				aRR, ok := rr.(*dns.A)
				Expect(ok).To(BeTrue())
				Expect(aRR.A.String()).To(Equal(expectedIP))
			}
		})

		It("should send DNS UPDATE for AAAA records", func(ctx SpecContext) {
			apiDNSRecord.Spec.RecordType = extensionsv1alpha1.DNSRecordTypeAAAA
			apiDNSRecord.Spec.Values = []string{"2001:db8::1"}

			Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).To(Succeed())

			Expect(mock.messages).To(HaveLen(1))
			msg := mock.messages[0]

			// RemoveRRset + 1 Insert
			Expect(msg.Ns).To(HaveLen(2))

			rr := msg.Ns[1]
			Expect(rr.Header().Rrtype).To(Equal(dns.TypeAAAA))
			aaaaRR, ok := rr.(*dns.AAAA)
			Expect(ok).To(BeTrue())
			Expect(aaaaRR.AAAA.String()).To(Equal("2001:db8::1"))
		})

		It("should send DNS UPDATE for CNAME records", func(ctx SpecContext) {
			apiDNSRecord.Spec.RecordType = extensionsv1alpha1.DNSRecordTypeCNAME
			apiDNSRecord.Spec.Values = []string{"some.other.name.gardener.cloud"}

			Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).To(Succeed())

			Expect(mock.messages).To(HaveLen(1))
			msg := mock.messages[0]

			Expect(msg.Ns).To(HaveLen(2))

			rr := msg.Ns[1]
			Expect(rr.Header().Rrtype).To(Equal(dns.TypeCNAME))
			cnameRR, ok := rr.(*dns.CNAME)
			Expect(ok).To(BeTrue())
			Expect(cnameRR.Target).To(Equal("some.other.name.gardener.cloud."))
		})

		It("should use default TTL when none is specified", func(ctx SpecContext) {
			apiDNSRecord.Spec.TTL = nil

			Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).To(Succeed())

			Expect(mock.messages).To(HaveLen(1))
			// Insert RR should use default TTL of 120
			Expect(mock.messages[0].Ns[1].Header().Ttl).To(Equal(uint32(120)))
		})
	})

	Describe("Delete", func() {
		It("should send DNS UPDATE with RemoveName", func(ctx SpecContext) {
			Expect(actuator.Delete(ctx, log, apiDNSRecord, cluster)).To(Succeed())

			Expect(mock.messages).To(HaveLen(1))
			msg := mock.messages[0]

			Expect(msg.Question).To(HaveLen(1))
			Expect(msg.Question[0].Name).To(Equal("local.gardener.cloud."))

			// Ns section contains the RemoveName RR
			Expect(msg.Ns).To(HaveLen(1))
			Expect(msg.Ns[0].Header().Name).To(Equal("api.something.local.gardener.cloud."))
			Expect(msg.Ns[0].Header().Rrtype).To(Equal(dns.TypeANY))
			Expect(msg.Ns[0].Header().Class).To(Equal(uint16(dns.ClassANY)))
		})
	})

	Describe("Migrate", func() {
		It("should delete for non-self-hosted shoots", func(ctx SpecContext) {
			Expect(actuator.Migrate(ctx, log, apiDNSRecord, cluster)).To(Succeed())

			Expect(mock.messages).To(HaveLen(1))
			msg := mock.messages[0]
			Expect(msg.Ns).To(HaveLen(1))
			Expect(msg.Ns[0].Header().Name).To(Equal("api.something.local.gardener.cloud."))
		})

		It("should not delete for self-hosted shoots", func(ctx SpecContext) {
			cluster.Shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{
				ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
			}}

			Expect(actuator.Migrate(ctx, log, apiDNSRecord, cluster)).To(Succeed())
			Expect(mock.messages).To(BeEmpty())
		})
	})
})
