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
	. "github.com/onsi/gomega/gstruct"
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

var _ = Describe("Actuator", func() {
	var (
		log       logr.Logger
		dnsClient *fakeDNSClient

		actuator extensionsdnsrecordcontroller.Actuator

		cluster   *extensionscontroller.Cluster
		dnsRecord *extensionsv1alpha1.DNSRecord
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter))
		dnsClient = &fakeDNSClient{resp: successResponse()}

		actuator = &Actuator{
			DNSClient: dnsClient,
		}

		cluster = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{},
			Seed:  &gardencorev1beta1.Seed{},
		}

		dnsRecord = &extensionsv1alpha1.DNSRecord{
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
		It("should send DNS UPDATE for A records", func(ctx SpecContext) {
			Expect(actuator.Reconcile(ctx, log, dnsRecord, cluster)).To(Succeed())

			Expect(dnsClient.messages).To(HaveLen(1))
			msg := dnsClient.messages[0]

			// Zone section
			Expect(msg.Question).To(HaveLen(1))
			Expect(msg.Question[0].Name).To(Equal("local.gardener.cloud."))

			// Ns section contains the resource records to update
			Expect(msg.Ns).To(HaveExactElements(
				// First RR: RemoveRRset (class ANY)
				PointTo(MatchAllFields(Fields{
					"Hdr": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal("api.something.local.gardener.cloud."),
						"Class":  BeEquivalentTo(dns.ClassANY),
						"Rrtype": Equal(dns.TypeA),
						"Ttl":    BeEquivalentTo(0),
					}),
				})),
				// Other RRs: Insert (class INET)
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Hdr": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal("api.something.local.gardener.cloud."),
						"Class":  BeEquivalentTo(dns.ClassINET),
						"Rrtype": Equal(dns.TypeA),
						"Ttl":    BeEquivalentTo(123),
					}),
					"A": MatchRegexp(`1\.2\.3\.4`),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Hdr": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal("api.something.local.gardener.cloud."),
						"Class":  BeEquivalentTo(dns.ClassINET),
						"Rrtype": Equal(dns.TypeA),
						"Ttl":    BeEquivalentTo(123),
					}),
					"A": MatchRegexp(`5\.6\.7\.8`),
				})),
			))
		})

		It("should send DNS UPDATE for AAAA records", func(ctx SpecContext) {
			dnsRecord.Spec.RecordType = extensionsv1alpha1.DNSRecordTypeAAAA
			dnsRecord.Spec.Values = []string{"2001:db8::1"}

			Expect(actuator.Reconcile(ctx, log, dnsRecord, cluster)).To(Succeed())

			Expect(dnsClient.messages).To(HaveLen(1))
			msg := dnsClient.messages[0]

			// Zone section
			Expect(msg.Question).To(HaveLen(1))
			Expect(msg.Question[0].Name).To(Equal("local.gardener.cloud."))

			// Ns section contains the resource records to update
			Expect(msg.Ns).To(HaveExactElements(
				// First RR: RemoveRRset (class ANY)
				PointTo(MatchAllFields(Fields{
					"Hdr": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal("api.something.local.gardener.cloud."),
						"Class":  BeEquivalentTo(dns.ClassANY),
						"Rrtype": Equal(dns.TypeAAAA),
						"Ttl":    BeEquivalentTo(0),
					}),
				})),
				// Other RRs: Insert (class INET)
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Hdr": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal("api.something.local.gardener.cloud."),
						"Class":  BeEquivalentTo(dns.ClassINET),
						"Rrtype": Equal(dns.TypeAAAA),
						"Ttl":    BeEquivalentTo(123),
					}),
					"AAAA": MatchRegexp(`2001:db8::1`),
				})),
			))
		})

		It("should send DNS UPDATE for CNAME records", func(ctx SpecContext) {
			dnsRecord.Spec.RecordType = extensionsv1alpha1.DNSRecordTypeCNAME
			dnsRecord.Spec.Values = []string{"some.other.name.gardener.cloud"}

			Expect(actuator.Reconcile(ctx, log, dnsRecord, cluster)).To(Succeed())

			Expect(dnsClient.messages).To(HaveLen(1))
			msg := dnsClient.messages[0]

			// Zone section
			Expect(msg.Question).To(HaveLen(1))
			Expect(msg.Question[0].Name).To(Equal("local.gardener.cloud."))

			// Ns section contains the resource records to update
			Expect(msg.Ns).To(HaveExactElements(
				// First RR: RemoveRRset (class ANY)
				PointTo(MatchAllFields(Fields{
					"Hdr": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal("api.something.local.gardener.cloud."),
						"Class":  BeEquivalentTo(dns.ClassANY),
						"Rrtype": Equal(dns.TypeCNAME),
						"Ttl":    BeEquivalentTo(0),
					}),
				})),
				// Other RRs: Insert (class INET)
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Hdr": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal("api.something.local.gardener.cloud."),
						"Class":  BeEquivalentTo(dns.ClassINET),
						"Rrtype": Equal(dns.TypeCNAME),
						"Ttl":    BeEquivalentTo(123),
					}),
					"Target": Equal("some.other.name.gardener.cloud."),
				})),
			))
		})

		It("should use default TTL when none is specified", func(ctx SpecContext) {
			dnsRecord.Spec.TTL = nil

			Expect(actuator.Reconcile(ctx, log, dnsRecord, cluster)).To(Succeed())

			Expect(dnsClient.messages).To(HaveLen(1))
			// Insert RR should use default TTL of 120
			Expect(dnsClient.messages[0].Ns[1].Header().Ttl).To(Equal(uint32(120)))
		})
	})

	Describe("Delete", func() {
		It("should send DNS UPDATE with RemoveRRset", func(ctx SpecContext) {
			Expect(actuator.Delete(ctx, log, dnsRecord, cluster)).To(Succeed())

			Expect(dnsClient.messages).To(HaveLen(1))
			msg := dnsClient.messages[0]

			// Zone section
			Expect(msg.Question).To(HaveLen(1))
			Expect(msg.Question[0].Name).To(Equal("local.gardener.cloud."))

			// Ns section contains the resource records to update
			Expect(msg.Ns).To(HaveExactElements(
				// First RR: RemoveRRset (class ANY)
				PointTo(MatchAllFields(Fields{
					"Hdr": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal("api.something.local.gardener.cloud."),
						"Class":  BeEquivalentTo(dns.ClassANY),
						"Rrtype": Equal(dns.TypeA),
						"Ttl":    BeEquivalentTo(0),
					}),
				})),
			))
		})
	})

	Describe("Migrate", func() {
		It("should delete for non-self-hosted shoots", func(ctx SpecContext) {
			Expect(actuator.Migrate(ctx, log, dnsRecord, cluster)).To(Succeed())

			Expect(dnsClient.messages).To(HaveLen(1))
			msg := dnsClient.messages[0]

			// Zone section
			Expect(msg.Question).To(HaveLen(1))
			Expect(msg.Question[0].Name).To(Equal("local.gardener.cloud."))

			// Ns section contains the resource records to update
			Expect(msg.Ns).To(HaveExactElements(
				// First RR: RemoveRRset (class ANY)
				PointTo(MatchAllFields(Fields{
					"Hdr": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal("api.something.local.gardener.cloud."),
						"Class":  BeEquivalentTo(dns.ClassANY),
						"Rrtype": Equal(dns.TypeA),
						"Ttl":    BeEquivalentTo(0),
					}),
				})),
			))
		})

		It("should not delete for self-hosted shoots", func(ctx SpecContext) {
			cluster.Shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{
				ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
			}}

			Expect(actuator.Migrate(ctx, log, dnsRecord, cluster)).To(Succeed())
			Expect(dnsClient.messages).To(BeEmpty())
		})
	})
})

// fakeDNSClient captures DNS messages sent via Exchange and returns configurable responses.
type fakeDNSClient struct {
	messages []*dns.Msg
	resp     *dns.Msg
	err      error
}

func (m *fakeDNSClient) Exchange(msg *dns.Msg, _ string) (*dns.Msg, time.Duration, error) {
	m.messages = append(m.messages, msg.Copy())
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.resp, 0, nil
}

func successResponse() *dns.Msg {
	return &dns.Msg{MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess}}
}
