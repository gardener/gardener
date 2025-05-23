// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("ReplicationController", func() {
	DescribeTable("CheckReplicationController",
		func(rc *corev1.ReplicationController, matcher types.GomegaMatcher) {
			err := health.CheckReplicationController(rc)
			Expect(err).To(matcher)
		},
		Entry("not observed at latest version", &corev1.ReplicationController{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
		Entry("not enough ready replicas", &corev1.ReplicationController{
			Spec:   corev1.ReplicationControllerSpec{Replicas: ptr.To[int32](2)},
			Status: corev1.ReplicationControllerStatus{ReadyReplicas: 1},
		}, HaveOccurred()),
		Entry("healthy", &corev1.ReplicationController{
			Spec:   corev1.ReplicationControllerSpec{Replicas: ptr.To[int32](2)},
			Status: corev1.ReplicationControllerStatus{ReadyReplicas: 2},
		}, BeNil()),
	)
})
