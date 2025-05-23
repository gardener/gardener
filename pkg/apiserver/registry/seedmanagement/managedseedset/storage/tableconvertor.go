// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
)

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

type convertor struct {
	headers []metav1beta1.TableColumnDefinition
}

func newTableConvertor() rest.TableConvertor {
	return &convertor{
		headers: []metav1beta1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Ready", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["ready"]},
			{Name: "Pending", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["pending"]},
			{Name: "Age", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
		},
	}
}

// ConvertToTable converts the output to a table.
func (c *convertor) ConvertToTable(_ context.Context, obj runtime.Object, _ runtime.Object) (*metav1beta1.Table, error) {
	var (
		err   error
		table = &metav1beta1.Table{
			ColumnDefinitions: c.headers,
		}
	)

	if m, err := meta.ListAccessor(obj); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.Continue = m.GetContinue()
	} else {
		if m, err := meta.CommonAccessor(obj); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
		}
	}

	table.Rows, err = metatable.MetaToTableRow(obj, func(obj runtime.Object, _ metav1.Object, _, _ string) ([]any, error) {
		var (
			managedSeedSet = obj.(*seedmanagement.ManagedSeedSet)
			cells          = []any{}
		)

		pending := ""
		if managedSeedSet.Status.PendingReplica != nil {
			pending = fmt.Sprintf("%s:%s", managedSeedSet.Status.PendingReplica.Name, managedSeedSet.Status.PendingReplica.Reason)
		}

		cells = append(cells, managedSeedSet.Name)
		cells = append(cells, fmt.Sprintf("%d/%d", managedSeedSet.Status.ReadyReplicas, *managedSeedSet.Spec.Replicas))
		cells = append(cells, pending)
		cells = append(cells, metatable.ConvertToHumanReadableDateType(managedSeedSet.CreationTimestamp))

		return cells, nil
	})

	return table, err
}
