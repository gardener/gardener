// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/test/utils/access"
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
	TargetSeedName          string
	SourceSeedName          string
	ShootName               string
	ShootNamespace          string
	AddTestRunTaint         string
	SkipNodeCheck           bool
	SkipMachinesCheck       bool
	SkipShootClientCreation bool
	SkipProtectedToleration bool
}

// ShootComparisonElements contains details about Machines and Nodes that will be compared during the tests
type ShootComparisonElements struct {
	MachineNames []string
	MachineNodes []string
	NodeNames    []string
	SecretsMap   map[string]corev1.Secret
}

// NewShootMigrationTest creates a new simple shoot migration test
func NewShootMigrationTest(ctx context.Context, f *GardenerFramework, cfg *ShootMigrationConfig) (*ShootMigrationTest, error) {
	t := &ShootMigrationTest{
		GardenerFramework: f,
		Config:            cfg,
	}
	return t, t.initializeShootMigrationTest(ctx)
}

func (t *ShootMigrationTest) initializeShootMigrationTest(ctx context.Context) error {
	if err := t.initShootAndClient(ctx); err != nil {
		return err
	}
	t.SeedShootNamespace = ComputeTechnicalID(t.GardenerFramework.ProjectNamespace, &t.Shoot)

	if err := t.initSeedsAndClients(ctx); err != nil {
		return err
	}

	return t.populateBeforeMigrationComparisonElements(ctx)
}

func (t *ShootMigrationTest) initShootAndClient(ctx context.Context) (err error) {
	shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: t.Config.ShootName, Namespace: t.Config.ShootNamespace}}
	if err = t.GardenerFramework.GetShoot(ctx, shoot); err != nil {
		return err
	}

	if !shoot.Status.IsHibernated && !t.Config.SkipShootClientCreation {
		t.ShootClient, err = access.CreateShootClientFromAdminKubeconfig(ctx, t.GardenerFramework.GardenClient, shoot)
		if err != nil {
			return err
		}
	}
	t.Shoot = *shoot
	return
}

func (t *ShootMigrationTest) initSeedsAndClients(ctx context.Context) error {
	t.Config.SourceSeedName = *t.Shoot.Spec.SeedName
	seed, seedClient, err := t.GardenerFramework.GetSeed(ctx, t.Config.TargetSeedName)
	if err != nil {
		return err
	}
	t.TargetSeedClient = seedClient
	t.TargetSeed = seed

	seed, seedClient, err = t.GardenerFramework.GetSeed(ctx, t.Config.SourceSeedName)
	if err != nil {
		return err
	}
	t.SourceSeedClient = seedClient
	t.SourceSeed = seed
	return nil
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
		if !t.Config.SkipProtectedToleration {
			shoot.Spec.Tolerations = appendToleration(shoot.Spec.Tolerations, gardencorev1beta1.SeedTaintProtected, nil)
		}
		if applyTestRunTaint, err := strconv.ParseBool(t.Config.AddTestRunTaint); applyTestRunTaint && err == nil {
			shoot.Spec.Tolerations = appendToleration(shoot.Spec.Tolerations, SeedTaintTestRun, ptr.To(GetTestRunID()))
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
		for i, t := range tolerations {
			if t.Key == key {
				tolerations[i].Value = value
				return tolerations
			}
		}
	}
	return append(tolerations, toleration)
}

// VerifyMigration checks that the shoot components are migrated properly
func (t ShootMigrationTest) VerifyMigration(ctx context.Context) error {
	if err := t.populateAfterMigrationComparisonElements(ctx); err != nil {
		return err
	}

	ginkgo.By("Compare all Machines, Nodes and persisted Secrets after the migration")
	if err := t.compareElementsAfterMigration(); err != nil {
		return err
	}

	ginkgo.By("Check for orphaned resources")
	return CheckForOrphanedNonNamespacedResources(ctx, t.SeedShootNamespace, t.SourceSeedClient.Client())
}

