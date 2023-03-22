// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package indexer

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
)

// BastionShootNameIndexerFunc extracts the .spec.shootRef.name field of a Bastion.
func BastionShootNameIndexerFunc(obj client.Object) []string {
	bastion, ok := obj.(*operationsv1alpha1.Bastion)
	if !ok {
		return []string{""}
	}
	return []string{bastion.Spec.ShootRef.Name}
}

// AddBastionShootName adds an index for operations.BastionShootName to the given indexer.
func AddBastionShootName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &operationsv1alpha1.Bastion{}, operations.BastionShootName, BastionShootNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to Bastion Informer: %w", operations.BastionShootName, err)
	}
	return nil
}
