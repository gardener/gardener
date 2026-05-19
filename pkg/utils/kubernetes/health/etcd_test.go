// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Etcd", func() {
	DescribeTable("Ready field",
		func(etcd *druidcorev1alpha1.Etcd, matcher types.GomegaMatcher) {
			Expect(health.CheckEtcd(etcd)).To(matcher)
		},
		Entry("nil", &druidcorev1alpha1.Etcd{}, MatchError(ContainSubstring("is not ready yet"))),
		Entry("false", &druidcorev1alpha1.Etcd{Status: druidcorev1alpha1.EtcdStatus{Ready: new(false)}}, MatchError(ContainSubstring("is not ready yet"))),
		Entry("true", &druidcorev1alpha1.Etcd{Status: druidcorev1alpha1.EtcdStatus{Ready: new(true)}}, BeNil()),
	)

	DescribeTable("Backup condition",
		func(backupReady *bool, matcher types.GomegaMatcher) {
			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name: "etcd-foo",
				},
				Status: druidcorev1alpha1.EtcdStatus{
					Ready: new(true),
				},
			}

			if backupReady != nil {
				var (
					message string
					status  = druidcorev1alpha1.ConditionTrue
				)
				if !*backupReady {
					message = "backup bucket is not accessible"
					status = druidcorev1alpha1.ConditionFalse
				}

				etcd.Status.Conditions = []druidcorev1alpha1.Condition{
					{
						Type:    druidcorev1alpha1.ConditionTypeBackupReady,
						Status:  status,
						Message: message,
					},
				}
			}

			Expect(health.CheckEtcd(etcd)).To(matcher)
		},
		Entry("no condition", nil, BeNil()),
		Entry("backup not ready", new(false), MatchError(ContainSubstring("backup for etcd \"etcd-foo\" is reported as unready"))),
		Entry("backup ready", new(true), BeNil()),
	)
})
