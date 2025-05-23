// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// SetDefaults_GardenletConfiguration sets defaults for the configuration of the Gardenlet.
func SetDefaults_GardenletConfiguration(obj *GardenletConfiguration) {
	if obj.GardenClientConnection == nil {
		obj.GardenClientConnection = &GardenClientConnection{}
	}

	if obj.SeedClientConnection == nil {
		obj.SeedClientConnection = &SeedClientConnection{}
	}

	if obj.ShootClientConnection == nil {
		obj.ShootClientConnection = &ShootClientConnection{}
	}

	if obj.Controllers == nil {
		obj.Controllers = &GardenletControllerConfiguration{}
	}

	if obj.LeaderElection == nil {
		obj.LeaderElection = &componentbaseconfigv1alpha1.LeaderElectionConfiguration{}
	}

	if obj.LogLevel == "" {
		obj.LogLevel = LogLevelInfo
	}

	if obj.LogFormat == "" {
		obj.LogFormat = LogFormatJSON
	}

	if obj.Logging == nil {
		obj.Logging = &Logging{}
	}

	// TODO: consider enabling profiling by default (like in k8s components)

	if obj.SNI == nil {
		obj.SNI = &SNI{}
	}

	if obj.Monitoring == nil {
		obj.Monitoring = &MonitoringConfig{}
	}

	if obj.ETCDConfig == nil {
		obj.ETCDConfig = &ETCDConfig{}
	}

	SetDefaults_ExposureClassHandler(obj.ExposureClassHandlers)
}

// SetDefaults_ServerConfiguration sets defaults for the configuration of the HTTP server.
func SetDefaults_ServerConfiguration(obj *ServerConfiguration) {
	if obj.HealthProbes == nil {
		obj.HealthProbes = &Server{}
	}
	if obj.HealthProbes.Port == 0 {
		obj.HealthProbes.Port = 2728
	}

	if obj.Metrics == nil {
		obj.Metrics = &Server{}
	}
	if obj.Metrics.Port == 0 {
		obj.Metrics.Port = 2729
	}
}

// SetDefaults_ExposureClassHandler sets defaults for the configuration for an exposure class handler.
func SetDefaults_ExposureClassHandler(obj []ExposureClassHandler) {
	var defaultSVCName = v1beta1constants.DefaultSNIIngressServiceName

	for i, handler := range obj {
		if obj[i].SNI == nil {
			obj[i].SNI = &SNI{Ingress: &SNIIngress{}}
		}
		if obj[i].SNI.Ingress == nil {
			obj[i].SNI.Ingress = &SNIIngress{}
		}
		if obj[i].SNI.Ingress.Namespace == nil {
			namespaceName := "istio-ingress-handler-" + handler.Name
			obj[i].SNI.Ingress.Namespace = &namespaceName
		}
		if obj[i].SNI.Ingress.ServiceName == nil {
			obj[i].SNI.Ingress.ServiceName = &defaultSVCName
		}
		if len(obj[i].SNI.Ingress.Labels) == 0 {
			obj[i].SNI.Ingress.Labels = map[string]string{
				v1beta1constants.LabelApp:   v1beta1constants.DefaultIngressGatewayAppLabelValue,
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleExposureClassHandler,
			}
		}
	}
}

// SetDefaults_GardenClientConnection sets defaults for the controller objects.
func SetDefaults_GardenClientConnection(obj *GardenClientConnection) {
	if obj.KubeconfigValidity == nil {
		obj.KubeconfigValidity = &KubeconfigValidity{}
	}
}

// SetDefaults_KubeconfigValidity sets defaults for the controller objects.
func SetDefaults_KubeconfigValidity(obj *KubeconfigValidity) {
	if obj.AutoRotationJitterPercentageMin == nil {
		obj.AutoRotationJitterPercentageMin = ptr.To[int32](70)
	}
	if obj.AutoRotationJitterPercentageMax == nil {
		obj.AutoRotationJitterPercentageMax = ptr.To[int32](90)
	}
}

