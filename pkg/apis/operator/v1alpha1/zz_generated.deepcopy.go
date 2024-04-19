//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuditWebhook) DeepCopyInto(out *AuditWebhook) {
	*out = *in
	if in.BatchMaxSize != nil {
		in, out := &in.BatchMaxSize, &out.BatchMaxSize
		*out = new(int32)
		**out = **in
	}
	if in.Version != nil {
		in, out := &in.Version, &out.Version
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuditWebhook.
func (in *AuditWebhook) DeepCopy() *AuditWebhook {
	if in == nil {
		return nil
	}
	out := new(AuditWebhook)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Authentication) DeepCopyInto(out *Authentication) {
	*out = *in
	if in.Webhook != nil {
		in, out := &in.Webhook, &out.Webhook
		*out = new(AuthenticationWebhook)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Authentication.
func (in *Authentication) DeepCopy() *Authentication {
	if in == nil {
		return nil
	}
	out := new(Authentication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthenticationWebhook) DeepCopyInto(out *AuthenticationWebhook) {
	*out = *in
	if in.CacheTTL != nil {
		in, out := &in.CacheTTL, &out.CacheTTL
		*out = new(v1.Duration)
		**out = **in
	}
	if in.Version != nil {
		in, out := &in.Version, &out.Version
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthenticationWebhook.
func (in *AuthenticationWebhook) DeepCopy() *AuthenticationWebhook {
	if in == nil {
		return nil
	}
	out := new(AuthenticationWebhook)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Backup) DeepCopyInto(out *Backup) {
	*out = *in
	out.SecretRef = in.SecretRef
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Backup.
func (in *Backup) DeepCopy() *Backup {
	if in == nil {
		return nil
	}
	out := new(Backup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlane) DeepCopyInto(out *ControlPlane) {
	*out = *in
	if in.HighAvailability != nil {
		in, out := &in.HighAvailability, &out.HighAvailability
		*out = new(HighAvailability)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlane.
func (in *ControlPlane) DeepCopy() *ControlPlane {
	if in == nil {
		return nil
	}
	out := new(ControlPlane)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Credentials) DeepCopyInto(out *Credentials) {
	*out = *in
	if in.Rotation != nil {
		in, out := &in.Rotation, &out.Rotation
		*out = new(CredentialsRotation)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Credentials.
func (in *Credentials) DeepCopy() *Credentials {
	if in == nil {
		return nil
	}
	out := new(Credentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CredentialsRotation) DeepCopyInto(out *CredentialsRotation) {
	*out = *in
	if in.CertificateAuthorities != nil {
		in, out := &in.CertificateAuthorities, &out.CertificateAuthorities
		*out = new(v1beta1.CARotation)
		(*in).DeepCopyInto(*out)
	}
	if in.ServiceAccountKey != nil {
		in, out := &in.ServiceAccountKey, &out.ServiceAccountKey
		*out = new(v1beta1.ServiceAccountKeyRotation)
		(*in).DeepCopyInto(*out)
	}
	if in.ETCDEncryptionKey != nil {
		in, out := &in.ETCDEncryptionKey, &out.ETCDEncryptionKey
		*out = new(v1beta1.ETCDEncryptionKeyRotation)
		(*in).DeepCopyInto(*out)
	}
	if in.Observability != nil {
		in, out := &in.Observability, &out.Observability
		*out = new(v1beta1.ObservabilityRotation)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CredentialsRotation.
func (in *CredentialsRotation) DeepCopy() *CredentialsRotation {
	if in == nil {
		return nil
	}
	out := new(CredentialsRotation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DNS) DeepCopyInto(out *DNS) {
	*out = *in
	if in.Domains != nil {
		in, out := &in.Domains, &out.Domains
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DNS.
func (in *DNS) DeepCopy() *DNS {
	if in == nil {
		return nil
	}
	out := new(DNS)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DashboardGitHub) DeepCopyInto(out *DashboardGitHub) {
	*out = *in
	out.SecretRef = in.SecretRef
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DashboardGitHub.
func (in *DashboardGitHub) DeepCopy() *DashboardGitHub {
	if in == nil {
		return nil
	}
	out := new(DashboardGitHub)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DashboardOIDC) DeepCopyInto(out *DashboardOIDC) {
	*out = *in
	if in.SessionLifetime != nil {
		in, out := &in.SessionLifetime, &out.SessionLifetime
		*out = new(v1.Duration)
		**out = **in
	}
	if in.AdditionalScopes != nil {
		in, out := &in.AdditionalScopes, &out.AdditionalScopes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.SecretRef = in.SecretRef
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DashboardOIDC.
func (in *DashboardOIDC) DeepCopy() *DashboardOIDC {
	if in == nil {
		return nil
	}
	out := new(DashboardOIDC)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DashboardTerminal) DeepCopyInto(out *DashboardTerminal) {
	*out = *in
	in.Container.DeepCopyInto(&out.Container)
	if in.AllowedHosts != nil {
		in, out := &in.AllowedHosts, &out.AllowedHosts
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DashboardTerminal.
func (in *DashboardTerminal) DeepCopy() *DashboardTerminal {
	if in == nil {
		return nil
	}
	out := new(DashboardTerminal)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DashboardTerminalContainer) DeepCopyInto(out *DashboardTerminalContainer) {
	*out = *in
	if in.Description != nil {
		in, out := &in.Description, &out.Description
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DashboardTerminalContainer.
func (in *DashboardTerminalContainer) DeepCopy() *DashboardTerminalContainer {
	if in == nil {
		return nil
	}
	out := new(DashboardTerminalContainer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ETCD) DeepCopyInto(out *ETCD) {
	*out = *in
	if in.Main != nil {
		in, out := &in.Main, &out.Main
		*out = new(ETCDMain)
		(*in).DeepCopyInto(*out)
	}
	if in.Events != nil {
		in, out := &in.Events, &out.Events
		*out = new(ETCDEvents)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ETCD.
func (in *ETCD) DeepCopy() *ETCD {
	if in == nil {
		return nil
	}
	out := new(ETCD)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ETCDEvents) DeepCopyInto(out *ETCDEvents) {
	*out = *in
	if in.Storage != nil {
		in, out := &in.Storage, &out.Storage
		*out = new(Storage)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ETCDEvents.
func (in *ETCDEvents) DeepCopy() *ETCDEvents {
	if in == nil {
		return nil
	}
	out := new(ETCDEvents)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ETCDMain) DeepCopyInto(out *ETCDMain) {
	*out = *in
	if in.Backup != nil {
		in, out := &in.Backup, &out.Backup
		*out = new(Backup)
		**out = **in
	}
	if in.Storage != nil {
		in, out := &in.Storage, &out.Storage
		*out = new(Storage)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ETCDMain.
func (in *ETCDMain) DeepCopy() *ETCDMain {
	if in == nil {
		return nil
	}
	out := new(ETCDMain)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Garden) DeepCopyInto(out *Garden) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Garden.
func (in *Garden) DeepCopy() *Garden {
	if in == nil {
		return nil
	}
	out := new(Garden)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Garden) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenList) DeepCopyInto(out *GardenList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Garden, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenList.
func (in *GardenList) DeepCopy() *GardenList {
	if in == nil {
		return nil
	}
	out := new(GardenList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GardenList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenSpec) DeepCopyInto(out *GardenSpec) {
	*out = *in
	in.RuntimeCluster.DeepCopyInto(&out.RuntimeCluster)
	in.VirtualCluster.DeepCopyInto(&out.VirtualCluster)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenSpec.
func (in *GardenSpec) DeepCopy() *GardenSpec {
	if in == nil {
		return nil
	}
	out := new(GardenSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenStatus) DeepCopyInto(out *GardenStatus) {
	*out = *in
	if in.Gardener != nil {
		in, out := &in.Gardener, &out.Gardener
		*out = new(v1beta1.Gardener)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1beta1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LastOperation != nil {
		in, out := &in.LastOperation, &out.LastOperation
		*out = new(v1beta1.LastOperation)
		(*in).DeepCopyInto(*out)
	}
	if in.Credentials != nil {
		in, out := &in.Credentials, &out.Credentials
		*out = new(Credentials)
		(*in).DeepCopyInto(*out)
	}
	if in.EncryptedResources != nil {
		in, out := &in.EncryptedResources, &out.EncryptedResources
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenStatus.
func (in *GardenStatus) DeepCopy() *GardenStatus {
	if in == nil {
		return nil
	}
	out := new(GardenStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Gardener) DeepCopyInto(out *Gardener) {
	*out = *in
	if in.APIServer != nil {
		in, out := &in.APIServer, &out.APIServer
		*out = new(GardenerAPIServerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.AdmissionController != nil {
		in, out := &in.AdmissionController, &out.AdmissionController
		*out = new(GardenerAdmissionControllerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.ControllerManager != nil {
		in, out := &in.ControllerManager, &out.ControllerManager
		*out = new(GardenerControllerManagerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Scheduler != nil {
		in, out := &in.Scheduler, &out.Scheduler
		*out = new(GardenerSchedulerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Dashboard != nil {
		in, out := &in.Dashboard, &out.Dashboard
		*out = new(GardenerDashboardConfig)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Gardener.
func (in *Gardener) DeepCopy() *Gardener {
	if in == nil {
		return nil
	}
	out := new(Gardener)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenerAPIServerConfig) DeepCopyInto(out *GardenerAPIServerConfig) {
	*out = *in
	in.KubernetesConfig.DeepCopyInto(&out.KubernetesConfig)
	if in.AdmissionPlugins != nil {
		in, out := &in.AdmissionPlugins, &out.AdmissionPlugins
		*out = make([]v1beta1.AdmissionPlugin, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AuditConfig != nil {
		in, out := &in.AuditConfig, &out.AuditConfig
		*out = new(v1beta1.AuditConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.AuditWebhook != nil {
		in, out := &in.AuditWebhook, &out.AuditWebhook
		*out = new(AuditWebhook)
		(*in).DeepCopyInto(*out)
	}
	if in.Logging != nil {
		in, out := &in.Logging, &out.Logging
		*out = new(v1beta1.APIServerLogging)
		(*in).DeepCopyInto(*out)
	}
	if in.Requests != nil {
		in, out := &in.Requests, &out.Requests
		*out = new(v1beta1.APIServerRequests)
		(*in).DeepCopyInto(*out)
	}
	if in.WatchCacheSizes != nil {
		in, out := &in.WatchCacheSizes, &out.WatchCacheSizes
		*out = new(v1beta1.WatchCacheSizes)
		(*in).DeepCopyInto(*out)
	}
	if in.EncryptionConfig != nil {
		in, out := &in.EncryptionConfig, &out.EncryptionConfig
		*out = new(v1beta1.EncryptionConfig)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenerAPIServerConfig.
func (in *GardenerAPIServerConfig) DeepCopy() *GardenerAPIServerConfig {
	if in == nil {
		return nil
	}
	out := new(GardenerAPIServerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenerAdmissionControllerConfig) DeepCopyInto(out *GardenerAdmissionControllerConfig) {
	*out = *in
	if in.LogLevel != nil {
		in, out := &in.LogLevel, &out.LogLevel
		*out = new(string)
		**out = **in
	}
	if in.ResourceAdmissionConfiguration != nil {
		in, out := &in.ResourceAdmissionConfiguration, &out.ResourceAdmissionConfiguration
		*out = new(ResourceAdmissionConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenerAdmissionControllerConfig.
func (in *GardenerAdmissionControllerConfig) DeepCopy() *GardenerAdmissionControllerConfig {
	if in == nil {
		return nil
	}
	out := new(GardenerAdmissionControllerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenerControllerManagerConfig) DeepCopyInto(out *GardenerControllerManagerConfig) {
	*out = *in
	in.KubernetesConfig.DeepCopyInto(&out.KubernetesConfig)
	if in.DefaultProjectQuotas != nil {
		in, out := &in.DefaultProjectQuotas, &out.DefaultProjectQuotas
		*out = make([]ProjectQuotaConfiguration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LogLevel != nil {
		in, out := &in.LogLevel, &out.LogLevel
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenerControllerManagerConfig.
func (in *GardenerControllerManagerConfig) DeepCopy() *GardenerControllerManagerConfig {
	if in == nil {
		return nil
	}
	out := new(GardenerControllerManagerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenerDashboardConfig) DeepCopyInto(out *GardenerDashboardConfig) {
	*out = *in
	if in.EnableTokenLogin != nil {
		in, out := &in.EnableTokenLogin, &out.EnableTokenLogin
		*out = new(bool)
		**out = **in
	}
	if in.FrontendConfigMapRef != nil {
		in, out := &in.FrontendConfigMapRef, &out.FrontendConfigMapRef
		*out = new(corev1.LocalObjectReference)
		**out = **in
	}
	if in.AssetsConfigMapRef != nil {
		in, out := &in.AssetsConfigMapRef, &out.AssetsConfigMapRef
		*out = new(corev1.LocalObjectReference)
		**out = **in
	}
	if in.GitHub != nil {
		in, out := &in.GitHub, &out.GitHub
		*out = new(DashboardGitHub)
		**out = **in
	}
	if in.LogLevel != nil {
		in, out := &in.LogLevel, &out.LogLevel
		*out = new(string)
		**out = **in
	}
	if in.OIDC != nil {
		in, out := &in.OIDC, &out.OIDC
		*out = new(DashboardOIDC)
		(*in).DeepCopyInto(*out)
	}
	if in.Terminal != nil {
		in, out := &in.Terminal, &out.Terminal
		*out = new(DashboardTerminal)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenerDashboardConfig.
func (in *GardenerDashboardConfig) DeepCopy() *GardenerDashboardConfig {
	if in == nil {
		return nil
	}
	out := new(GardenerDashboardConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenerSchedulerConfig) DeepCopyInto(out *GardenerSchedulerConfig) {
	*out = *in
	in.KubernetesConfig.DeepCopyInto(&out.KubernetesConfig)
	if in.LogLevel != nil {
		in, out := &in.LogLevel, &out.LogLevel
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenerSchedulerConfig.
func (in *GardenerSchedulerConfig) DeepCopy() *GardenerSchedulerConfig {
	if in == nil {
		return nil
	}
	out := new(GardenerSchedulerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GroupResource) DeepCopyInto(out *GroupResource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GroupResource.
func (in *GroupResource) DeepCopy() *GroupResource {
	if in == nil {
		return nil
	}
	out := new(GroupResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HighAvailability) DeepCopyInto(out *HighAvailability) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HighAvailability.
func (in *HighAvailability) DeepCopy() *HighAvailability {
	if in == nil {
		return nil
	}
	out := new(HighAvailability)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Ingress) DeepCopyInto(out *Ingress) {
	*out = *in
	if in.Domains != nil {
		in, out := &in.Domains, &out.Domains
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.Controller.DeepCopyInto(&out.Controller)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Ingress.
func (in *Ingress) DeepCopy() *Ingress {
	if in == nil {
		return nil
	}
	out := new(Ingress)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeAPIServerConfig) DeepCopyInto(out *KubeAPIServerConfig) {
	*out = *in
	if in.KubeAPIServerConfig != nil {
		in, out := &in.KubeAPIServerConfig, &out.KubeAPIServerConfig
		*out = new(v1beta1.KubeAPIServerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.AuditWebhook != nil {
		in, out := &in.AuditWebhook, &out.AuditWebhook
		*out = new(AuditWebhook)
		(*in).DeepCopyInto(*out)
	}
	if in.Authentication != nil {
		in, out := &in.Authentication, &out.Authentication
		*out = new(Authentication)
		(*in).DeepCopyInto(*out)
	}
	if in.ResourcesToStoreInETCDEvents != nil {
		in, out := &in.ResourcesToStoreInETCDEvents, &out.ResourcesToStoreInETCDEvents
		*out = make([]GroupResource, len(*in))
		copy(*out, *in)
	}
	if in.SNI != nil {
		in, out := &in.SNI, &out.SNI
		*out = new(SNI)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeAPIServerConfig.
func (in *KubeAPIServerConfig) DeepCopy() *KubeAPIServerConfig {
	if in == nil {
		return nil
	}
	out := new(KubeAPIServerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeControllerManagerConfig) DeepCopyInto(out *KubeControllerManagerConfig) {
	*out = *in
	if in.KubeControllerManagerConfig != nil {
		in, out := &in.KubeControllerManagerConfig, &out.KubeControllerManagerConfig
		*out = new(v1beta1.KubeControllerManagerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.CertificateSigningDuration != nil {
		in, out := &in.CertificateSigningDuration, &out.CertificateSigningDuration
		*out = new(v1.Duration)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeControllerManagerConfig.
func (in *KubeControllerManagerConfig) DeepCopy() *KubeControllerManagerConfig {
	if in == nil {
		return nil
	}
	out := new(KubeControllerManagerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Kubernetes) DeepCopyInto(out *Kubernetes) {
	*out = *in
	if in.KubeAPIServer != nil {
		in, out := &in.KubeAPIServer, &out.KubeAPIServer
		*out = new(KubeAPIServerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.KubeControllerManager != nil {
		in, out := &in.KubeControllerManager, &out.KubeControllerManager
		*out = new(KubeControllerManagerConfig)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Kubernetes.
func (in *Kubernetes) DeepCopy() *Kubernetes {
	if in == nil {
		return nil
	}
	out := new(Kubernetes)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Maintenance) DeepCopyInto(out *Maintenance) {
	*out = *in
	out.TimeWindow = in.TimeWindow
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Maintenance.
func (in *Maintenance) DeepCopy() *Maintenance {
	if in == nil {
		return nil
	}
	out := new(Maintenance)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Networking) DeepCopyInto(out *Networking) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Networking.
func (in *Networking) DeepCopy() *Networking {
	if in == nil {
		return nil
	}
	out := new(Networking)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectQuotaConfiguration) DeepCopyInto(out *ProjectQuotaConfiguration) {
	*out = *in
	in.Config.DeepCopyInto(&out.Config)
	if in.ProjectSelector != nil {
		in, out := &in.ProjectSelector, &out.ProjectSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectQuotaConfiguration.
func (in *ProjectQuotaConfiguration) DeepCopy() *ProjectQuotaConfiguration {
	if in == nil {
		return nil
	}
	out := new(ProjectQuotaConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Provider) DeepCopyInto(out *Provider) {
	*out = *in
	if in.Zones != nil {
		in, out := &in.Zones, &out.Zones
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Provider.
func (in *Provider) DeepCopy() *Provider {
	if in == nil {
		return nil
	}
	out := new(Provider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceAdmissionConfiguration) DeepCopyInto(out *ResourceAdmissionConfiguration) {
	*out = *in
	if in.Limits != nil {
		in, out := &in.Limits, &out.Limits
		*out = make([]ResourceLimit, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.UnrestrictedSubjects != nil {
		in, out := &in.UnrestrictedSubjects, &out.UnrestrictedSubjects
		*out = make([]rbacv1.Subject, len(*in))
		copy(*out, *in)
	}
	if in.OperationMode != nil {
		in, out := &in.OperationMode, &out.OperationMode
		*out = new(ResourceAdmissionWebhookMode)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceAdmissionConfiguration.
func (in *ResourceAdmissionConfiguration) DeepCopy() *ResourceAdmissionConfiguration {
	if in == nil {
		return nil
	}
	out := new(ResourceAdmissionConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceLimit) DeepCopyInto(out *ResourceLimit) {
	*out = *in
	if in.APIGroups != nil {
		in, out := &in.APIGroups, &out.APIGroups
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.APIVersions != nil {
		in, out := &in.APIVersions, &out.APIVersions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.Size = in.Size.DeepCopy()
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceLimit.
func (in *ResourceLimit) DeepCopy() *ResourceLimit {
	if in == nil {
		return nil
	}
	out := new(ResourceLimit)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuntimeCluster) DeepCopyInto(out *RuntimeCluster) {
	*out = *in
	in.Ingress.DeepCopyInto(&out.Ingress)
	in.Networking.DeepCopyInto(&out.Networking)
	in.Provider.DeepCopyInto(&out.Provider)
	if in.Settings != nil {
		in, out := &in.Settings, &out.Settings
		*out = new(Settings)
		(*in).DeepCopyInto(*out)
	}
	if in.Volume != nil {
		in, out := &in.Volume, &out.Volume
		*out = new(Volume)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuntimeCluster.
func (in *RuntimeCluster) DeepCopy() *RuntimeCluster {
	if in == nil {
		return nil
	}
	out := new(RuntimeCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuntimeNetworking) DeepCopyInto(out *RuntimeNetworking) {
	*out = *in
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = new(string)
		**out = **in
	}
	if in.BlockCIDRs != nil {
		in, out := &in.BlockCIDRs, &out.BlockCIDRs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuntimeNetworking.
func (in *RuntimeNetworking) DeepCopy() *RuntimeNetworking {
	if in == nil {
		return nil
	}
	out := new(RuntimeNetworking)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SNI) DeepCopyInto(out *SNI) {
	*out = *in
	if in.DomainPatterns != nil {
		in, out := &in.DomainPatterns, &out.DomainPatterns
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SNI.
func (in *SNI) DeepCopy() *SNI {
	if in == nil {
		return nil
	}
	out := new(SNI)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SettingLoadBalancerServices) DeepCopyInto(out *SettingLoadBalancerServices) {
	*out = *in
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SettingLoadBalancerServices.
func (in *SettingLoadBalancerServices) DeepCopy() *SettingLoadBalancerServices {
	if in == nil {
		return nil
	}
	out := new(SettingLoadBalancerServices)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SettingTopologyAwareRouting) DeepCopyInto(out *SettingTopologyAwareRouting) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SettingTopologyAwareRouting.
func (in *SettingTopologyAwareRouting) DeepCopy() *SettingTopologyAwareRouting {
	if in == nil {
		return nil
	}
	out := new(SettingTopologyAwareRouting)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SettingVerticalPodAutoscaler) DeepCopyInto(out *SettingVerticalPodAutoscaler) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SettingVerticalPodAutoscaler.
func (in *SettingVerticalPodAutoscaler) DeepCopy() *SettingVerticalPodAutoscaler {
	if in == nil {
		return nil
	}
	out := new(SettingVerticalPodAutoscaler)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Settings) DeepCopyInto(out *Settings) {
	*out = *in
	if in.LoadBalancerServices != nil {
		in, out := &in.LoadBalancerServices, &out.LoadBalancerServices
		*out = new(SettingLoadBalancerServices)
		(*in).DeepCopyInto(*out)
	}
	if in.VerticalPodAutoscaler != nil {
		in, out := &in.VerticalPodAutoscaler, &out.VerticalPodAutoscaler
		*out = new(SettingVerticalPodAutoscaler)
		(*in).DeepCopyInto(*out)
	}
	if in.TopologyAwareRouting != nil {
		in, out := &in.TopologyAwareRouting, &out.TopologyAwareRouting
		*out = new(SettingTopologyAwareRouting)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Settings.
func (in *Settings) DeepCopy() *Settings {
	if in == nil {
		return nil
	}
	out := new(Settings)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Storage) DeepCopyInto(out *Storage) {
	*out = *in
	if in.Capacity != nil {
		in, out := &in.Capacity, &out.Capacity
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.ClassName != nil {
		in, out := &in.ClassName, &out.ClassName
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Storage.
func (in *Storage) DeepCopy() *Storage {
	if in == nil {
		return nil
	}
	out := new(Storage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualCluster) DeepCopyInto(out *VirtualCluster) {
	*out = *in
	if in.ControlPlane != nil {
		in, out := &in.ControlPlane, &out.ControlPlane
		*out = new(ControlPlane)
		(*in).DeepCopyInto(*out)
	}
	in.DNS.DeepCopyInto(&out.DNS)
	if in.ETCD != nil {
		in, out := &in.ETCD, &out.ETCD
		*out = new(ETCD)
		(*in).DeepCopyInto(*out)
	}
	in.Gardener.DeepCopyInto(&out.Gardener)
	in.Kubernetes.DeepCopyInto(&out.Kubernetes)
	out.Maintenance = in.Maintenance
	out.Networking = in.Networking
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualCluster.
func (in *VirtualCluster) DeepCopy() *VirtualCluster {
	if in == nil {
		return nil
	}
	out := new(VirtualCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Volume) DeepCopyInto(out *Volume) {
	*out = *in
	if in.MinimumSize != nil {
		in, out := &in.MinimumSize, &out.MinimumSize
		x := (*in).DeepCopy()
		*out = &x
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Volume.
func (in *Volume) DeepCopy() *Volume {
	if in == nil {
		return nil
	}
	out := new(Volume)
	in.DeepCopyInto(out)
	return out
}
