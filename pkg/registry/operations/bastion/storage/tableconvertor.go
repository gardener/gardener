// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/apis/operations"

	"k8s.io/apimachinery/pkg/api/meta"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apiserver/pkg/registry/rest"
)

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

type convertor struct {
	headers []metav1beta1.TableColumnDefinition
}

func newTableConvertor() rest.TableConvertor {
	return &convertor{
		headers: []metav1beta1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Shoot", Type: "string", Format: "name", Description: "The Shoot this bastion belongs to."},
			{Name: "Seed", Type: "string", Format: "name", Description: "The Seed cluster on which the Shoot is scheduled."},
			{Name: "IP", Type: "string", Format: "name", Description: "The bastion host's IP and/or hostname."},
			{Name: "Heartbeat", Type: "string", Format: "name", Description: "The time and date when the last heartbeat occurred."},
			{Name: "Expires", Type: "string", Format: "name", Description: "The time and date after which the bastion will be deleted."},
			{Name: "Age", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
		},
	}
}

// ConvertToTable converts the output to a table.
func (c *convertor) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
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
			bastion = obj.(*operations.Bastion)
			cells   = []interface{}{}
		)

		ingress := bastion.Status.Ingress

		cells = append(cells, bastion.Name)
		cells = append(cells, bastion.Spec.ShootRef.Name)

		if bastion.Spec.SeedName == nil {
			cells = append(cells, "<pending>")
		} else {
			cells = append(cells, *bastion.Spec.SeedName)
		}

		if ingress == nil || (ingress.IP == "" && ingress.Hostname == "") {
			cells = append(cells, "<pending>")
		} else if ingress.Hostname != "" {
			cells = append(cells, ingress.Hostname)
		} else {
			cells = append(cells, ingress.IP)
		}

		lastHeartbeat := "<never>"
		if !bastion.Status.LastHeartbeatTimestamp.IsZero() {
			lastHeartbeat = fmt.Sprintf("%s ago", duration.HumanDuration(time.Since(bastion.Status.LastHeartbeatTimestamp.Time)))
		}
		cells = append(cells, lastHeartbeat)

		expires := "<pending>"
		if !bastion.Status.ExpirationTimestamp.IsZero() {
			remaining := time.Until(bastion.Status.ExpirationTimestamp.Time)
			if remaining < 0 {
				expires = "<expired>"
			} else {
				expires = duration.HumanDuration(remaining)
			}
		}
		cells = append(cells, expires)

		cells = append(cells, metatable.ConvertToHumanReadableDateType(bastion.CreationTimestamp))

		return cells, nil
	})

	return table, err
}