// SetDefaults_GardenletControllerConfiguration sets defaults for the controller objects.
func SetDefaults_GardenletControllerConfiguration(obj *GardenletControllerConfiguration) {
	if obj.BackupBucket == nil {
		obj.BackupBucket = &BackupBucketControllerConfiguration{}
	}
	if obj.BackupEntry == nil {
		obj.BackupEntry = &BackupEntryControllerConfiguration{}
	}
	if obj.Bastion == nil {
		obj.Bastion = &BastionControllerConfiguration{}
	}
	if obj.ControllerInstallation == nil {
		obj.ControllerInstallation = &ControllerInstallationControllerConfiguration{}
	}
	if obj.ControllerInstallationCare == nil {
		obj.ControllerInstallationCare = &ControllerInstallationCareControllerConfiguration{}
	}
	if obj.ControllerInstallationRequired == nil {
		obj.ControllerInstallationRequired = &ControllerInstallationRequiredControllerConfiguration{}
	}
	if obj.Gardenlet == nil {
		obj.Gardenlet = &GardenletObjectControllerConfiguration{}
	}
	if obj.Seed == nil {
		obj.Seed = &SeedControllerConfiguration{}
	}
	if obj.Shoot == nil {
		obj.Shoot = &ShootControllerConfiguration{}
	}
	if obj.ShootCare == nil {
		obj.ShootCare = &ShootCareControllerConfiguration{}
	}
	if obj.SeedCare == nil {
		obj.SeedCare = &SeedCareControllerConfiguration{}
	}
	if obj.ShootState == nil {
		obj.ShootState = &ShootStateControllerConfiguration{}
	}
	if obj.NetworkPolicy == nil {
		obj.NetworkPolicy = &NetworkPolicyControllerConfiguration{}
	}
	if obj.ManagedSeed == nil {
		obj.ManagedSeed = &ManagedSeedControllerConfiguration{}
	}
	if obj.TokenRequestorServiceAccount == nil {
		obj.TokenRequestorServiceAccount = &TokenRequestorServiceAccountControllerConfiguration{}
	}
	if obj.TokenRequestorWorkloadIdentity == nil {
		obj.TokenRequestorWorkloadIdentity = &TokenRequestorWorkloadIdentityControllerConfiguration{}
	}
	if obj.VPAEvictionRequirements == nil {
		obj.VPAEvictionRequirements = &VPAEvictionRequirementsControllerConfiguration{}
	}
}

// SetDefaults_ClientConnectionConfiguration sets defaults for the client connection objects.
func SetDefaults_ClientConnectionConfiguration(obj *componentbaseconfigv1alpha1.ClientConnectionConfiguration) {
	if obj.QPS == 0.0 {
		obj.QPS = 50.0
	}
	if obj.Burst == 0 {
		obj.Burst = 100
	}
}

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the gardenlet.
func SetDefaults_LeaderElectionConfiguration(obj *componentbaseconfigv1alpha1.LeaderElectionConfiguration) {
	if obj.ResourceLock == "" {
		// Don't use a constant from the client-go resourcelock package here (resourcelock is not an API package, pulls
		// in some other dependencies and is thereby not suitable to be used in this API package).
		obj.ResourceLock = "leases"
	}

	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(obj)

	if obj.ResourceNamespace == "" {
		obj.ResourceNamespace = GardenletDefaultLockObjectNamespace
	}
	if obj.ResourceName == "" {
		obj.ResourceName = GardenletDefaultLockObjectName
	}
}

// SetDefaults_BackupBucketControllerConfiguration sets defaults for the backup bucket controller.
func SetDefaults_BackupBucketControllerConfiguration(obj *BackupBucketControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_BackupEntryControllerConfiguration sets defaults for the backup entry controller.
func SetDefaults_BackupEntryControllerConfiguration(obj *BackupEntryControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.DeletionGracePeriodHours == nil || *obj.DeletionGracePeriodHours < 0 {
		v := DefaultBackupEntryDeletionGracePeriodHours
		obj.DeletionGracePeriodHours = &v
	}
}

// SetDefaults_MonitoringConfig sets the defaults for the monitoring stack.
func SetDefaults_MonitoringConfig(obj *MonitoringConfig) {
	if obj.Shoot == nil {
		obj.Shoot = &ShootMonitoringConfig{}
	}
}

// SetDefaults_ShootMonitoringConfig sets the defaults for the shoot monitoring.
func SetDefaults_ShootMonitoringConfig(obj *ShootMonitoringConfig) {
	if obj.Enabled == nil {
		v := true
		obj.Enabled = &v
	}
}

// SetDefaults_BastionControllerConfiguration sets defaults for the bastion controller.
func SetDefaults_BastionControllerConfiguration(obj *BastionControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_ControllerInstallationControllerConfiguration sets defaults for the controller installation controller.
func SetDefaults_ControllerInstallationControllerConfiguration(obj *ControllerInstallationControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_ControllerInstallationCareControllerConfiguration sets defaults for the controller installation care controller.
func SetDefaults_ControllerInstallationCareControllerConfiguration(obj *ControllerInstallationCareControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: 30 * time.Second}
		obj.SyncPeriod = &v
	}
}

// SetDefaults_ControllerInstallationRequiredControllerConfiguration sets defaults for the ControllerInstallationRequired controller.
func SetDefaults_ControllerInstallationRequiredControllerConfiguration(obj *ControllerInstallationRequiredControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		// The controller actually starts one controller per extension resource per Seed.
		// For one seed that is already 1 * 10 extension resources = 10 workers.
		v := 1
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_GardenletObjectControllerConfiguration sets defaults for the gardenlet controller.
func SetDefaults_GardenletObjectControllerConfiguration(obj *GardenletObjectControllerConfiguration) {
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: 1 * time.Hour}
	}
}