// GetNodeNames uses the shootClient to fetch all Node names from the Shoot
func (t *ShootMigrationTest) GetNodeNames(ctx context.Context, shootClient kubernetes.Interface) (nodeNames []string, err error) {
	if t.Shoot.Status.IsHibernated {
		return make([]string, 0), nil // Initialize to empty slice in order pass 0 elements DeepEqual check
	}

	nodeList := corev1.NodeList{}
	t.GardenerFramework.Logger.Info("Listing nodes")
	if err := shootClient.Client().List(ctx, &nodeList); err != nil {
		return nil, err
	}

	nodeNames = make([]string, len(nodeList.Items))
	for i, node := range nodeList.Items {
		t.GardenerFramework.Logger.Info("Found node", "index", i, "nodeName", node.Name)
		nodeNames[i] = node.Name
	}
	slices.Sort(nodeNames)
	return
}

// GetMachineDetails uses the seedClient to fetch all Machine names and the names of their corresponding Nodes
func (t *ShootMigrationTest) GetMachineDetails(ctx context.Context, seedClient kubernetes.Interface) (machineNames, machineNodes []string, err error) {
	log := t.GardenerFramework.Logger.WithValues("namespace", t.SeedShootNamespace)

	machineList := unstructured.UnstructuredList{}
	machineList.SetAPIVersion("machine.sapcloud.io/v1alpha1")
	machineList.SetKind("Machine")

	log.Info("Listing machines")
	if err := seedClient.Client().List(ctx, &machineList, client.InNamespace(t.SeedShootNamespace)); err != nil {
		return nil, nil, err
	}

	log.Info("Found machines", "count", len(machineList.Items))

	machineNames = make([]string, len(machineList.Items))
	machineNodes = make([]string, len(machineList.Items))
	for i, machine := range machineList.Items {
		log.Info("Found machine", "index", i, "machineName", machine.GetName(), "nodeName", machine.GetLabels()["node"])
		machineNames[i] = machine.GetName()
		machineNodes[i] = machine.GetLabels()["node"]
	}
	slices.Sort(machineNames)
	slices.Sort(machineNodes)
	return
}

// GetPersistedSecrets uses the seedClient to fetch the data of all Secrets that have the `persist` label key set to true
// from the Shoot's control plane namespace
func (t *ShootMigrationTest) GetPersistedSecrets(ctx context.Context, seedClient kubernetes.Interface) (map[string]corev1.Secret, error) {
	return GetPersistedSecrets(ctx, seedClient.Client(), t.SeedShootNamespace)
}

// GetPersistedSecrets uses the seedClient to fetch the data of all Secrets that have the `persist` label key set to true
// from the Shoot's control plane namespace
func GetPersistedSecrets(ctx context.Context, seedClient client.Reader, namespace string) (map[string]corev1.Secret, error) {
	secretList := &corev1.SecretList{}
	if err := seedClient.List(
		ctx,
		secretList,
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{secretsmanager.LabelKeyPersist: secretsmanager.LabelValueTrue}),
	); err != nil {
		return nil, err
	}

	secretsMap := make(map[string]corev1.Secret, len(secretList.Items))
	for _, secret := range secretList.Items {
		secretsMap[secret.Name] = secret
	}

	return secretsMap, nil
}

// ComparePersistedSecrets ensures that two secret maps are equal.
func ComparePersistedSecrets(secretsBefore, secretsAfter map[string]corev1.Secret) error {
	var errorMsg string
	for name, secret := range secretsBefore {
		if !reflect.DeepEqual(secret.Data, secretsAfter[name].Data) {
			errorMsg += fmt.Sprintf("Secret %s/%s did not have it's data persisted.\n", secret.Namespace, secret.Name)
		}
		if !maps.Equal(secret.Labels, secretsAfter[name].Labels) {
			errorMsg += fmt.Sprintf("Secret %s/%s did not have it's labels persisted: labels before migration: %v, labels after migration: %v\n",
				secret.Namespace,
				secret.Name,
				secret.Labels,
				secretsAfter[name].Labels,
			)
		}
	}
	if len(errorMsg) > 0 {
		return fmt.Errorf("control plane secrets did not have their data or labels persisted during control plane migration:\n %s", errorMsg)
	}
	return nil
}

// PopulateBeforeMigrationComparisonElements fills the ShootMigrationTest.ComparisonElementsBeforeMigration with the necessary Machine details and Node names
func (t *ShootMigrationTest) populateBeforeMigrationComparisonElements(ctx context.Context) (err error) {
	if !t.Config.SkipMachinesCheck {
		t.ComparisonElementsBeforeMigration.MachineNames, t.ComparisonElementsBeforeMigration.MachineNodes, err = t.GetMachineDetails(ctx, t.SourceSeedClient)
		if err != nil {
			return
		}
	}
	if !t.Config.SkipNodeCheck {
		t.ComparisonElementsBeforeMigration.NodeNames, err = t.GetNodeNames(ctx, t.ShootClient)
		if err != nil {
			return
		}
	}
	t.ComparisonElementsBeforeMigration.SecretsMap, err = t.GetPersistedSecrets(ctx, t.SourceSeedClient)
	return
}

