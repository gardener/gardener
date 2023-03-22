// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health_test

import (
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Etcd", func() {
	DescribeTable("Ready field",
		func(etcd *druidv1alpha1.Etcd, matcher types.GomegaMatcher) {
			Expect(health.CheckEtcd(etcd)).To(matcher)
		},
		Entry("nil", &druidv1alpha1.Etcd{}, MatchError(ContainSubstring("is not ready yet"))),
		Entry("false", &druidv1alpha1.Etcd{Status: druidv1alpha1.EtcdStatus{Ready: pointer.Bool(false)}}, MatchError(ContainSubstring("is not ready yet"))),
		Entry("true", &druidv1alpha1.Etcd{Status: druidv1alpha1.EtcdStatus{Ready: pointer.Bool(true)}}, BeNil()),
	)

	DescribeTable("Backup condition",
		func(backupReady *bool, matcher types.GomegaMatcher) {
			etcd := &druidv1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name: "etcd-foo",
				},
				Status: druidv1alpha1.EtcdStatus{
					Ready: pointer.Bool(true),
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
		Entry("backup not ready", pointer.Bool(false), MatchError(ContainSubstring("backup for etcd \"etcd-foo\" is reported as unready"))),
		Entry("backup ready", pointer.Bool(true), BeNil()),
	)
})