// SetDefaults_SeedControllerConfiguration sets defaults for the seed controller.
func SetDefaults_SeedControllerConfiguration(obj *SeedControllerConfiguration) {
	if obj.SyncPeriod == nil {
		v := DefaultControllerSyncPeriod
		obj.SyncPeriod = &v
	}

	if obj.LeaseResyncSeconds == nil {
		obj.LeaseResyncSeconds = ptr.To[int32](2)
	}

	if obj.LeaseResyncMissThreshold == nil {
		obj.LeaseResyncMissThreshold = ptr.To[int32](10)
	}
}

// SetDefaults_SeedCareControllerConfiguration sets defaults for the seed care controller.
func SetDefaults_SeedCareControllerConfiguration(obj *SeedCareControllerConfiguration) {
	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: 30 * time.Second}
		obj.SyncPeriod = &v
	}
}

// SetDefaults_ShootControllerConfiguration sets defaults for the shoot controller.
func SetDefaults_ShootControllerConfiguration(obj *ShootControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: time.Hour}
		obj.SyncPeriod = &v
	}

	if obj.RespectSyncPeriodOverwrite == nil {
		falseVar := false
		obj.RespectSyncPeriodOverwrite = &falseVar
	}

	if obj.ReconcileInMaintenanceOnly == nil {
		falseVar := false
		obj.ReconcileInMaintenanceOnly = &falseVar
	}

	if obj.RetryDuration == nil {
		v := metav1.Duration{Duration: 12 * time.Hour}
		obj.RetryDuration = &v
	}

	if obj.DNSEntryTTLSeconds == nil {
		obj.DNSEntryTTLSeconds = ptr.To[int64](120)
	}
}

// SetDefaults_ShootCareControllerConfiguration sets defaults for the shoot care controller.
func SetDefaults_ShootCareControllerConfiguration(obj *ShootCareControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: time.Minute}
		obj.SyncPeriod = &v
	}

	if obj.StaleExtensionHealthChecks == nil {
		v := StaleExtensionHealthChecks{Enabled: true}
		obj.StaleExtensionHealthChecks = &v
	}
}

// SetDefaults_StaleExtensionHealthChecks sets defaults for the stale extension health checks.
func SetDefaults_StaleExtensionHealthChecks(obj *StaleExtensionHealthChecks) {
	if obj.Threshold == nil {
		v := metav1.Duration{Duration: 5 * time.Minute}
		obj.Threshold = &v
	}
}

// SetDefaults_ShootStateControllerConfiguration sets defaults for the shoot state controller.
func SetDefaults_ShootStateControllerConfiguration(obj *ShootStateControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: 6 * time.Hour}
	}
}

// SetDefaults_NetworkPolicyControllerConfiguration sets defaults for the network policy controller.
func SetDefaults_NetworkPolicyControllerConfiguration(obj *NetworkPolicyControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := 5
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_ManagedSeedControllerConfiguration sets defaults for the managed seed controller.
func SetDefaults_ManagedSeedControllerConfiguration(obj *ManagedSeedControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: 1 * time.Hour}
		obj.SyncPeriod = &v
	}

	if obj.WaitSyncPeriod == nil {
		v := metav1.Duration{Duration: 15 * time.Second}
		obj.WaitSyncPeriod = &v
	}

	if obj.SyncJitterPeriod == nil {
		v := metav1.Duration{Duration: 5 * time.Minute}
		obj.SyncJitterPeriod = &v
	}

	if obj.JitterUpdates == nil {
		obj.JitterUpdates = ptr.To(false)
	}
}

