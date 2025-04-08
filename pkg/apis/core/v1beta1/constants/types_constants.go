// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

import (
	"time"
)

const (
	// SecretManagerIdentityControllerManager is the identity for the secret manager used inside controller-manager.
	SecretManagerIdentityControllerManager = "controller-manager"
	// SecretManagerIdentityGardenlet is the identity for the secret manager used inside gardenlet.
	SecretManagerIdentityGardenlet = "gardenlet"

	// SecretNameCACluster is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of a shoot cluster.
	SecretNameCACluster = "ca"
	// SecretNameCAClient is a constant for the name of a Kubernetes secret object that contains the client CA
	// certificate of a shoot cluster.
	SecretNameCAClient = "ca-client"
	// SecretNameCAETCD is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the etcd of a shoot cluster.
	SecretNameCAETCD = "ca-etcd"
	// SecretNameCAETCDPeer is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the etcd peer network of a shoot cluster.
	SecretNameCAETCDPeer = "ca-etcd-peer" // #nosec G101 -- No credential.
	// SecretNameCAFrontProxy is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the kube-aggregator a shoot cluster.
	SecretNameCAFrontProxy = "ca-front-proxy"
	// SecretNameCAKubelet is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the kubelet of a shoot cluster.
	SecretNameCAKubelet = "ca-kubelet"
	// SecretNameCAMetricsServer is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the metrics-server of a shoot cluster.
	SecretNameCAMetricsServer = "ca-metrics-server" // #nosec G101 -- No credential.
	// SecretNameCAVPN is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the VPN components of a shoot cluster.
	SecretNameCAVPN = "ca-vpn"
	// SecretNameCASeed is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate generated for a seed cluster.
	SecretNameCASeed = "ca-seed"

	// SecretNameCloudProvider is a constant for the name of a Kubernetes secret object that contains the provider
	// specific credentials that shall be used to create/delete the shoot.
	SecretNameCloudProvider = "cloudprovider"
	// SecretNameSSHKeyPair is a constant for the name of a Kubernetes secret object that contains the SSH key pair
	// (public and private key) that can be used to SSH into the shoot nodes.
	SecretNameSSHKeyPair = "ssh-keypair" // #nosec G101 -- No credential.
	// SecretNameServiceAccountKey is a constant for the name of a Kubernetes secret object that contains a
	// PEM-encoded private RSA or ECDSA key used by the Kube Controller Manager to sign service account tokens.
	SecretNameServiceAccountKey = "service-account-key"
	// SecretNameObservabilityIngress is a constant for the name of a Kubernetes secret object that contains the ingress
	// credentials for observability components.
	SecretNameObservabilityIngress = "observability-ingress" // #nosec G101 -- No credential.
	// SecretNameObservabilityIngressUsers is a constant for the name of a Kubernetes secret object that contains the
	// user's ingress credentials for observability components.
	SecretNameObservabilityIngressUsers = "observability-ingress-users" // #nosec G101 -- No credential.
	// SecretNameETCDEncryptionKey is a constant for the name of a Kubernetes secret object that contains the key
	// for encryption data in ETCD.
	SecretNameETCDEncryptionKey = "kube-apiserver-etcd-encryption-key" // #nosec G101 -- No credential.
	// SecretNamePrefixETCDEncryptionConfiguration is a constant for the name prefix of a Kubernetes secret object that
	// contains the configuration for encryption data in ETCD.
	SecretNamePrefixETCDEncryptionConfiguration = "kube-apiserver-etcd-encryption-configuration" // #nosec G101 -- No credential.
	// SecretNameGardenerETCDEncryptionKey is a constant for the name of a Kubernetes secret object that contains the
	// key for encryption data in ETCD for gardener-apiserver.
	SecretNameGardenerETCDEncryptionKey = "gardener-apiserver-etcd-encryption-key"
	// SecretNamePrefixGardenerETCDEncryptionConfiguration is a constant for the name prefix of a Kubernetes secret
	// object that contains the configuration for encryption data in ETCD for gardener-apiserver.
	SecretNamePrefixGardenerETCDEncryptionConfiguration = "gardener-apiserver-etcd-encryption-configuration"

	// SecretNameGardener is a constant for the name of a Kubernetes secret object that contains the client
	// certificate and a kubeconfig for a shoot cluster. It is used by Gardener and can be used by extension
	// controllers in order to communicate with the shoot's API server. The client certificate has administrator
	// privileges.
	SecretNameGardener = "gardener"
	// SecretNameGardenerInternal is a constant for the name of a Kubernetes secret object that contains the client
	// certificate and a kubeconfig for a shoot cluster. It is used by Gardener and can be used by extension
	// controllers in order to communicate with the shoot's API server. The client certificate has administrator
	// privileges. The difference to the "gardener" secret is that is contains the in-cluster endpoint as address to
	// for the shoot API server instead the DNS name or load balancer address.
	SecretNameGardenerInternal = "gardener-internal"

	// SecretPrefixGeneratedBackupBucket is a constant for the prefix of a secret name in the garden cluster related to
	// BackpuBuckets.
	SecretPrefixGeneratedBackupBucket = "generated-bucket-"

	// SecretNameGenericTokenKubeconfig is a constant for the name of the kubeconfig used by the shoot controlplane
	// components to authenticate against the shoot Kubernetes API server.
	// Use `pkg/extensions.GenericTokenKubeconfigSecretNameFromCluster` instead.
	SecretNameGenericTokenKubeconfig = "generic-token-kubeconfig"
	// SecretNameGenericGardenKubeconfig is a constant for the name of the kubeconfig used by the extension
	// components to authenticate against the garden Kubernetes API server.
	SecretNameGenericGardenKubeconfig = "generic-garden-kubeconfig"
	// AnnotationKeyGenericTokenKubeconfigSecretName is a constant for the key of an annotation on
	// extensions.gardener.cloud/v1alpha1.Cluster resources whose value contains the name of the generic token
	// kubeconfig secret in the seed cluster.
	AnnotationKeyGenericTokenKubeconfigSecretName = "generic-token-kubeconfig.secret.gardener.cloud/name"

	// ExtensionGardenServiceAccountPrefix is the prefix of the default garden ServiceAccount generated for each
	// ControllerInstallation.
	ExtensionGardenServiceAccountPrefix = "extension-"

	// ReferenceProtectionFinalizerName is the name of the finalizer used for the reference protection.
	ReferenceProtectionFinalizerName = "gardener.cloud/reference-protection"

	// DeploymentNameClusterAutoscaler is a constant for the name of a Kubernetes deployment object that contains
	// the cluster-autoscaler pod.
	DeploymentNameClusterAutoscaler = "cluster-autoscaler"
	// DeploymentNameKubeAPIServer is a constant for the name of a Kubernetes deployment object that contains
	// the kube-apiserver pod.
	DeploymentNameKubeAPIServer = "kube-apiserver"
	// DeploymentNameKubeControllerManager is a constant for the name of a Kubernetes deployment object that contains
	// the kube-controller-manager pod.
	DeploymentNameKubeControllerManager = "kube-controller-manager"
	// DeploymentNameDependencyWatchdogProber is a constant for the name of a Kubernetes deployment object that contains
	// the dependency-watchdog-prober pod.
	DeploymentNameDependencyWatchdogProber = "dependency-watchdog-prober"
	// DeploymentNameDependencyWatchdogWeeder is a constant for the name of a Kubernetes deployment object that contains
	// the dependency-watchdog-weeder pod.
	DeploymentNameDependencyWatchdogWeeder = "dependency-watchdog-weeder"
	// DeploymentNameGardenlet is a constant for the name of a Kubernetes deployment object that contains
	// the Gardenlet pod.
	DeploymentNameGardenlet = "gardenlet"
	// DeploymentNameGardenerOperator is a constant for the name of a Kubernetes deployment object that contains
	// the gardener-operator pod.
	DeploymentNameGardenerOperator = "gardener-operator"

	// DeploymentNameVPNSeedServer is a constant for the name of a Kubernetes deployment object that contains
	// the vpn-seed-server pod.
	DeploymentNameVPNSeedServer = "vpn-seed-server"

	// DeploymentNameKubeScheduler is a constant for the name of a Kubernetes deployment object that contains
	// the kube-scheduler pod.
	DeploymentNameKubeScheduler = "kube-scheduler"
	// DeploymentNameGardenerResourceManager is a constant for the name of a Kubernetes deployment object that contains
	// the gardener-resource-manager pod.
	DeploymentNameGardenerResourceManager = "gardener-resource-manager"
	// DeploymentNamePlutono is a constant for the name of a Kubernetes deployment object that contains the plutono pod.
	DeploymentNamePlutono = "plutono"
	// DeploymentNameEventLogger is a constant for the name of a Kubernetes deployment object that contains
	// the event-logger pod.
	DeploymentNameEventLogger = "event-logger"
	// DeploymentNameFluentOperator is a constant for the name of a Kubernetes deployment object that contains
	// the fluent-operator pod.
	DeploymentNameFluentOperator = "fluent-operator"
	// DaemonSetNameFluentBit is a constant for the name of a Kubernetes Daemonset object that contains
	// the fluent-bit pod.
	DaemonSetNameFluentBit = "fluent-bit"
	// DeploymentNameKubeStateMetrics is a constant for the name of a Kubernetes deployment object that contains
	// the kube-state-metrics pod.
	DeploymentNameKubeStateMetrics = "kube-state-metrics"
	// DeploymentNameGardenerMetricsExporter is a constant for the name of a Kubernetes deployment object that contains
	// the gardener-metrics-exporter pod.
	DeploymentNameGardenerMetricsExporter = "gardener-metrics-exporter"

	// DeploymentNameVPAAdmissionController is a constant for the name of the VPA admission controller deployment.
	DeploymentNameVPAAdmissionController = "vpa-admission-controller"
	// DeploymentNameVPARecommender is a constant for the name of the VPA recommender deployment.
	DeploymentNameVPARecommender = "vpa-recommender"
	// DeploymentNameVPAUpdater is a constant for the name of the VPA updater deployment.
	DeploymentNameVPAUpdater = "vpa-updater"

	// DeploymentNameKubernetesDashboard is a constant for the name of the kubernetes dashboard deployment.
	DeploymentNameKubernetesDashboard = "kubernetes-dashboard"
	// DeploymentNameDashboardMetricsScraper is a constant for the name of the dashboard metrics scraper deployment.
	DeploymentNameDashboardMetricsScraper = "dashboard-metrics-scraper"

	// DeploymentNameMachineControllerManager is a constant for the name of a Kubernetes deployment object that contains
	// the machine-controller-manager pod.
	DeploymentNameMachineControllerManager = "machine-controller-manager"

	// ConfigMapNameShootInfo is the name of a ConfigMap in the kube-system namespace of shoot clusters which contains
	// information about the shoot cluster.
	ConfigMapNameShootInfo = "shoot-info"

	// StatefulSetNameAlertManager is a constant for the name of a Kubernetes stateful set object that contains
	// the alertmanager pod.
	StatefulSetNameAlertManager = "alertmanager"
	// ETCDRoleMain is a constant for the main etcd role.
	ETCDRoleMain = "main"
	// ETCDRoleEvents is a constant for the events etcd role.
	ETCDRoleEvents = "events"
	// ETCDMain is a constant for the name of etcd-main Etcd object.
	ETCDMain = "etcd-" + ETCDRoleMain
	// ETCDEvents is a constant for the name of etcd-events Etcd object.
	ETCDEvents = "etcd-" + ETCDRoleEvents
	// StatefulSetNameVali is a constant for the name of a Kubernetes stateful set object that contains
	// the vali pod.
	StatefulSetNameVali = "vali"

	// GardenerPurpose is a constant for the key in a label describing the purpose of the respective object.
	GardenerPurpose = "gardener.cloud/purpose"
	// GardenerDescription is a constant for a key in an annotation describing what the resource is used for.
	GardenerDescription = "gardener.cloud/description"
	// GardenerWarning is a constant for a key in an annotation containing a warning message.
	GardenerWarning = "gardener.cloud/warning"

	// GardenCreatedBy is the key for an annotation of a Shoot cluster whose value indicates contains the username
	// of the user that created the resource.
	GardenCreatedBy = "gardener.cloud/created-by"
	// GardenerOperation is a constant for an annotation on a resource that describes a desired operation.
	GardenerOperation = "gardener.cloud/operation"
	// GardenerMaintenanceOperation is a constant for an annotation on a Shoot that describes a desired operation which
	// will be performed during maintenance.
	GardenerMaintenanceOperation = "maintenance.gardener.cloud/operation"
	// GardenerOperationReconcile is a constant for the value of the operation annotation describing a reconcile
	// operation.
	GardenerOperationReconcile = "reconcile"
	// GardenerTimestamp is a constant for an annotation on a resource that describes the timestamp when a reconciliation has been requested.
	// It is only used to guarantee an update event for watching clients in case the operation-annotation is already present.
	GardenerTimestamp = "gardener.cloud/timestamp"
	// GardenerOperationMigrate is a constant for the value of the operation annotation describing a migration
	// operation.
	GardenerOperationMigrate = "migrate"
	// GardenerOperationRestore is a constant for the value of the operation annotation describing a restoration
	// operation.
	GardenerOperationRestore = "restore"
	// GardenerOperationWaitForState is a constant for the value of the operation annotation describing a wait
	// operation.
	GardenerOperationWaitForState = "wait-for-state"
	// GardenerOperationKeepalive is a constant for the value of the operation annotation describing an
	// operation that extends the lifetime of the object having the operation annotation.
	GardenerOperationKeepalive = "keepalive"
	// GardenerOperationRenewKubeconfig is a constant for the value of the operation annotation to renew the gardenlet's
	// kubeconfig secret.
	GardenerOperationRenewKubeconfig = "renew-kubeconfig"

	// GardenRole is a constant for a label that describes a role.
	GardenRole = "gardener.cloud/role"
	// GardenRoleExtension is a constant for a label that describes the 'extensions' role.
	GardenRoleExtension = "extension"
	// GardenRoleGarden is the value of the GardenRole key indicating type 'garden'.
	GardenRoleGarden = "garden"
	// GardenRoleSeed is the value of the GardenRole key indicating type 'seed'.
	GardenRoleSeed = "seed"
	// GardenRoleShoot is the value of the GardenRole key indicating type 'shoot'.
	GardenRoleShoot = "shoot"
	// GardenRoleLogging is the value of the GardenRole key indicating type 'logging'.
	GardenRoleLogging = "logging"
	// GardenRoleIstioSystem is the value of the GardenRole key indicating type 'istio-system'.
	GardenRoleIstioSystem = "istio-system"
	// GardenRoleIstioIngress is the value of the GardenRole key indicating type 'istio-ingress'.
	GardenRoleIstioIngress = "istio-ingress"
	// GardenRoleProject is the value of GardenRole key indicating type 'project'.
	GardenRoleProject = "project"
	// GardenRoleControlPlane is the value of the GardenRole key indicating type 'controlplane'.
	GardenRoleControlPlane = "controlplane"
	// GardenRoleSystemComponent is the value of the GardenRole key indicating type 'system-component'.
	GardenRoleSystemComponent = "system-component"
	// GardenRoleSeedSystemComponent is the value of the GardenRole key indicating type 'seed-system-component'.
	GardenRoleSeedSystemComponent = "seed-system-component"
	// GardenRoleMonitoring is the value of the GardenRole key indicating type 'monitoring'.
	GardenRoleMonitoring = "monitoring"
	// GardenRoleOptionalAddon is the value of the GardenRole key indicating type 'optional-addon'.
	GardenRoleOptionalAddon = "optional-addon"
	// GardenRoleOperatingSystemConfig is the value of the GardenRole key indicating type 'operating-system-config'.
	GardenRoleOperatingSystemConfig = "operating-system-config"
	// GardenRoleKubeconfig is the value of the GardenRole key indicating type 'kubeconfig'.
	GardenRoleKubeconfig = "kubeconfig"
	// GardenRoleCACluster is the value of the GardenRole key indicating type 'ca-cluster'.
	GardenRoleCACluster = "ca-cluster"
	// GardenRoleCAKubelet is the value of the GardenRole key indicating type 'ca-kubelet'.
	GardenRoleCAKubelet = "ca-kubelet"
	// GardenRoleCAClient is the value of the GardenRole key indicating type 'ca-client'.
	GardenRoleCAClient = "ca-client"
	// GardenRoleSSHKeyPair is the value of the GardenRole key indicating type 'ssh-keypair'.
	GardenRoleSSHKeyPair = "ssh-keypair"
	// GardenRoleDefaultDomain is the value of the GardenRole key indicating type 'default-domain'.
	GardenRoleDefaultDomain = "default-domain"
	// GardenRoleInternalDomain is the value of the GardenRole key indicating type 'internal-domain'.
	GardenRoleInternalDomain = "internal-domain"
	// GardenRoleGlobalMonitoring is the value of the GardenRole key indicating type 'global-monitoring'
	GardenRoleGlobalMonitoring = "global-monitoring"
	// GardenRoleGlobalShootRemoteWriteMonitoring is the value of the GardenRole key indicating type 'global-shoot-remote-write-monitoring'
	GardenRoleGlobalShootRemoteWriteMonitoring = "global-shoot-remote-write-monitoring"
	// GardenRoleAlerting is the value of GardenRole key indicating type 'alerting'.
	GardenRoleAlerting = "alerting"
	// GardenRoleControlPlaneWildcardCert is the value of the GardenRole key indicating type 'controlplane-cert'.
	// It refers to a wildcard TLS certificate which can be used for seed services exposed under the corresponding domain.
	GardenRoleControlPlaneWildcardCert = "controlplane-cert"
	// GardenRoleGardenWildcardCert is the value of the GardenRole key indicating type 'garden-cert'.
	// It refers to a wildcard TLS certificate which can be used for Garden runtime services exposed under the corresponding domain.
	GardenRoleGardenWildcardCert = "garden-cert"
	// GardenRoleExposureClassHandler is the value of the GardenRole key indicating type 'exposureclass-handler'.
	GardenRoleExposureClassHandler = "exposureclass-handler"
	// GardenRoleShootServiceAccountIssuer is the value of the GardenRole key indicating type 'shoot-service-account-issuer'.
	GardenRoleShootServiceAccountIssuer = "shoot-service-account-issuer"
	// GardenRoleHelmPullSecret is the value of the GardenRole key indicating type 'helm-pull-secret'.
	GardenRoleHelmPullSecret = "helm-pull-secret"

	// ShootUID is an annotation key for the shoot namespace in the seed cluster,
	// which value will be the value of `shoot.status.uid`
	ShootUID = "shoot.gardener.cloud/uid"
	// ShootPurpose is a constant for the shoot purpose.
	ShootPurpose = "shoot.gardener.cloud/purpose"
	// ShootSyncPeriod is a constant for an annotation on a Shoot which may be used to overwrite the global Shoot controller sync period.
	// The value must be a duration. It can also be used to disable the reconciliation at all by setting it to 0m. Disabling the reconciliation
	// does only mean that the period reconciliation is disabled. However, when the Gardener is restarted/redeployed or the specification is
	// changed then the reconciliation flow will be executed.
	ShootSyncPeriod = "shoot.gardener.cloud/sync-period"
	// ShootIgnore is a constant for an annotation on a Shoot which may be used to tell the Gardener that the Shoot with this name should be
	// ignored completely. That means that the Shoot will never reach the reconciliation flow (independent of the operation (create/update/
	// delete)).
	ShootIgnore = "shoot.gardener.cloud/ignore"
	// ShootNoCleanup is a constant for a label on a resource indicating that the Gardener cleaner should not delete this
	// resource when cleaning a shoot during the deletion flow.
	ShootNoCleanup = "shoot.gardener.cloud/no-cleanup"
	// ShootDisableIstioTLSTermination is a constant for an annotation on a Shoot stating that the Istio TLS termination
	// for its kube-apiserver shall be disabled.
	ShootDisableIstioTLSTermination = "shoot.gardener.cloud/disable-istio-tls-termination"

	// ShootAlphaControlPlaneScaleDownDisabled is a constant for an annotation on the Shoot resource stating that the
	// automatic scale-down shall be disabled for the etcd, kube-apiserver, kube-controller-manager.
	// Note that this annotation is alpha and can be removed anytime without further notice. Only use it if you know
	// what you do.
	ShootAlphaControlPlaneScaleDownDisabled = "alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled"

	// ShootAlphaControlPlaneHAVPN is a constant for an annotation on the Shoot resource to enforce
	// enabling/disabling the high availability setup for the VPN connection.
	// By default, the HA setup for VPN connections is activated automatically if the control plane high availability is enabled.
	// Note that this annotation is alpha and can be removed anytime without further notice. Only use it if you know
	// what you do.
	ShootAlphaControlPlaneHAVPN = "alpha.control-plane.shoot.gardener.cloud/high-availability-vpn"
	// ShootAlphaControlPlaneVPNVPAUpdateDisabled is a constant for an annotation on the Shoot resource to enforce
	// disabling the vertical pod autoscaler update resources related to the VPN connection.
	ShootAlphaControlPlaneVPNVPAUpdateDisabled = "alpha.control-plane.shoot.gardener.cloud/vpn-vpa-update-disabled"
	// ShootExpirationTimestamp is an annotation on a Shoot resource whose value represents the time when the Shoot lifetime
	// is expired. The lifetime can be extended, but at most by the minimal value of the 'clusterLifetimeDays' property
	// of referenced quotas.
	ShootExpirationTimestamp = "shoot.gardener.cloud/expiration-timestamp"
	// ShootStatus is a constant for a label on a Shoot resource indicating that the Shoot's health.
	ShootStatus = "shoot.gardener.cloud/status"
	// FailedShootNeedsRetryOperation is a constant for an annotation on a Shoot in a failed state indicating that a retry operation should be triggered during the next maintenance time window.
	FailedShootNeedsRetryOperation = "maintenance.shoot.gardener.cloud/needs-retry-operation"
	// LabelExcludeWebhookFromRemediation is a constant for a label on a webhook in the shoot which makes it being
	// excluded from automatic remediation.
	LabelExcludeWebhookFromRemediation = "remediation.webhook.shoot.gardener.cloud/exclude"

	// ShootTasks is a constant for an annotation on a Shoot which states that certain tasks should be done.
	ShootTasks = "shoot.gardener.cloud/tasks"
	// ShootTaskDeployInfrastructure is a name for a Shoot's infrastructure deployment task. It indicates that the
	// Infrastructure extension resource shall be reconciled.
	ShootTaskDeployInfrastructure = "deployInfrastructure"
	// ShootTaskDeployDNSRecordInternal is a name for a Shoot's internal DNS record deployment task. It indicates that
	// the internal DNSRecord extension resources shall be reconciled.
	ShootTaskDeployDNSRecordInternal = "deployDNSRecordInternal"
	// ShootTaskDeployDNSRecordExternal is a name for a Shoot's external DNS record deployment task. It indicates that
	// the external DNSRecord extension resources shall be reconciled.
	ShootTaskDeployDNSRecordExternal = "deployDNSRecordExternal"
	// ShootTaskDeployDNSRecordIngress is a name for a Shoot's ingress DNS record deployment task. It indicates that
	// the ingress DNSRecord extension resources shall be reconciled.
	ShootTaskDeployDNSRecordIngress = "deployDNSRecordIngress"
	// ShootTaskRestartControlPlanePods is a name for a Shoot task which is dedicated to restart related control plane pods.
	ShootTaskRestartControlPlanePods = "restartControlPlanePods"
	// ShootTaskRestartCoreAddons is a name for a Shoot task which is dedicated to restart some core addons.
	ShootTaskRestartCoreAddons = "restartCoreAddons"
	// ShootOperationMaintain is a constant for an annotation on a Shoot indicating that the Shoot maintenance shall be
	// executed as soon as possible.
	ShootOperationMaintain = "maintain"
	// ShootOperationRetry is a constant for an annotation on a Shoot indicating that a failed Shoot reconciliation shall be
	// retried.
	ShootOperationRetry = "retry"
	// ShootOperationForceInPlaceUpdate is a constant for the value of the operation annotation that must be set
	// to forcibly trigger an in-place update when a previous update is still in progress.
	ShootOperationForceInPlaceUpdate = "force-in-place-update"
	// OperationRotateCredentialsStart is a constant for an annotation indicating that the rotation of all credentials
	// shall be started. This includes CAs, certificates, kubeconfigs, SSH keypairs, observability credentials, and
	// ServiceAccount signing key.
	OperationRotateCredentialsStart = "rotate-credentials-start" // #nosec G101 -- No credential.
	// OperationRotateCredentialsStartWithoutWorkersRollout is a constant for an annotation indicating that the rotation
	// of all credentials shall be started without rolling out the workers. This includes CAs, certificates,
	// kubeconfigs, SSH keypairs, observability credentials, and ServiceAccount signing key.
	OperationRotateCredentialsStartWithoutWorkersRollout = "rotate-credentials-start-without-workers-rollout" // #nosec G101 -- No credential.
	// OperationRotateCredentialsComplete is a constant for an annotation indicating that the rotation of the
	// credentials shall be completed.
	OperationRotateCredentialsComplete = "rotate-credentials-complete" // #nosec G101 -- No credential.
	// ShootOperationRotateSSHKeypair is a constant for an annotation on a Shoot indicating that the SSH keypair for the
	// shoot nodes shall be rotated.
	ShootOperationRotateSSHKeypair = "rotate-ssh-keypair"
	// OperationRotateCAStart is a constant for an annotation indicating that the rotation of the certificate
	// authorities shall be started.
	OperationRotateCAStart = "rotate-ca-start"
	// OperationRotateCAStartWithoutWorkersRollout is a constant for an annotation indicating that the rotation of the
	// certificate authorities shall be started without rolling out the workers.
	OperationRotateCAStartWithoutWorkersRollout = "rotate-ca-start-without-workers-rollout"
	// OperationRotateCAComplete is a constant for an annotation indicating that the rotation of the certificate
	// authorities shall be completed.
	OperationRotateCAComplete = "rotate-ca-complete"
	// OperationRotateObservabilityCredentials is a constant for an annotation indicating that the
	// credentials for the observability stack secret shall be rotated. Note that this only affects the user credentials
	// since the operator credentials are rotated automatically each `30d`.
	OperationRotateObservabilityCredentials = "rotate-observability-credentials" // #nosec G101 -- No credential.
	// OperationRotateServiceAccountKeyStart is a constant for an annotation on a Shoot indicating that the
	// rotation of the service account signing key shall be started.
	OperationRotateServiceAccountKeyStart = "rotate-serviceaccount-key-start"
	// OperationRotateServiceAccountKeyStartWithoutWorkersRollout is a constant for an annotation on a Shoot indicating that the
	// rotation of the service account signing key shall be started without rolling out the workers.
	OperationRotateServiceAccountKeyStartWithoutWorkersRollout = "rotate-serviceaccount-key-start-without-workers-rollout"
	// OperationRotateServiceAccountKeyComplete is a constant for an annotation on a Shoot indicating that the
	// rotation of the service account signing key shall be completed.
	OperationRotateServiceAccountKeyComplete = "rotate-serviceaccount-key-complete"
	// OperationRotateETCDEncryptionKeyStart is a constant for an annotation on a Shoot indicating that the
	// rotation of the ETCD encryption key shall be started.
	OperationRotateETCDEncryptionKeyStart = "rotate-etcd-encryption-key-start"
	// OperationRotateETCDEncryptionKeyComplete is a constant for an annotation on a Shoot indicating that the
	// rotation of the ETCD encryption key shall be completed.
	OperationRotateETCDEncryptionKeyComplete = "rotate-etcd-encryption-key-complete"
	// OperationRotateRolloutWorkers is a constant for an annotation triggering the rollout of one or more worker pools
	// (comma-separated) when the certificate authorities or service account signing key credentials rotation is in
	// WaitingForWorkersRollout phase.
	OperationRotateRolloutWorkers = "rotate-rollout-workers"
	// SeedOperationRenewGardenAccessSecrets is a constant for an annotation on a Seed indicating that
	// all garden access secrets on the seed shall be renewed.
	SeedOperationRenewGardenAccessSecrets = "renew-garden-access-secrets" // #nosec G101 -- No credential.
	// SeedOperationRenewWorkloadIdentityTokens is a constant for an annotation on a Seed indicating that
	// all workload identity tokens on the seed shall be renewed.
	SeedOperationRenewWorkloadIdentityTokens = "renew-workload-identity-tokens"
	// KubeconfigSecretOperationRenew is a constant for an annotation on the secret in a Seed containing the garden
	// cluster kubeconfig of a gardenlet indicating that it should be renewed.
	KubeconfigSecretOperationRenew = "renew"

	// ConfirmationDeletion is an annotation on a Shoot, Project, and ShootState resources whose value must be set to
	// "true" in order to allow deleting the resource (if the annotation is not set any DELETE request will be denied).
	ConfirmationDeletion = "confirmation.gardener.cloud/deletion"
	// DeletionConfirmedBy is an annotation on a resource whose value is the subject which confirmed the deletion.
	DeletionConfirmedBy = "deletion.gardener.cloud/confirmed-by"

	// SeedResourceManagerClass is the resource-class managed by the Gardener-Resource-Manager
	// instance in the garden namespace on the seeds.
	SeedResourceManagerClass = "seed"
	// LabelBackupProvider is used to identify the backup provider.
	LabelBackupProvider = "backup.gardener.cloud/provider"
	// LabelSeedProvider is used to identify the seed provider.
	LabelSeedProvider = "seed.gardener.cloud/provider"
	// LabelShootProvider is used to identify the shoot provider.
	LabelShootProvider = "shoot.gardener.cloud/provider"
	// LabelShootProviderPrefix is used to prefix label that indicates the provider type.
	// The label key is in the form provider.shoot.gardener.cloud/<type>.
	LabelShootProviderPrefix = "provider.shoot.gardener.cloud/"
	// LabelNetworkingProvider is used to identify the networking provider for the cni plugin.
	LabelNetworkingProvider = "networking.shoot.gardener.cloud/provider"
	// LabelExtensionPrefix is used to prefix extension specific labels.
	LabelExtensionPrefix = "extensions.gardener.cloud/"
	// LabelLogging is a constant for a label for logging stack configurations
	LabelLogging = "logging"
	// LabelMonitoring is a constant for a label for monitoring stack configurations
	LabelMonitoring = "monitoring"
	// LabelPrefixMonitoringDashboard is the prefix of a label key on ConfigMaps for indicating that the data contains a
	// dashboard.
	LabelPrefixMonitoringDashboard = "dashboard.monitoring.gardener.cloud/"
	// LabelKeyCustomLoggingResource is the key of the label which is used from the operator to select the CustomResources which will be imported in the FluentBit configuration.
	// TODO(nickytd): the label key has to be migrated to "fluentbit.gardener.cloud/type".
	LabelKeyCustomLoggingResource = "fluentbit.gardener/type"
	// LabelValueCustomLoggingResource is the value of the label which is used from the operator to select the CustomResources which will be imported in the FluentBit configuration.
	LabelValueCustomLoggingResource = "seed"
	// LabelSeedNetwork is used to specify whether the seed is reachable from the garden cluster.
	LabelSeedNetwork = "seed.gardener.cloud/network"
	// LabelSeedNetworkPrivate is used to specify that the seed is in private networks and not reachable from the garden
	// cluster.
	LabelSeedNetworkPrivate = "private"
	// LabelKeyAggregateToProjectMember is a constant for a label on ClusterRoles that are aggregated to the project
	// member ClusterRole.
	LabelKeyAggregateToProjectMember = "rbac.gardener.cloud/aggregate-to-project-member"
	// LabelAutonomousShootCluster is a constant for a label on a Seed indicating that it is an autonomous shoot cluster.
	LabelAutonomousShootCluster = "seed.gardener.cloud/autonomous-shoot-cluster"
	// LabelSecretBindingReference is used to identify secrets which are referred by a SecretBinding (not necessarily in the same namespace).
	LabelSecretBindingReference = "reference.gardener.cloud/secretbinding"
	// LabelCredentialsBindingReference is used to identify credentials which are referred by a CredentialsBinding (not necessarily in the same namespace).
	LabelCredentialsBindingReference = "reference.gardener.cloud/credentialsbinding"
	// LabelPrefixSeedName is the prefix for the label key describing the name of a seed, e.g. name.seed.gardener.cloud/my-seed=true.
	LabelPrefixSeedName = "name.seed.gardener.cloud/"

	// LabelExtensionExtensionTypePrefix is used to prefix extension label for extension types.
	LabelExtensionExtensionTypePrefix = "extensions.extensions.gardener.cloud/"
	// LabelExtensionProviderTypePrefix is used to prefix extension label for cloud provider types.
	LabelExtensionProviderTypePrefix = "provider.extensions.gardener.cloud/"
	// LabelExtensionDNSRecordTypePrefix is used to prefix extension label for DNS types.
	LabelExtensionDNSRecordTypePrefix = "dnsrecord.extensions.gardener.cloud/"
	// LabelExtensionNetworkingTypePrefix is used to prefix extension label for networking plugin types.
	LabelExtensionNetworkingTypePrefix = "networking.extensions.gardener.cloud/"
	// LabelExtensionOperatingSystemConfigTypePrefix is used to prefix extension label for OperatingSystemConfig types.
	LabelExtensionOperatingSystemConfigTypePrefix = "operatingsystemconfig.extensions.gardener.cloud/"
	// LabelExtensionContainerRuntimeTypePrefix is used to prefix extension label for ContainerRuntime types.
	LabelExtensionContainerRuntimeTypePrefix = "containerruntime.extensions.gardener.cloud/"

	// LabelExtensionProviderMutatedByControlplaneWebhook is used to specify extension provider controlplane webhook targets
	LabelExtensionProviderMutatedByControlplaneWebhook = LabelExtensionProviderTypePrefix + "mutated-by-controlplane-webhook"

	// LabelNetworkPolicyToBlockedCIDRs allows Egress from pods labeled with 'networking.gardener.cloud/to-blocked-cidrs=allowed'.
	LabelNetworkPolicyToBlockedCIDRs = "networking.gardener.cloud/to-blocked-cidrs"
	// LabelNetworkPolicyToDNS allows Egress from pods labeled with 'networking.gardener.cloud/to-dns=allowed' to DNS running in 'kube-system'.
	// In practice, most of the Pods which require network Egress need this label.
	LabelNetworkPolicyToDNS = "networking.gardener.cloud/to-dns"
	// LabelNetworkPolicyToPrivateNetworks allows Egress from pods labeled with 'networking.gardener.cloud/to-private-networks=allowed' to the
	// private networks (RFC1918), Carrier-grade NAT (RFC6598) except for cloudProvider's specific metadata service IP, seed networks,
	// shoot networks.
	LabelNetworkPolicyToPrivateNetworks = "networking.gardener.cloud/to-private-networks"
	// LabelNetworkPolicyToPublicNetworks allows Egress from pods labeled with 'networking.gardener.cloud/to-public-networks=allowed' to all public
	// network IPs, except for private networks (RFC1918), carrier-grade NAT (RFC6598), cloudProvider's specific metadata service IP.
	// In practice, this blocks Egress traffic to all networks in the Seed cluster and only traffic to public IPv4 addresses.
	LabelNetworkPolicyToPublicNetworks = "networking.gardener.cloud/to-public-networks"
	// LabelNetworkPolicyToSeedAPIServer allows Egress from pods labeled with 'networking.gardener.cloud/to-seed-apiserver=allowed' to Seed's Kubernetes
	// API Server.
	//
	// Deprecated: Use LabelNetworkPolicyToRuntimeAPIServer instead.
	LabelNetworkPolicyToSeedAPIServer = "networking.gardener.cloud/to-seed-apiserver"
	// LabelNetworkPolicyToRuntimeAPIServer allows Egress from pods labeled with 'networking.gardener.cloud/to-runtime-apiserver=allowed' to runtime Kubernetes
	// API Server.
	LabelNetworkPolicyToRuntimeAPIServer = "networking.gardener.cloud/to-runtime-apiserver"
	// LabelNetworkPolicyFromPrometheus allows Ingress from Prometheus to pods labeled with 'networking.gardener.cloud/from-prometheus=allowed' and ports
	// named 'metrics' in the PodSpecification.
	//
	// Deprecated: This label is deprecated and will be removed in a future version. Components in shoot namespaces
	//  which need to be scraped by Prometheus need to annotate their Services with
	//  `networking.resources.gardener.cloud/from-policy-pod-label-selector=all-scrape-targets` and
	//  `networking.resources.gardener.cloud/from-policy-allowed-ports=[{"protocol":<protocol>,"port":<port>}]`.
	LabelNetworkPolicyFromPrometheus = "networking.gardener.cloud/from-prometheus"
	// LabelNetworkPolicyShootFromSeed allows Ingress traffic from the seed cluster (where the shoot's kube-apiserver
	// runs).
	LabelNetworkPolicyShootFromSeed = "networking.gardener.cloud/from-seed"
	// LabelNetworkPolicyShootToAPIServer allows Egress traffic to the shoot's API server.
	LabelNetworkPolicyShootToAPIServer = "networking.gardener.cloud/to-apiserver"
	// LabelNetworkPolicyShootToKubelet allows Egress traffic to the kubelets.
	LabelNetworkPolicyShootToKubelet = "networking.gardener.cloud/to-kubelet"
	// LabelNetworkPolicyAllowed is a constant for allowing a network policy.
	LabelNetworkPolicyAllowed = "allowed"
	// LabelNetworkPolicyScrapeTargets is a constant for pod selector label which can be used on Services for components
	// which should be scraped by Prometheus.
	// See https://github.com/gardener/gardener/blob/master/docs/concepts/resource-manager.md#overwriting-the-pod-selector-label.
	LabelNetworkPolicyScrapeTargets = "all-scrape-targets"
	// LabelNetworkPolicyGardenScrapeTargets is a constant for pod selector label which can be used on Services for
	// garden system components or extensions which should be scraped by Prometheus.
	// See https://github.com/gardener/gardener/blob/master/docs/concepts/resource-manager.md#overwriting-the-pod-selector-label.
	LabelNetworkPolicyGardenScrapeTargets = "all-garden-scrape-targets"
	// LabelNetworkPolicySeedScrapeTargets is a constant for pod selector label which can be used on Services for
	// seed system components or extensions which should be scraped by Prometheus.
	// See https://github.com/gardener/gardener/blob/master/docs/concepts/resource-manager.md#overwriting-the-pod-selector-label.
	LabelNetworkPolicySeedScrapeTargets = "all-seed-scrape-targets"
	// LabelNetworkPolicyWebhookTargets is a constant for pod selector label which can be used on Services for
	// garden or shoot components which serve a webhook endpoint that must be reachable by the kube-apiserver.
	// See https://github.com/gardener/gardener/blob/master/docs/concepts/resource-manager.md#overwriting-the-pod-selector-label.
	LabelNetworkPolicyWebhookTargets = "all-webhook-targets"
	// LabelNetworkPolicyShootNamespaceAlias is a constant for the alias for shoot namespaces used in NetworkPolicy
	// labels.
	LabelNetworkPolicyShootNamespaceAlias = "all-shoots"
	// LabelNetworkPolicyExtensionsNamespaceAlias is a constant for the alias for extension namespaces used in
	// NetworkPolicy labels.
	LabelNetworkPolicyExtensionsNamespaceAlias = "extensions"
	// LabelNetworkPolicyIstioIngressNamespaceAlias is a constant for the alias for shoot namespaces used in
	// NetworkPolicy labels.
	LabelNetworkPolicyIstioIngressNamespaceAlias = "all-istio-ingresses"
	// LabelNetworkPolicyAccessTargetAPIServer is a constant for the alias for a namespace which runs components that
	// need to initiate the communication with a target API server (e.g., shoot API server or virtual garden API
	// server).
	LabelNetworkPolicyAccessTargetAPIServer = "networking.gardener.cloud/access-target-apiserver"

	// LabelAuthorizationExtensionsServiceAccountSelector is a constant for an annotation key on ClusterRoles in the
	// garden cluster which can be used to describe a selector for labels on ServiceAccounts which are allowed to get
	// bound to this ClusterRole.
	LabelAuthorizationExtensionsServiceAccountSelector = "authorization.gardener.cloud/extensions-serviceaccount-selector"
	// LabelAuthorizationCustomExtensionsPermissions is a constant for a label key on ClusterRoles in the garden
	// cluster which can be used to describe that this ClusterRole contains custom permissions for extensions.
	LabelAuthorizationCustomExtensionsPermissions = "authorization.gardener.cloud/custom-extensions-permissions"

	// LabelObservabilityApplication is a constant for a label key set to all observability applications in gardener exposing a public endpoint.
	LabelObservabilityApplication = "observability.gardener.cloud/app"

	// LabelApp is a constant for a label key.
	LabelApp = "app"
	// LabelRole is a constant for a label key.
	LabelRole = "role"
	// LabelKubernetes is a constant for a label for Kubernetes workload.
	LabelKubernetes = "kubernetes"
	// LabelGardener is a constant for a label for Gardener workload.
	LabelGardener = "gardener"
	// LabelAPIServer is a constant for a label for the kube-apiserver.
	LabelAPIServer = "apiserver"
	// LabelControllerManager is a constant for a label for the kube-controller-manager.
	LabelControllerManager = "controller-manager"
	// LabelScheduler is a constant for a label for the kube-scheduler.
	LabelScheduler = "scheduler"
	// LabelProxy is a constant for a label for the kube-proxy.
	LabelProxy = "proxy"
	// LabelExtensionProjectRole is a constant for a label value for extension project roles
	LabelExtensionProjectRole = "extension-project-role"

	// LabelShootNamespace is a constant for a label key that indicates a relationship to a shoot in the specified namespace.
	LabelShootNamespace = "shoot.gardener.cloud/namespace"
	// LabelShootName is a constant for a label key that indicates a relationship to a shoot with the specified name.
	LabelShootName = "shoot.gardener.cloud/name"
	// LabelShootUID is a constant for a label key that indicates a relationship to a shoot with the specified UID.
	LabelShootUID = "shoot.gardener.cloud/uid"

	// LabelPublicKeys is a constant for a label key that indicates that a resource contains public keys.
	// Deprecated: Use LabelDiscoveryPublic instead.
	LabelPublicKeys = "authentication.gardener.cloud/public-keys" // TODO(dimityrmirchev): Deprecate in favour of LabelDiscoveryPublic
	// LabelPublicKeysServiceAccount is a constant for a label value that indicates that a resource contains service account public keys.
	LabelPublicKeysServiceAccount = "serviceaccount"

	// LabelDiscoveryPublic is a constant for a label key that indicates that the labeled resource is of interest to the Gardener Discovery Server.
	LabelDiscoveryPublic = "discovery.gardener.cloud/public"
	// DiscoveryShootCA is a constant for a label value that indicates that the labeled resource contains shoot cluster certificate authority.
	DiscoveryShootCA = "shoot-ca"

	// LabelExposureClassHandlerName is the label key for exposure class handler names.
	LabelExposureClassHandlerName = "handler.exposureclass.gardener.cloud/name"

	// LabelNodeLocalDNS is a constant for a label key, which the provider extensions set on the nodes.
	// The value can be true or false.
	LabelNodeLocalDNS = "networking.gardener.cloud/node-local-dns-enabled"

	// LabelVPAEvictionRequirementsController is a constant for a label indicating that a VPA resource is under control
	// of the VPAEvictionRequirementsController.
	LabelVPAEvictionRequirementsController = "autoscaling.gardener.cloud/eviction-requirements"
	// EvictionRequirementManagedByController is a constant to be used as a value for the label LabelVPAEvictionRequirementsController
	// to indicate that the vpa-eviction-requirements-controller manages all EvictionRequirements on a VPA object.
	EvictionRequirementManagedByController = "managed-by-controller"

	// AnnotationVPAEvictionRequirementDownscaleRestriction is a constant for an annotation key on a VPA object indicating that
	// the VPAEvictionRequirementsController should add an EvictionRestriction that prevents downscaling.
	// Possible values are "in-maintenance-window-only" and "never", available as constants below.
	AnnotationVPAEvictionRequirementDownscaleRestriction = "eviction-requirements.autoscaling.gardener.cloud/downscale-restriction"
	// EvictionRequirementInMaintenanceWindowOnly is a constant to be used as a value for the annotation AnnotationVPAEvictionRequirementDownscaleRestriction,
	// indicating that downscaling should be restricted to the Shoot's maintenance window.
	EvictionRequirementInMaintenanceWindowOnly = "in-maintenance-window-only"
	// EvictionRequirementNever is a constant to be used as a value for the annotation AnnotationVPAEvictionRequirementDownscaleRestriction,
	// indicating that downscaling should never be allowed.
	EvictionRequirementNever = "never"
	// AnnotationShootMaintenanceWindow is a constant for an annotation key used on VPA objects to hold the Shoot's maintenance window start and end.
	AnnotationShootMaintenanceWindow = "shoot.gardener.cloud/maintenance-window"

	// GardenNamespace is the namespace in which the configuration and secrets for
	// the Gardener controller manager will be stored (e.g., secrets for the Seed clusters).
	// It is also used by the gardener-apiserver.
	GardenNamespace = "garden"
	// IstioSystemNamespace is the istio-system namespace.
	IstioSystemNamespace = "istio-system"
	// KubernetesDashboardNamespace is the kubernetes-dashboard namespace.
	KubernetesDashboardNamespace = "kubernetes-dashboard"

	// DefaultSNIIngressNamespace is the default sni ingress namespace.
	DefaultSNIIngressNamespace = "istio-ingress"
	// DefaultSNIIngressServiceName is the default sni ingress service name.
	DefaultSNIIngressServiceName = "istio-ingressgateway"
	// DefaultIngressGatewayAppLabelValue is the ingress gateway value for the app label.
	DefaultIngressGatewayAppLabelValue = "istio-ingressgateway"

	// DataTypeSecret is a constant for a value of the 'Type' field in 'GardenerResourceData' structs describing that
	// the data is a secret.
	DataTypeSecret = "secret"
	// DataTypeMachineState is a constant for a value of the 'Type' field in 'GardenerResourceData' structs describing
	// that the data is machine state.
	DataTypeMachineState = "machine-state"

	// DefaultSchedulerName is the name of the default scheduler.
	DefaultSchedulerName = "default-scheduler"
	// SchedulingPurpose is a constant for the key in a label describing the purpose of the scheduler related object.
	SchedulingPurpose = "scheduling.gardener.cloud/purpose"
	// SchedulingPurposeRegionConfig is a constant for a label value indicating that the object should be considered as a region config.
	SchedulingPurposeRegionConfig = "region-config"
	// AnnotationSchedulingCloudProfiles is a constant for an annotation key on a configmap which denotes
	// the linked cloudprofiles containing the region distances.
	AnnotationSchedulingCloudProfiles = "scheduling.gardener.cloud/cloudprofiles"

	// AnnotationConfirmationForceDeletion is a constant for an annotation on a Shoot resource whose value must be set to "true" in order to
	// trigger force-deletion of the cluster. It can only be set if the Shoot has a deletion timestamp and contains an ErrorCode in the Shoot Status.
	AnnotationConfirmationForceDeletion = "confirmation.gardener.cloud/force-deletion"
	// AnnotationShootIgnoreAlerts is the key for an annotation of a Shoot cluster whose value indicates
	// if alerts for this cluster should be ignored
	AnnotationShootIgnoreAlerts = "shoot.gardener.cloud/ignore-alerts"
	// AnnotationShootSkipCleanup is a key for an annotation on a Shoot resource that declares that the clean up steps should be skipped when the
	// cluster is deleted. Concretely, this will skip everything except the deletion of (load balancer) services and persistent volume resources.
	AnnotationShootSkipCleanup = "shoot.gardener.cloud/skip-cleanup"
	// AnnotationShootSkipReadiness is a key for an annotation on a Shoot resource that instructs the shoot flow to skip readiness steps during reconciliation.
	AnnotationShootSkipReadiness = "shoot.gardener.cloud/skip-readiness"
	// AnnotationShootCleanupWebhooksFinalizeGracePeriodSeconds is a key for an annotation on a Shoot resource that
	// declares the grace period in seconds for finalizing the resources handled in the 'cleanup webhooks' step.
	// Concretely, after the specified seconds, all the finalizers of the affected resources are forcefully removed.
	AnnotationShootCleanupWebhooksFinalizeGracePeriodSeconds = "shoot.gardener.cloud/cleanup-webhooks-finalize-grace-period-seconds"
	// AnnotationShootCleanupExtendedAPIsFinalizeGracePeriodSeconds is a key for an annotation on a Shoot resource that
	// declares the grace period in seconds for finalizing the resources handled in the 'cleanup extended APIs' step.
	// Concretely, after the specified seconds, all the finalizers of the affected resources are forcefully removed.
	AnnotationShootCleanupExtendedAPIsFinalizeGracePeriodSeconds = "shoot.gardener.cloud/cleanup-extended-apis-finalize-grace-period-seconds"
	// AnnotationShootCleanupKubernetesResourcesFinalizeGracePeriodSeconds is a key for an annotation on a Shoot
	// resource that declares the grace period in seconds for finalizing the resources handled in the 'cleanup
	// Kubernetes resources' step. Concretely, after the specified seconds, all the finalizers of the affected resources
	// are forcefully removed.
	AnnotationShootCleanupKubernetesResourcesFinalizeGracePeriodSeconds = "shoot.gardener.cloud/cleanup-kubernetes-resources-finalize-grace-period-seconds"
	// AnnotationShootCloudConfigExecutionMaxDelaySeconds is a key for an annotation on a Shoot resource that declares
	// the maximum delay in seconds when potentially updated cloud-config user data is executed on the worker nodes.
	// Concretely, the gardener-node-agent systemd service running on all worker nodes will wait
	// for a random duration based on the configured value before executing the user data (default value is 300) plus an
	// additional offset of 30s. If set to 0 then no random delay will be applied and the minimum delay (30s) applies.
	// Any value above 1800 is ignored (in this case the default value is used).
	// Note that changing this value only applies to new nodes. Existing nodes which already computed their individual
	// delays will not recompute it.
	AnnotationShootCloudConfigExecutionMaxDelaySeconds = "shoot.gardener.cloud/cloud-config-execution-max-delay-seconds"

	// AnnotationAuthenticationIssuer is the key for an annotation applied to a Shoot which specifies
	// if the shoot's issuer is managed by Gardener.
	AnnotationAuthenticationIssuer = "authentication.gardener.cloud/issuer"
	// AnnotationAuthenticationIssuerManaged is the value for [AnnotationAuthenticationIssuer] annotation that indicates that
	// a shoot's issuer should be managed by Gardener.
	AnnotationAuthenticationIssuerManaged = "managed"

	// AnnotationPodSecurityEnforce is a constant for an annotation on `ControllerRegistration`s and `ControllerInstallation`s. When set the
	// `extension` namespace is created with "pod-security.kubernetes.io/enforce" label set to AnnotationPodSecurityEnforce's value.
	AnnotationPodSecurityEnforce = "security.gardener.cloud/pod-security-enforce"
	// OperatingSystemConfigUnitNameKubeletService is a constant for a unit in the operating system config that contains the kubelet service.
	OperatingSystemConfigUnitNameKubeletService = "kubelet.service"
	// OperatingSystemConfigUnitNameContainerDService is a constant for a unit in the operating system config that contains the containerd service.
	OperatingSystemConfigUnitNameContainerDService = "containerd.service"
	// OperatingSystemConfigFilePathKernelSettings is a constant for a path to a file in the operating system config that contains some general kernel settings.
	OperatingSystemConfigFilePathKernelSettings = "/etc/sysctl.d/99-k8s-general.conf"
	// OperatingSystemConfigFilePathKubeletConfig is a constant for a path to a file in the operating system config that contains the kubelet configuration.
	OperatingSystemConfigFilePathKubeletConfig = "/var/lib/kubelet/config/kubelet"
	// OperatingSystemConfigUnitNameValitailService is a constant for a unit in the operating system config that contains the valitail service.
	OperatingSystemConfigUnitNameValitailService = "valitail.service"
	// OperatingSystemConfigFilePathValitailConfig is a constant for a path to a file in the operating system config that contains the kubelet configuration.
	OperatingSystemConfigFilePathValitailConfig = "/var/lib/valitail/config/config"
	// OperatingSystemConfigFilePathBinaries is a constant for a path to a directory in the operating system config that contains the binaries.
	OperatingSystemConfigFilePathBinaries = "/opt/bin"

	// FluentBitConfigMapKubernetesFilter is a constant for the Fluent Bit ConfigMap's section regarding Kubernetes filters
	FluentBitConfigMapKubernetesFilter = "filter-kubernetes.conf"
	// FluentBitConfigMapParser is a constant for the Fluent Bit ConfigMap's section regarding Parsers for common container types
	FluentBitConfigMapParser = "parsers.conf"

	// LabelControllerRegistrationName is the key of a label on extension namespaces that indicates the controller registration name.
	LabelControllerRegistrationName = "controllerregistration.core.gardener.cloud/name"
	// LabelPodMaintenanceRestart is a constant for a label that describes that a pod should be restarted during maintenance.
	LabelPodMaintenanceRestart = "maintenance.gardener.cloud/restart"
	// LabelCareConditionType is a key for a label on a ManagedResource indicating to which condition type its status
	// should be aggregated.
	LabelCareConditionType = "care.gardener.cloud/condition-type"
	// ObservabilityComponentsHealthy is a constant for a condition type indicating the health of observability components.
	ObservabilityComponentsHealthy = "ObservabilityComponentsHealthy"

	// LabelWorkerName is a constant for a label that indicates the name of the Worker resource the MachineDeployment belongs to.
	LabelWorkerName = "worker.gardener.cloud/name"
	// LabelWorkerPool is a constant for a label that indicates the worker pool the node belongs to
	LabelWorkerPool = "worker.gardener.cloud/pool"
	// LabelWorkerKubernetesVersion is a constant for a label that indicates the Kubernetes version used for the worker pool nodes.
	LabelWorkerKubernetesVersion = "worker.gardener.cloud/kubernetes-version"
	// LabelWorkerPoolDeprecated is a deprecated constant for a label that indicates the worker pool the node belongs to
	LabelWorkerPoolDeprecated = "worker.garden.sapcloud.io/group"
	// LabelWorkerPoolSystemComponents is a constant that indicates whether the worker pool should host system components
	LabelWorkerPoolSystemComponents = "worker.gardener.cloud/system-components"
	// LabelWorkerPoolGardenerNodeAgentSecretName is the name of the secret used by the gardener node agent
	LabelWorkerPoolGardenerNodeAgentSecretName = "worker.gardener.cloud/gardener-node-agent-secret-name"

	// LabelUpdateRestriction is a constant for a label key that indicates
	// that a resource must be only updated by the gardenlet.
	LabelUpdateRestriction = "gardener.cloud/update-restriction"

	// EventResourceReferenced indicates that the resource deletion is in waiting mode because the resource is still
	// being referenced by at least one other resource (e.g. a SecretBinding is still referenced by a Shoot)
	EventResourceReferenced = "ResourceReferenced"

	// ReferencedResourcesPrefix is the prefix used when copying referenced resources to the Shoot namespace in the Seed,
	// to avoid naming collisions with resources managed by Gardener.
	ReferencedResourcesPrefix = "ref-"

	// ClusterIdentity is a constant equal to the name and data key (that stores the identity) of the cluster-identity ConfigMap
	ClusterIdentity = "cluster-identity"
	// ClusterIdentityOrigin is a constant equal to the data key that stores the identity origin of the cluster-identity ConfigMap
	ClusterIdentityOrigin = "origin"
	// ClusterIdentityOriginGardenerAPIServer defines a cluster-identity ConfigMap originated from gardener-apiserver
	ClusterIdentityOriginGardenerAPIServer = "gardener-apiserver"
	// ClusterIdentityOriginSeed defines a cluster-identity ConfigMap originated from seed
	ClusterIdentityOriginSeed = "seed"
	// ClusterIdentityOriginShoot defines a cluster-identity ConfigMap originated from shoot
	ClusterIdentityOriginShoot = "shoot"

	// SeedNginxIngressClass defines the ingress class for the seed nginx ingress controller
	SeedNginxIngressClass = "nginx-ingress-gardener"
	// ShootNginxIngressClass defines the ingress class for the shoot nginx ingress controller addon.
	ShootNginxIngressClass = "nginx"
	// IngressKindNginx defines nginx as kind as managed Seed ingress
	IngressKindNginx = "nginx"

	// SeedsGroup is the identity group for gardenlets when authenticating to the API server.
	SeedsGroup = "gardener.cloud:system:seeds"
	// SeedUserNamePrefix is the identity user name prefix for gardenlets when authenticating to the API server.
	SeedUserNamePrefix = "gardener.cloud:system:seed:"

	// ShootGroupViewers is a constant for a group name in shoot clusters whose users get read-only privileges (except
	// for core/v1.Secrets).
	ShootGroupViewers = "gardener.cloud:system:viewers"
	// ClusterRoleNameGardenerAdministrators is the name of a cluster role in the garden cluster defining privileges
	// for administrators.
	ClusterRoleNameGardenerAdministrators = "gardener.cloud:system:administrators"

	// ProjectName is the key of a label on namespaces whose value holds the project name.
	ProjectName = "project.gardener.cloud/name"
	// ProjectSkipStaleCheck is the key of an annotation on a project namespace that marks the associated Project to be
	// skipped by the stale project controller. If the project has already configured stale timestamps in its status
	// then they will be reset.
	ProjectSkipStaleCheck = "project.gardener.cloud/skip-stale-check"
	// NamespaceProject is the key of an annotation on namespace whose value holds the project uid.
	NamespaceProject = "namespace.gardener.cloud/project"
	// NamespaceKeepAfterProjectDeletion is a constant for an annotation on a `Namespace` resource that states that it
	// should not be deleted if the corresponding `Project` gets deleted. Please note that all project related labels
	// from the namespace will be removed when the project is being deleted.
	NamespaceKeepAfterProjectDeletion = "namespace.gardener.cloud/keep-after-project-deletion"
	// NamespaceCreatedByProjectController is a constant for annotation on a `Namespace` resource that states that it
	// was created by the project controller because either the Project's `spec.namespace` field was not specified
	// or the specified namespace was not present.
	NamespaceCreatedByProjectController = "namespace.gardener.cloud/created-by-project-controller"

	// DefaultVPNRangeV6 is the default IPv6 network range for the VPN between seed and shoot cluster.
	DefaultVPNRangeV6 = "fd8f:6d53:b97a:1::/96"
	// ReservedKubeApiServerMappingRange is the IPv4 network range for the "kubernetes" service used by apiserver-proxy
	ReservedKubeApiServerMappingRange = "240.0.0.0/8"
	// ReservedSeedPodNetworkMappedRange is the IPv4 network range for the seed pod network used in the VPN between seed and shoot cluster.
	ReservedSeedPodNetworkMappedRange = "241.0.0.0/8"
	// ReservedShootNodeNetworkMappedRange is the IPv4 network range for the shoot node network used in the VPN between seed and shoot cluster.
	ReservedShootNodeNetworkMappedRange = "242.0.0.0/8"
	// ReservedShootServiceNetworkMappedRange is the IPv4 network range for the shoot service network used in the VPN between seed and shoot cluster.
	ReservedShootServiceNetworkMappedRange = "243.0.0.0/8"
	// ReservedShootPodNetworkMappedRange is the IPv4 network range for the shoot pod network used in the VPN between seed and shoot cluster.
	ReservedShootPodNetworkMappedRange = "244.0.0.0/8"
	// EnvoyNonRootUserId is the user ID for the non-root user in the envoy container.
	EnvoyNonRootUserId = 65532

	// BackupSecretName is the name of secret having credentials for etcd backups.
	BackupSecretName string = "etcd-backup"
	// DataKeyBackupBucketName is the name of a data key whose value contains the backup bucket name.
	DataKeyBackupBucketName string = "bucketName"
	// BackupSourcePrefix is the prefix for names of resources related to source backupentries when copying backups.
	BackupSourcePrefix = "source"

	// GardenerAudience is the identifier for Gardener controllers when interacting with the API Server
	GardenerAudience = "gardener"

	// DNSRecordInternalName is a constant for DNSRecord objects used for the internal domain name.
	DNSRecordInternalName = "internal"
	// DNSRecordExternalName is a constant for DNSRecord objects used for the external domain name.
	DNSRecordExternalName = "external"

	// ArchitectureName is a constant for the 'architecture' cloud profile capability name.
	ArchitectureName = "architecture"
	// ArchitectureAMD64 is a constant for the 'amd64' architecture.
	ArchitectureAMD64 = "amd64"
	// ArchitectureARM64 is a constant for the 'arm64' architecture.
	ArchitectureARM64 = "arm64"

	// EnvGenericGardenKubeconfig is a constant for the environment variable which holds the path to the generic garden kubeconfig.
	EnvGenericGardenKubeconfig = "GARDEN_KUBECONFIG"
	// EnvSeedName is a constant for the environment variable which holds the name of the Seed that the extension
	// controller is running on.
	EnvSeedName = "SEED_NAME"

	// IngressTLSCertificateValidity is the default validity for ingress TLS certificates.
	IngressTLSCertificateValidity = 730 * 24 * time.Hour // ~2 years, see https://support.apple.com/en-us/HT210176
	// IngressDomainPrefixPrometheusAggregate is the prefix of a domain exposing prometheus-aggregate in seed clusters.
	IngressDomainPrefixPrometheusAggregate = "p-seed"

	// VPNTunnel dictates that VPN is used as a tunnel between seed and shoot networks.
	VPNTunnel string = "vpn-shoot"

	// AdvertisedAddressExternal is a constant that represents the name of the external kube-apiserver address.
	AdvertisedAddressExternal = "external"
	// AdvertisedAddressInternal is a constant that represents the name of the internal kube-apiserver address.
	AdvertisedAddressInternal = "internal"
	// AdvertisedAddressUnmanaged is a constant that represents the name of the unmanaged kube-apiserver address.
	AdvertisedAddressUnmanaged = "unmanaged"
	// AdvertisedAddressServiceAccountIssuer is a constant that represents the name of the address that is used as a
	// service account issuer for the kube-apiserver.
	AdvertisedAddressServiceAccountIssuer = "service-account-issuer"
	// AdvertisedAddressWildcardTLSSeedBound is a constant that represents the name of the address that is
	// seed-specific (i.e., changes when the Seed changes) and backed by a central wildcard TLS certificate.
	AdvertisedAddressWildcardTLSSeedBound = "wildcard-tls-seed-bound"

	// CloudProfileReferenceKindCloudProfile is a constant for the CloudProfile kind reference.
	CloudProfileReferenceKindCloudProfile = "CloudProfile"
	// CloudProfileReferenceKindNamespacedCloudProfile is a constant for the NamespacedCloudProfile kind reference.
	CloudProfileReferenceKindNamespacedCloudProfile = "NamespacedCloudProfile"
)

