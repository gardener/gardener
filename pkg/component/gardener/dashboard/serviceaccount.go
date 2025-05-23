// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const serviceAccountNameTerminal = "dashboard-terminal-admin"

func (g *gardenerDashboard) serviceAccountTerminal() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountNameTerminal,
			Namespace: metav1.NamespaceSystem,
			Labels:    GetLabels(),
		},
	}
}
