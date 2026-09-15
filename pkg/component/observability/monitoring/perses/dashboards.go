// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (p *perses) dashboards() []client.Object {
	var objs []client.Object

	for name, config := range p.values.Dashboards {
		objs = append(objs, &persesv1alpha2.PersesDashboard{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: p.namespace,
				Labels:    p.getLabels(),
			},
			Spec: persesv1alpha2.PersesDashboardSpec{
				Config:           config,
				InstanceSelector: p.instanceSelector(),
			},
		})
	}

	return objs
}
