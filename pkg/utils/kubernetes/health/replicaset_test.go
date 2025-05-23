// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("ReplicaSet", func() {
	DescribeTable("CheckReplicaSet",
		func(rs *appsv1.ReplicaSet, matcher types.GomegaMatcher) {
			err := health.CheckReplicaSet(rs)
			Expect(err).To(matcher)
		},
		Entry("not observed at latest version", &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
		Entry("not enough ready replicas", &appsv1.ReplicaSet{
			Spec:   appsv1.ReplicaSetSpec{Replicas: ptr.To[int32](2)},
			Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1},
		}, HaveOccurred()),
		Entry("healthy", &appsv1.ReplicaSet{
			Spec:   appsv1.ReplicaSetSpec{Replicas: ptr.To[int32](2)},
			Status: appsv1.ReplicaSetStatus{ReadyReplicas: 2},
		}, BeNil()),
	)
})