// SetDefaults_TokenRequestorServiceAccountControllerConfiguration sets defaults for the TokenRequestorServiceAccount controller.
func SetDefaults_TokenRequestorServiceAccountControllerConfiguration(obj *TokenRequestorServiceAccountControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
}

// SetDefaults_TokenRequestorWorkloadIdentityControllerConfiguration sets defaults for the TokenRequestorWorkloadIdentity controller.
func SetDefaults_TokenRequestorWorkloadIdentityControllerConfiguration(obj *TokenRequestorWorkloadIdentityControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
}

// SetDefaults_VPAEvictionRequirementsControllerConfiguration sets defaults for the VPAEvictionRequirements controller.
func SetDefaults_VPAEvictionRequirementsControllerConfiguration(obj *VPAEvictionRequirementsControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
}

// SetDefaults_SNI sets defaults for SNI.
func SetDefaults_SNI(obj *SNI) {
	if obj.Ingress == nil {
		obj.Ingress = &SNIIngress{}
	}
}

// SetDefaults_SNIIngress sets defaults for SNI ingressgateway.
func SetDefaults_SNIIngress(obj *SNIIngress) {
	var (
		defaultNS      = v1beta1constants.DefaultSNIIngressNamespace
		defaultSVCName = v1beta1constants.DefaultSNIIngressServiceName
	)

	if obj.Namespace == nil {
		obj.Namespace = &defaultNS
	}

	if obj.ServiceName == nil {
		obj.ServiceName = &defaultSVCName
	}

	if obj.Labels == nil {
		obj.Labels = map[string]string{
			v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
			"istio":                   "ingressgateway",
		}
	}
}

// SetDefaults_Logging sets defaults for the Logging stack.
func SetDefaults_Logging(obj *Logging) {
	if obj.Enabled == nil {
		obj.Enabled = ptr.To(false)
	}
	if obj.Vali == nil {
		obj.Vali = &Vali{}
	}
	if obj.Vali.Enabled == nil {
		obj.Vali.Enabled = obj.Enabled
	}
	if obj.Vali.Garden == nil {
		obj.Vali.Garden = &GardenVali{}
	}
	if obj.Vali.Garden.Storage == nil {
		obj.Vali.Garden.Storage = &DefaultCentralValiStorage
	}
	if obj.ShootEventLogging == nil {
		obj.ShootEventLogging = &ShootEventLogging{}
	}
	if obj.ShootEventLogging.Enabled == nil {
		obj.ShootEventLogging.Enabled = obj.Enabled
	}
}

// SetDefaults_ETCDConfig sets defaults for the ETCD.
func SetDefaults_ETCDConfig(obj *ETCDConfig) {
	if obj.ETCDController == nil {
		obj.ETCDController = &ETCDController{}
	}
	if obj.CustodianController == nil {
		obj.CustodianController = &CustodianController{}
	}
	if obj.BackupCompactionController == nil {
		obj.BackupCompactionController = &BackupCompactionController{}
	}
}

// SetDefaults_ETCDController sets defaults for the ETCD controller.
func SetDefaults_ETCDController(obj *ETCDController) {
	if obj.Workers == nil {
		obj.Workers = ptr.To[int64](50)
	}
}

// SetDefaults_CustodianController sets defaults for the ETCD custodian controller.
func SetDefaults_CustodianController(obj *CustodianController) {
	if obj.Workers == nil {
		obj.Workers = ptr.To[int64](10)
	}
}

// SetDefaults_BackupCompactionController sets defaults for the ETCD backup compaction controller.
func SetDefaults_BackupCompactionController(obj *BackupCompactionController) {
	if obj.Workers == nil {
		obj.Workers = ptr.To[int64](3)
	}
	if obj.EnableBackupCompaction == nil {
		obj.EnableBackupCompaction = ptr.To(false)
	}
	if obj.EventsThreshold == nil {
		obj.EventsThreshold = ptr.To[int64](1000000)
	}
	if obj.MetricsScrapeWaitDuration == nil {
		obj.MetricsScrapeWaitDuration = &metav1.Duration{Duration: 60 * time.Second}
	}
}
