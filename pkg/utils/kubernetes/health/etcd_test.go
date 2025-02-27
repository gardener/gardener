// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	druidv1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Etcd", func() {
	DescribeTable("Ready field",
		func(etcd *druidv1alpha1.Etcd, matcher types.GomegaMatcher) {
			Expect(health.CheckEtcd(etcd)).To(matcher)
		},
		Entry("nil", &druidv1alpha1.Etcd{}, MatchError(ContainSubstring("is not ready yet"))),
		Entry("false", &druidv1alpha1.Etcd{Status: druidv1alpha1.EtcdStatus{Ready: ptr.To(false)}}, MatchError(ContainSubstring("is not ready yet"))),
		Entry("true", &druidv1alpha1.Etcd{Status: druidv1alpha1.EtcdStatus{Ready: ptr.To(true)}}, BeNil()),
	)

	DescribeTable("Backup condition",
		func(backupReady *bool, matcher types.GomegaMatcher) {
			etcd := &druidv1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name: "etcd-foo",
				},
				Status: druidv1alpha1.EtcdStatus{
					Ready: ptr.To(true),
				},
			}

			if backupReady != nil {
				var (
					message string
					status  = druidv1alpha1.ConditionTrue
				)
				if !*backupReady {
					message = "backup bucket is not accessible"
					status = druidv1alpha1.ConditionFalse
				}

				etcd.Status.Conditions = []druidv1alpha1.Condition{
					{
						Type:    druidv1alpha1.ConditionTypeBackupReady,
						Status:  status,
						Message: message,
					},
				}
			}

			Expect(health.CheckEtcd(etcd)).To(matcher)
		},
		Entry("no condition", nil, BeNil()),
		Entry("backup not ready", ptr.To(false), MatchError(ContainSubstring("backup for etcd \"etcd-foo\" is reported as unready"))),
		Entry("backup ready", ptr.To(true), BeNil()),
	)
})
