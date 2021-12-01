// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ShootMigrationTest represents a shoot migration test.
// It can be used to test the migration of shoots between various seeds.
type ShootMigrationTest struct {
	GardenerFramework                 *GardenerFramework
	Config                            *ShootMigrationConfig
	TargetSeedClient                  kubernetes.Interface
	SourceSeedClient                  kubernetes.Interface
	ShootClient                       kubernetes.Interface
	TargetSeed                        *gardencorev1beta1.Seed
	SourceSeed                        *gardencorev1beta1.Seed
	ComparisonElementsBeforeMigration ShootComparisonElements
	ComparisonElementsAfterMigration  ShootComparisonElements
	Shoot                             gardencorev1beta1.Shoot
	SeedShootNamespace                string
	MigrationTime                     metav1.Time
}

// ShootMigrationConfig is the configuration for a shoot migration test that will be filled with user provided data
type ShootMigrationConfig struct {
	TargetSeedName  string
	SourceSeedName  string
	ShootName       string
	ShootNamespace  string
	AddTestRunTaint string
}

// ShootComparisonElements contains details about Machines and Nodes that will be compared during the tests
type ShootComparisonElements struct {
	MachineNames []string
	MachineNodes []string
	NodeNames    []string
}

// NewShootMigrationTest creates a new simple shoot migration test
func NewShootMigrationTest(f *GardenerFramework, cfg *ShootMigrationConfig) *ShootMigrationTest {
	t := &ShootMigrationTest{
		GardenerFramework: f,
		Config:            cfg,
	}
	return t
}

// MigrateShoot triggers shoot migration by changing the value of "shoot.Spec.SeedName" to the value of "ShootMigrationConfig.TargetSeedName"
func (t *ShootMigrationTest) MigrateShoot(ctx context.Context) error {
	// Dump gardener state if delete shoot is in exit handler
	if os.Getenv("TM_PHASE") == "Exit" {
		if shootFramework, err := t.GardenerFramework.NewShootFramework(ctx, &t.Shoot); err == nil {
			shootFramework.DumpState(ctx)
		} else {
			t.GardenerFramework.DumpState(ctx)
		}
	}

	t.MigrationTime = metav1.Now()
	return t.GardenerFramework.MigrateShoot(ctx, &t.Shoot, t.TargetSeed, func(shoot *gardencorev1beta1.Shoot) error {
		shoot.Spec.Tolerations = appendToleration(shoot.Spec.Tolerations, gardencorev1beta1.SeedTaintProtected, nil)
		if applyTestRunTaint, err := strconv.ParseBool(t.Config.AddTestRunTaint); applyTestRunTaint && err == nil {
			shoot.Spec.Tolerations = appendToleration(shoot.Spec.Tolerations, SeedTaintTestRun, pointer.String(GetTestRunID()))
		}
		return nil
	})
}

func appendToleration(tolerations []gardencorev1beta1.Toleration, key string, value *string) []gardencorev1beta1.Toleration {
	toleration := gardencorev1beta1.Toleration{
		Key:   key,
		Value: value,
	}
	if tolerations == nil {
		tolerations = make([]gardencorev1beta1.Toleration, 0)
	} else {
		for _, t := range tolerations {
			if t.Key == key {
				t.Value = value
				return tolerations
			}
		}
	}
	return append(tolerations, toleration)
}

// GetNodeNames uses the shootClient to fetch all Node names from the Shoot
func (t *ShootMigrationTest) GetNodeNames(ctx context.Context, shootClient kubernetes.Interface) (nodeNames []string, err error) {
	if t.Shoot.Status.IsHibernated {
		return make([]string, 0), nil // Initialize to empty slice in order pass 0 elements DeepEqual check
	}

	nodeList := corev1.NodeList{}
	t.GardenerFramework.Logger.Infof("Getting node names...")
	if err := shootClient.Client().List(ctx, &nodeList); err != nil {
		return nil, err
	}

	nodeNames = make([]string, len(nodeList.Items))
	for i, node := range nodeList.Items {
		t.GardenerFramework.Logger.Infof("%d. %s", i, node.Name)
		nodeNames[i] = node.Name
	}
	sort.Strings(nodeNames)
	return
}