var (
	// ControlPlaneSecretRoles contains all role values used for control plane secrets synced to the Garden cluster.
	ControlPlaneSecretRoles = []string{
		GardenRoleKubeconfig,
		GardenRoleSSHKeyPair,
		GardenRoleMonitoring,
	}

	// ValidArchitectures contains all CPU architectures which are supported by the Shoot.
	ValidArchitectures = []string{
		ArchitectureAMD64,
		ArchitectureARM64,
	}
)

// constants for well-known PriorityClass names
const (
	// PriorityClassNameGardenSystemCritical is the name of a PriorityClass for Garden system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameGardenSystemCritical = "gardener-garden-system-critical"
	// PriorityClassNameGardenSystem500 is the name of a PriorityClass for Garden system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameGardenSystem500 = "gardener-garden-system-500"
	// PriorityClassNameGardenSystem400 is the name of a PriorityClass for Garden system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameGardenSystem400 = "gardener-garden-system-400"
	// PriorityClassNameGardenSystem300 is the name of a PriorityClass for Garden system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameGardenSystem300 = "gardener-garden-system-300"
	// PriorityClassNameGardenSystem200 is the name of a PriorityClass for Garden system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameGardenSystem200 = "gardener-garden-system-200"
	// PriorityClassNameGardenSystem100 is the name of a PriorityClass for Garden system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameGardenSystem100 = "gardener-garden-system-100"

	// PriorityClassNameShootSystem900 is the name of a PriorityClass for Shoot system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameShootSystem900 = "gardener-shoot-system-900"
	// PriorityClassNameShootSystem800 is the name of a PriorityClass for Shoot system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameShootSystem800 = "gardener-shoot-system-800"
	// PriorityClassNameShootSystem700 is the name of a PriorityClass for Shoot system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameShootSystem700 = "gardener-shoot-system-700"
	// PriorityClassNameShootSystem600 is the name of a PriorityClass for Shoot system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameShootSystem600 = "gardener-shoot-system-600"

	// PriorityClassNameSeedSystemCritical is the name of a PriorityClass for Seed system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameSeedSystemCritical = "gardener-system-critical"
	// PriorityClassNameSeedSystem900 is the name of a PriorityClass for Seed system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameSeedSystem900 = "gardener-system-900"
	// PriorityClassNameSeedSystem800 is the name of a PriorityClass for Seed system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameSeedSystem800 = "gardener-system-800"
	// PriorityClassNameSeedSystem700 is the name of a PriorityClass for Seed system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameSeedSystem700 = "gardener-system-700"
	// PriorityClassNameSeedSystem600 is the name of a PriorityClass for Seed system components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameSeedSystem600 = "gardener-system-600"
	// PriorityClassNameReserveExcessCapacity is the name of a PriorityClass for reserving excess capacity on a Seed cluster.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameReserveExcessCapacity = "gardener-reserve-excess-capacity"

	// PriorityClassNameShootControlPlane500 is the name of a PriorityClass for Shoot control plane components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameShootControlPlane500 = "gardener-system-500"
	// PriorityClassNameShootControlPlane400 is the name of a PriorityClass for Shoot control plane components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameShootControlPlane400 = "gardener-system-400"
	// PriorityClassNameShootControlPlane300 is the name of a PriorityClass for Shoot control plane components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameShootControlPlane300 = "gardener-system-300"
	// PriorityClassNameShootControlPlane200 is the name of a PriorityClass for Shoot control plane components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameShootControlPlane200 = "gardener-system-200"
	// PriorityClassNameShootControlPlane100 is the name of a PriorityClass for Shoot control plane components.
	// Please consider the documentation in https://github.com/gardener/gardener/blob/master/docs/development/priority-classes.md
	PriorityClassNameShootControlPlane100 = "gardener-system-100"

	// TechnicalIDPrefix is a prefix used for a shoot's technical id. For historic reasons, there is only one 'dash'
	// while nowadays we always use two dashes after "shoot".
	TechnicalIDPrefix = "shoot-"

	// TaintNodeCriticalComponentsNotReady is the key for the gardener-managed node components taint.
	TaintNodeCriticalComponentsNotReady = "node.gardener.cloud/critical-components-not-ready"
	// LabelNodeCriticalComponent is the label key for marking node-critical component pods.
	LabelNodeCriticalComponent = "node.gardener.cloud/critical-component"
	// AnnotationPrefixWaitForCSINode is the annotation key for csi-driver-node pods, indicating they use the driver
	// specified in the value.
	AnnotationPrefixWaitForCSINode = "node.gardener.cloud/wait-for-csi-node-"
	// AnnotationNodeAgentReconciliationDelay is the annotation key for specifying how long the gardener-node-agent
	// should wait with reconciliation of the operating system config (to prevent too many node-agents from restarting
	// kubelet or other critical units at the same time).
	AnnotationNodeAgentReconciliationDelay = "node-agent.gardener.cloud/reconciliation-delay"
	// NodeAgentsGroup is the identity group for gardener-node-agents when authenticating to the API server.
	NodeAgentsGroup = "gardener.cloud:node-agents"
	// NodeAgentUserNamePrefix is the identity username prefix for gardener-node-agent when authenticating to the API server.
	NodeAgentUserNamePrefix = "gardener.cloud:node-agent:machine:"

	// GardenPurposeMachineClass is a constant for the 'machineclass' value in a label.
	GardenPurposeMachineClass = "machineclass"

	// LabelInjectGardenKubeconfig is a constant for a label on workload resources that indicates that a kubeconfig to
	// the garden cluster should be injected.
	LabelInjectGardenKubeconfig = "extensions.gardener.cloud/inject-garden-kubeconfig"
)
