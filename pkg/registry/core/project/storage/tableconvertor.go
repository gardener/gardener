// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
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
			{Name: "Namespace", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["namespace"]},
			{Name: "Status", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["phase"]},
			{Name: "Owner", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["owner"]},
			{Name: "Creator", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["creator"]},
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

	table.Rows, err = metatable.MetaToTableRow(obj, func(obj runtime.Object, m metav1.Object, name, age string) ([]interface{}, error) {
		var (
			project = obj.(*core.Project)
			cells   = []interface{}{}
		)

		cells = append(cells, project.Name)
		if namespace := project.Spec.Namespace; namespace != nil {
			cells = append(cells, *namespace)
		} else {
			cells = append(cells, "<unknown>")
		}
		if phase := project.Status.Phase; len(phase) > 0 {
			cells = append(cells, phase)
		} else {
			cells = append(cells, "<unknown>")
		}
		if owner := project.Spec.Owner; owner != nil {
			cells = append(cells, owner.Name)
		} else {
			cells = append(cells, "<unknown>")
		}
		if createdBy := project.Spec.CreatedBy; createdBy != nil {
			cells = append(cells, createdBy.Name)
		} else {
			cells = append(cells, "<unknown>")
		}
		cells = append(cells, metatable.ConvertToHumanReadableDateType(project.CreationTimestamp))

		return cells, nil
	})

	return table, err
}