// GetMachineDetails uses the seedClient to fetch all Machine names and the names of their corresponding Nodes
func (t *ShootMigrationTest) GetMachineDetails(ctx context.Context, seedClient kubernetes.Interface) (machineNames, machineNodes []string, err error) {
	machineList := unstructured.UnstructuredList{}
	machineList.SetAPIVersion("machine.sapcloud.io/v1alpha1")
	machineList.SetKind("Machine")
	t.GardenerFramework.Logger.Infof("Getting machine details in namespace: %s", t.SeedShootNamespace)
	if err := seedClient.Client().List(ctx, &machineList, client.InNamespace(t.SeedShootNamespace)); err != nil {
		t.GardenerFramework.Logger.Errorf("Error while getting Machine details, %s", err.Error())
		return nil, nil, err
	}

	t.GardenerFramework.Logger.Infof("Found: %d Machine items", len(machineList.Items))

	machineNames = make([]string, len(machineList.Items))
	machineNodes = make([]string, len(machineList.Items))
	for i, machine := range machineList.Items {
		t.GardenerFramework.Logger.Infof("%d. Machine Name: %s, Node Name: %s", i, machine.GetName(), machine.GetLabels()["node"])
		machineNames[i] = machine.GetName()
		machineNodes[i] = machine.GetLabels()["node"]
	}
	sort.Strings(machineNames)
	sort.Strings(machineNodes)
	return
}

// PopulateBeforeMigrationComparisonElements fills the ShootMigrationTest.ComparisonElementsBeforeMigration with the necessary Machine details and Node names
func (t *ShootMigrationTest) PopulateBeforeMigrationComparisonElements(ctx context.Context) (err error) {
	t.ComparisonElementsBeforeMigration.MachineNames, t.ComparisonElementsBeforeMigration.MachineNodes, err = t.GetMachineDetails(ctx, t.SourceSeedClient)
	if err != nil {
		return
	}
	t.ComparisonElementsBeforeMigration.NodeNames, err = t.GetNodeNames(ctx, t.ShootClient)

	return
}

// PopulateAfterMigrationComparisonElements fills the ShootMigrationTest.ComparisonElementsAfterMigration with the necessary Machine details and Node names
func (t *ShootMigrationTest) PopulateAfterMigrationComparisonElements(ctx context.Context) (err error) {
	t.ComparisonElementsAfterMigration.MachineNames, t.ComparisonElementsAfterMigration.MachineNodes, err = t.GetMachineDetails(ctx, t.TargetSeedClient)
	if err != nil {
		return
	}
	t.ComparisonElementsAfterMigration.NodeNames, err = t.GetNodeNames(ctx, t.ShootClient)

	return
}

// CompareElementsAfterMigration compares the Machine details, Node names and Pod statuses before and after migration and returns error if there are differences.
func (t *ShootMigrationTest) CompareElementsAfterMigration() error {
	if !reflect.DeepEqual(t.ComparisonElementsBeforeMigration.MachineNames, t.ComparisonElementsAfterMigration.MachineNames) {
		return fmt.Errorf("initial Machines %s, do not match after-migrate Machines %s", t.ComparisonElementsBeforeMigration.MachineNames, t.ComparisonElementsAfterMigration.MachineNames)
	}
	if !reflect.DeepEqual(t.ComparisonElementsBeforeMigration.MachineNodes, t.ComparisonElementsAfterMigration.MachineNodes) {
		return fmt.Errorf("initial Machine Nodes (label) %s, do not match after-migrate Machine Nodes (label) %s", t.ComparisonElementsBeforeMigration.MachineNodes, t.ComparisonElementsAfterMigration.MachineNodes)
	}
	if !reflect.DeepEqual(t.ComparisonElementsBeforeMigration.NodeNames, t.ComparisonElementsAfterMigration.NodeNames) {
		return fmt.Errorf("initial Nodes %s, do not match after-migrate Nodes %s", t.ComparisonElementsBeforeMigration.NodeNames, t.ComparisonElementsAfterMigration.NodeNames)
	}
	if !reflect.DeepEqual(t.ComparisonElementsAfterMigration.MachineNodes, t.ComparisonElementsAfterMigration.NodeNames) {
		return fmt.Errorf("machine Nodes (label) %s, do not match after-migrate Nodes %s", t.ComparisonElementsAfterMigration.MachineNodes, t.ComparisonElementsAfterMigration.NodeNames)

	}
	return nil
}

