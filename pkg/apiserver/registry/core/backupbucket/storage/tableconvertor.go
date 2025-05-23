// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/core"
)

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

type convertor struct {
	headers []metav1beta1.TableColumnDefinition
}

func newTableConvertor() rest.TableConvertor {
	return &convertor{
		headers: []metav1beta1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Provider", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["provider"]},
			{Name: "Seed", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["seed"]},
			{Name: "Operation", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["operation"]},
			{Name: "Progress", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["progress"]},
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
			backupBucket = obj.(*core.BackupBucket)
			cells        = []any{}
		)

		cells = append(cells, backupBucket.Name)
		cells = append(cells, backupBucket.Spec.Provider.Type)
		cells = append(cells, backupBucket.Spec.SeedName)
		if lastOp := backupBucket.Status.LastOperation; lastOp != nil {
			cells = append(cells, lastOp.State)
			cells = append(cells, lastOp.Progress)
		} else {
			cells = append(cells, "<pending>")
			cells = append(cells, 0)
		}
		cells = append(cells, metatable.ConvertToHumanReadableDateType(backupBucket.CreationTimestamp))

		return cells, nil
	})

	return table, err
}
