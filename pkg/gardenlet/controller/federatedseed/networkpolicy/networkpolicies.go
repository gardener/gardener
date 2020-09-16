// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	networkingv1 "k8s.io/api/networking/v1"
)

func (c *Controller) networkPolicyUpdate(_, newObj interface{}) {
	policy := newObj.(*networkingv1.NetworkPolicy)
	c.namespaceQueue.Add(policy.Namespace)
}

func (c *Controller) networkPolicyDelete(obj interface{}) {
	policy := obj.(*networkingv1.NetworkPolicy)
	c.namespaceQueue.Add(policy.Namespace)
}