// CheckObjectsTimestamp checks the timestamp of all objects that the resource-manager creates in the Shoot cluster.
// The timestamp should not be after ShootMigrationTest.MigrationTime.
func (t *ShootMigrationTest) CheckObjectsTimestamp(ctx context.Context, mrExcludeList, resourcesWithGeneratedName []string) error {
	mrList := &resourcesv1alpha1.ManagedResourceList{}
	if err := t.TargetSeedClient.Client().List(
		ctx,
		mrList,
		client.InNamespace(t.SeedShootNamespace),
	); err != nil {
		return err
	}

	for _, mr := range mrList.Items {
		if mr.Spec.Class == nil || *mr.Spec.Class != "seed" {
			if !utils.ValueExists(mr.GetName(), mrExcludeList) {
				t.GardenerFramework.Logger.Infof("=== Managed Resource: %s/%s ===", mr.Namespace, mr.Name)
				for _, r := range mr.Status.Resources {
					if len(r.Name) > 9 && utils.ValueExists(r.Name[:len(r.Name)-9], resourcesWithGeneratedName) {
						continue
					}

					obj := &unstructured.Unstructured{}
					obj.SetAPIVersion(r.APIVersion)
					obj.SetKind(r.Kind)

					if err := t.ShootClient.Client().Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: r.Name}, obj); err != nil {
						return err
					}

					creationTimestamp := obj.GetCreationTimestamp()
					t.GardenerFramework.Logger.Infof("Object: %s %s/%s Created At: %s", obj.GetKind(), obj.GetNamespace(), obj.GetName(), creationTimestamp)
					if t.MigrationTime.Before(&creationTimestamp) {
						t.GardenerFramework.Logger.Errorf("object: %s %s/%s Created At: %s is created after the Shoot migration %s", obj.GetKind(), obj.GetNamespace(), obj.GetName(), creationTimestamp, t.MigrationTime)
						return fmt.Errorf("object: %s %s/%s Created At: %s is created after the Shoot migration %s", obj.GetKind(), obj.GetNamespace(), obj.GetName(), creationTimestamp, t.MigrationTime)
					}
				}
			}
		}
	}
	return nil
}

// CheckForOrphanedNonNamespacedResources checks if there are orphaned resources left on the target seed after the shoot migration.
// The function checks for Cluster, DNSOwner, BackupEntry, ClusterRoleBinding, ClusterRole and PersistentVolume
func (t *ShootMigrationTest) CheckForOrphanedNonNamespacedResources(ctx context.Context) error {
	seedClientScheme := t.SourceSeedClient.Client().Scheme()

	if err := extensionsv1alpha1.AddToScheme(seedClientScheme); err != nil {
		return err
	}

	leakedObjects := []string{}

	for _, obj := range []client.ObjectList{
		&extensionsv1alpha1.ClusterList{},
		&dnsv1alpha1.DNSOwnerList{},
		&v1alpha1.BackupEntryList{},
		&rbacv1.ClusterRoleBindingList{},
		&rbacv1.ClusterRoleList{},
	} {
		if err := t.SourceSeedClient.Client().List(ctx, obj, client.InNamespace(corev1.NamespaceAll)); err != nil {
			return err
		}

		if err := meta.EachListItem(obj, func(object runtime.Object) error {
			if strings.Contains(object.(client.Object).GetName(), t.SeedShootNamespace) {
				leakedObjects = append(leakedObjects, fmt.Sprintf("%s/%s", object.(client.Object).GetObjectKind(), object.(client.Object).GetName()))
			}
			return nil
		}); err != nil {
			return err
		}
	}

	pvList := &corev1.PersistentVolumeList{}
	if err := t.SourceSeedClient.Client().List(ctx, pvList, client.InNamespace(corev1.NamespaceAll)); err != nil {
		return err
	}
	if err := meta.EachListItem(pvList, func(obj runtime.Object) error {
		pv := obj.(*corev1.PersistentVolume)
		if strings.Contains(pv.Spec.ClaimRef.Namespace, t.SeedShootNamespace) {
			leakedObjects = append(leakedObjects, fmt.Sprintf("PersistentVolume/%s", pv.GetName()))
		}
		return nil
	}); err != nil {
		return err
	}
	if len(leakedObjects) > 0 {
		return fmt.Errorf("the following object(s) still exists in the source seed %v", leakedObjects)
	}
	return nil
}
