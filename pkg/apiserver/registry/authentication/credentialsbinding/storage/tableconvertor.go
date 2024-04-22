// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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

	"github.com/gardener/gardener/pkg/apis/authentication"
)

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

type convertor struct {
	headers []metav1beta1.TableColumnDefinition
}

func newTableConvertor() rest.TableConvertor {
	return &convertor{
		headers: []metav1beta1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Provider", Type: "string", Description: swaggerMetadataDescriptions["provider.type"]},
			{Name: "Secret", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["credentialsRef.secret"]},
			{Name: "WorkloadIdentity", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["credentialsRef.workloadIdentity"]},
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

	table.Rows, err = metatable.MetaToTableRow(o, func(o runtime.Object, _ metav1.Object, _, _ string) ([]interface{}, error) {
		var (
			obj   = o.(*authentication.CredentialsBinding)
			cells = []interface{}{}
		)

		cells = append(cells, obj.Name)
		cells = append(cells, obj.Provider.Type)
		if obj.CredentialsRef.Secret != nil {
			cells = append(cells, obj.CredentialsRef.Secret.Namespace+"/"+obj.CredentialsRef.Secret.Name)
		} else {
			cells = append(cells, "<none>")
		}

		if obj.CredentialsRef.WorkloadIdentity != nil {
			cells = append(cells, obj.CredentialsRef.WorkloadIdentity.Namespace+"/"+obj.CredentialsRef.WorkloadIdentity.Name)
		} else {
			cells = append(cells, "<none>")
		}

		return cells, nil
	})

	return table, err
}
