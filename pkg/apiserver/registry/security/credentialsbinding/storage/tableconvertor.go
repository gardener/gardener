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

	"github.com/gardener/gardener/pkg/apis/security"
)

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

type convertor struct {
	headers []metav1beta1.TableColumnDefinition
}

func newTableConvertor() rest.TableConvertor {
	return &convertor{
		headers: []metav1beta1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Provider", Type: "string", Description: "Provide is the provider type of the CredentialsBinding."},
			{Name: "APIVersion", Type: "string", Format: "name", Description: "APIVersion is the apiVersion of the referenced credentials provider."},
			{Name: "Kind", Type: "string", Format: "name", Description: "Kind is the kind of the referenced credentials provider."},
			{Name: "Name", Type: "string", Format: "name", Description: "Name is the namespace and name of the referenced credentials provider."},
			{Name: "Age", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
		},
	}
}

// ConvertToTable converts the output to a table.
func (c *convertor) ConvertToTable(_ context.Context, o runtime.Object, _ runtime.Object) (*metav1beta1.Table, error) {
	var (
		err   error
		table = &metav1beta1.Table{
			ColumnDefinitions: c.headers,
		}
	)

	if m, err := meta.ListAccessor(o); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.Continue = m.GetContinue()
	} else {
		if m, err := meta.CommonAccessor(o); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
		}
	}

	table.Rows, err = metatable.MetaToTableRow(o, func(o runtime.Object, _ metav1.Object, _, _ string) ([]any, error) {
		var (
			obj   = o.(*security.CredentialsBinding)
			cells = []any{}
		)

		cells = append(cells, obj.Name)
		cells = append(cells, obj.Provider.Type)

		cells = append(cells, obj.CredentialsRef.APIVersion)
		cells = append(cells, obj.CredentialsRef.Kind)
		cells = append(cells, obj.CredentialsRef.Namespace+"/"+obj.CredentialsRef.Name)

		cells = append(cells, metatable.ConvertToHumanReadableDateType(obj.CreationTimestamp))

		return cells, nil
	})

	return table, err
}