// PopulateAfterMigrationComparisonElements fills the ShootMigrationTest.ComparisonElementsAfterMigration with the necessary Machine details and Node names
func (t *ShootMigrationTest) populateAfterMigrationComparisonElements(ctx context.Context) (err error) {
	if !t.Config.SkipMachinesCheck {
		t.ComparisonElementsAfterMigration.MachineNames, t.ComparisonElementsAfterMigration.MachineNodes, err = t.GetMachineDetails(ctx, t.TargetSeedClient)
		if err != nil {
			return
		}
	}
	if !t.Config.SkipNodeCheck {
		t.ComparisonElementsAfterMigration.NodeNames, err = t.GetNodeNames(ctx, t.ShootClient)
		if err != nil {
			return
		}
	}
	t.ComparisonElementsAfterMigration.SecretsMap, err = t.GetPersistedSecrets(ctx, t.TargetSeedClient)
	return
}

// CompareElementsAfterMigration compares the Machine details, Node names and Pod statuses before and after migration and returns error if there are differences.
func (t *ShootMigrationTest) compareElementsAfterMigration() error {
	if !t.Config.SkipMachinesCheck {
		if !reflect.DeepEqual(t.ComparisonElementsBeforeMigration.MachineNames, t.ComparisonElementsAfterMigration.MachineNames) {
			return fmt.Errorf("initial Machines %s, do not match after-migrate Machines %s", t.ComparisonElementsBeforeMigration.MachineNames, t.ComparisonElementsAfterMigration.MachineNames)
		}
		if !reflect.DeepEqual(t.ComparisonElementsBeforeMigration.MachineNodes, t.ComparisonElementsAfterMigration.MachineNodes) {
			return fmt.Errorf("initial Machine Nodes (label) %s, do not match after-migrate Machine Nodes (label) %s", t.ComparisonElementsBeforeMigration.MachineNodes, t.ComparisonElementsAfterMigration.MachineNodes)
		}
	}
	if t.Config.SkipNodeCheck {
		if !reflect.DeepEqual(t.ComparisonElementsBeforeMigration.NodeNames, t.ComparisonElementsAfterMigration.NodeNames) {
			return fmt.Errorf("initial Nodes %s, do not match after-migrate Nodes %s", t.ComparisonElementsBeforeMigration.NodeNames, t.ComparisonElementsAfterMigration.NodeNames)
		}
		if !reflect.DeepEqual(t.ComparisonElementsAfterMigration.MachineNodes, t.ComparisonElementsAfterMigration.NodeNames) {
			return fmt.Errorf("machine Nodes (label) %s, do not match after-migrate Nodes %s", t.ComparisonElementsAfterMigration.MachineNodes, t.ComparisonElementsAfterMigration.NodeNames)
		}
	}

	return ComparePersistedSecrets(t.ComparisonElementsBeforeMigration.SecretsMap, t.ComparisonElementsAfterMigration.SecretsMap)
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
			if !slices.Contains(mrExcludeList, mr.GetName()) {
				log := t.GardenerFramework.Logger.WithValues("managedResource", client.ObjectKeyFromObject(&mr))
				log.Info("Found ManagedResource")

				for _, r := range mr.Status.Resources {
					if len(r.Name) > 9 && slices.Contains(resourcesWithGeneratedName, r.Name[:len(r.Name)-9]) {
						continue
					}

					obj := &unstructured.Unstructured{}
					obj.SetAPIVersion(r.APIVersion)
					obj.SetKind(r.Kind)

					if err := t.ShootClient.Client().Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: r.Name}, obj); err != nil {
						return err
					}

					// Ignore immutable objects because if their data changes, they will be recreated
					if isImmutable, ok := obj.Object["immutable"]; ok && isImmutable == true {
						continue
					}

					creationTimestamp := obj.GetCreationTimestamp()
					objectLog := log.WithValues("objectKind", obj.GetKind(), "objectNamespace", obj.GetNamespace(), "objectName", obj.GetName(), "creationTimestamp", creationTimestamp)

					objectLog.Info("Found object")
					if t.MigrationTime.Before(&creationTimestamp) {
						objectLog.Info("Object is created after shoot migration", "migrationTime", t.MigrationTime)
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
func CheckForOrphanedNonNamespacedResources(ctx context.Context, shootNamespace string, sourceSeedClient client.Client) error {
	seedClientScheme := sourceSeedClient.Scheme()

	if err := extensionsv1alpha1.AddToScheme(seedClientScheme); err != nil {
		return err
	}

	var leakedObjects []string

	for _, obj := range []client.ObjectList{
		&extensionsv1alpha1.ClusterList{},
		&extensionsv1alpha1.BackupEntryList{},
		&rbacv1.ClusterRoleBindingList{},
		&rbacv1.ClusterRoleList{},
	} {
		if err := sourceSeedClient.List(ctx, obj, client.InNamespace(corev1.NamespaceAll)); err != nil {
			return err
		}

		if err := meta.EachListItem(obj, func(object runtime.Object) error {
			if strings.Contains(object.(client.Object).GetName(), shootNamespace) {
				leakedObjects = append(leakedObjects, fmt.Sprintf("%T %s", object, object.(client.Object).GetName()))
			}
			return nil
		}); err != nil {
			return err
		}
	}

	pvList := &corev1.PersistentVolumeList{}
	if err := sourceSeedClient.List(ctx, pvList, client.InNamespace(corev1.NamespaceAll)); err != nil {
		return err
	}
	if err := meta.EachListItem(pvList, func(obj runtime.Object) error {
		pv := obj.(*corev1.PersistentVolume)
		if strings.Contains(pv.Spec.ClaimRef.Namespace, shootNamespace) {
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

// MarkOSCSecret marks the operating system config pool hashes secret to verify that it is correctly migrated
func (t ShootMigrationTest) MarkOSCSecret(ctx context.Context) error {
	secret := &corev1.Secret{}
	if err := t.SourceSeedClient.Client().Get(ctx, types.NamespacedName{Namespace: t.SeedShootNamespace, Name: operatingsystemconfig.WorkerPoolHashesSecretName}, secret); err != nil {
		return err
	}
	metav1.SetMetaDataLabel(&secret.ObjectMeta, "gardener.cloud/custom-test-annotation", "test")
	return t.SourceSeedClient.Client().Update(ctx, secret)
}

// CreateSecretAndServiceAccount creates test secret and service account
func (t ShootMigrationTest) CreateSecretAndServiceAccount(ctx context.Context) error {
	testSecret, testServiceAccount := constructTestSecretAndServiceAccount()
	if err := t.ShootClient.Client().Create(ctx, testSecret); err != nil {
		return err
	}

	return t.ShootClient.Client().Create(ctx, testServiceAccount)
}

// CheckSecretAndServiceAccount checks the test secret and service account exists in the shoot.
func (t ShootMigrationTest) CheckSecretAndServiceAccount(ctx context.Context) error {
	testSecret, testServiceAccount := constructTestSecretAndServiceAccount()
	if err := t.ShootClient.Client().Get(ctx, client.ObjectKeyFromObject(testSecret), testSecret); err != nil {
		return err
	}

	return t.ShootClient.Client().Get(ctx, client.ObjectKeyFromObject(testServiceAccount), testServiceAccount)
}

// CleanUpSecretAndServiceAccount cleans up the test secret and service account
func (t ShootMigrationTest) CleanUpSecretAndServiceAccount(ctx context.Context) error {
	testSecret, testServiceAccount := constructTestSecretAndServiceAccount()

	return errors.Join(
		t.ShootClient.Client().Delete(ctx, testSecret),
		t.ShootClient.Client().Delete(ctx, testServiceAccount),
	)
}

func constructTestSecretAndServiceAccount() (*corev1.Secret, *corev1.ServiceAccount) {
	const (
		secretName              = "test-shoot-migration-secret"
		secretNamespace         = metav1.NamespaceDefault
		serviceAccountName      = "test-service-account"
		serviceAccountNamespace = metav1.NamespaceDefault
	)
	testSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
	}
	testServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: serviceAccountNamespace,
		}}
	return testSecret, testServiceAccount
}
