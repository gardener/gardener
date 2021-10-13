//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by deepcopy-gen. DO NOT EDIT.

package imports

import (
	json "encoding/json"

	apiv1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	v1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	configv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerAdmissionConfiguration) DeepCopyInto(out *APIServerAdmissionConfiguration) {
	*out = *in
	if in.EnableAdmissionPlugins != nil {
		in, out := &in.EnableAdmissionPlugins, &out.EnableAdmissionPlugins
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DisableAdmissionPlugins != nil {
		in, out := &in.DisableAdmissionPlugins, &out.DisableAdmissionPlugins
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Plugins != nil {
		in, out := &in.Plugins, &out.Plugins
		*out = make([]v1.AdmissionPluginConfiguration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ValidatingWebhook != nil {
		in, out := &in.ValidatingWebhook, &out.ValidatingWebhook
		*out = new(APIServerAdmissionWebhookCredentials)
		(*in).DeepCopyInto(*out)
	}
	if in.MutatingWebhook != nil {
		in, out := &in.MutatingWebhook, &out.MutatingWebhook
		*out = new(APIServerAdmissionWebhookCredentials)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerAdmissionConfiguration.
func (in *APIServerAdmissionConfiguration) DeepCopy() *APIServerAdmissionConfiguration {
	if in == nil {
		return nil
	}
	out := new(APIServerAdmissionConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerAdmissionWebhookCredentials) DeepCopyInto(out *APIServerAdmissionWebhookCredentials) {
	*out = *in
	if in.Kubeconfig != nil {
		in, out := &in.Kubeconfig, &out.Kubeconfig
		*out = new(v1alpha1.Target)
		(*in).DeepCopyInto(*out)
	}
	if in.TokenProjection != nil {
		in, out := &in.TokenProjection, &out.TokenProjection
		*out = new(APIServerAdmissionWebhookCredentialsTokenProjection)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerAdmissionWebhookCredentials.
func (in *APIServerAdmissionWebhookCredentials) DeepCopy() *APIServerAdmissionWebhookCredentials {
	if in == nil {
		return nil
	}
	out := new(APIServerAdmissionWebhookCredentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerAdmissionWebhookCredentialsTokenProjection) DeepCopyInto(out *APIServerAdmissionWebhookCredentialsTokenProjection) {
	*out = *in
	if in.Audience != nil {
		in, out := &in.Audience, &out.Audience
		*out = new(string)
		**out = **in
	}
	if in.ExpirationSeconds != nil {
		in, out := &in.ExpirationSeconds, &out.ExpirationSeconds
		*out = new(int32)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerAdmissionWebhookCredentialsTokenProjection.
func (in *APIServerAdmissionWebhookCredentialsTokenProjection) DeepCopy() *APIServerAdmissionWebhookCredentialsTokenProjection {
	if in == nil {
		return nil
	}
	out := new(APIServerAdmissionWebhookCredentialsTokenProjection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerAuditCommonBackendConfiguration) DeepCopyInto(out *APIServerAuditCommonBackendConfiguration) {
	*out = *in
	if in.BatchBufferSize != nil {
		in, out := &in.BatchBufferSize, &out.BatchBufferSize
		*out = new(int32)
		**out = **in
	}
	if in.BatchMaxSize != nil {
		in, out := &in.BatchMaxSize, &out.BatchMaxSize
		*out = new(int32)
		**out = **in
	}
	if in.BatchMaxWait != nil {
		in, out := &in.BatchMaxWait, &out.BatchMaxWait
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.BatchThrottleBurst != nil {
		in, out := &in.BatchThrottleBurst, &out.BatchThrottleBurst
		*out = new(int32)
		**out = **in
	}
	if in.BatchThrottleEnable != nil {
		in, out := &in.BatchThrottleEnable, &out.BatchThrottleEnable
		*out = new(bool)
		**out = **in
	}
	if in.BatchThrottleQPS != nil {
		in, out := &in.BatchThrottleQPS, &out.BatchThrottleQPS
		*out = new(float32)
		**out = **in
	}
	if in.Mode != nil {
		in, out := &in.Mode, &out.Mode
		*out = new(string)
		**out = **in
	}
	if in.TruncateEnabled != nil {
		in, out := &in.TruncateEnabled, &out.TruncateEnabled
		*out = new(bool)
		**out = **in
	}
	if in.TruncateMaxBatchSize != nil {
		in, out := &in.TruncateMaxBatchSize, &out.TruncateMaxBatchSize
		*out = new(int32)
		**out = **in
	}
	if in.TruncateMaxEventSize != nil {
		in, out := &in.TruncateMaxEventSize, &out.TruncateMaxEventSize
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

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerAuditCommonBackendConfiguration.
func (in *APIServerAuditCommonBackendConfiguration) DeepCopy() *APIServerAuditCommonBackendConfiguration {
	if in == nil {
		return nil
	}
	out := new(APIServerAuditCommonBackendConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerAuditConfiguration) DeepCopyInto(out *APIServerAuditConfiguration) {
	*out = *in
	if in.DynamicConfiguration != nil {
		in, out := &in.DynamicConfiguration, &out.DynamicConfiguration
		*out = new(bool)
		**out = **in
	}
	if in.Policy != nil {
		in, out := &in.Policy, &out.Policy
		*out = new(auditv1.Policy)
		(*in).DeepCopyInto(*out)
	}
	if in.Log != nil {
		in, out := &in.Log, &out.Log
		*out = new(APIServerAuditLogBackend)
		(*in).DeepCopyInto(*out)
	}
	if in.Webhook != nil {
		in, out := &in.Webhook, &out.Webhook
		*out = new(APIServerAuditWebhookBackend)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerAuditConfiguration.
func (in *APIServerAuditConfiguration) DeepCopy() *APIServerAuditConfiguration {
	if in == nil {
		return nil
	}
	out := new(APIServerAuditConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerAuditLogBackend) DeepCopyInto(out *APIServerAuditLogBackend) {
	*out = *in
	in.APIServerAuditCommonBackendConfiguration.DeepCopyInto(&out.APIServerAuditCommonBackendConfiguration)
	if in.Format != nil {
		in, out := &in.Format, &out.Format
		*out = new(string)
		**out = **in
	}
	if in.MaxAge != nil {
		in, out := &in.MaxAge, &out.MaxAge
		*out = new(int32)
		**out = **in
	}
	if in.MaxBackup != nil {
		in, out := &in.MaxBackup, &out.MaxBackup
		*out = new(int32)
		**out = **in
	}
	if in.MaxSize != nil {
		in, out := &in.MaxSize, &out.MaxSize
		*out = new(int32)
		**out = **in
	}
	if in.Path != nil {
		in, out := &in.Path, &out.Path
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerAuditLogBackend.
func (in *APIServerAuditLogBackend) DeepCopy() *APIServerAuditLogBackend {
	if in == nil {
		return nil
	}
	out := new(APIServerAuditLogBackend)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerAuditWebhookBackend) DeepCopyInto(out *APIServerAuditWebhookBackend) {
	*out = *in
	in.APIServerAuditCommonBackendConfiguration.DeepCopyInto(&out.APIServerAuditCommonBackendConfiguration)
	in.Kubeconfig.DeepCopyInto(&out.Kubeconfig)
	if in.InitialBackoff != nil {
		in, out := &in.InitialBackoff, &out.InitialBackoff
		*out = new(metav1.Duration)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerAuditWebhookBackend.
func (in *APIServerAuditWebhookBackend) DeepCopy() *APIServerAuditWebhookBackend {
	if in == nil {
		return nil
	}
	out := new(APIServerAuditWebhookBackend)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerComponentConfiguration) DeepCopyInto(out *APIServerComponentConfiguration) {
	*out = *in
	if in.ClusterIdentity != nil {
		in, out := &in.ClusterIdentity, &out.ClusterIdentity
		*out = new(string)
		**out = **in
	}
	if in.Encryption != nil {
		in, out := &in.Encryption, &out.Encryption
		*out = new(configv1.EncryptionConfiguration)
		(*in).DeepCopyInto(*out)
	}
	in.Etcd.DeepCopyInto(&out.Etcd)
	if in.CA != nil {
		in, out := &in.CA, &out.CA
		*out = new(CA)
		(*in).DeepCopyInto(*out)
	}
	if in.TLS != nil {
		in, out := &in.TLS, &out.TLS
		*out = new(TLSServer)
		(*in).DeepCopyInto(*out)
	}
	if in.FeatureGates != nil {
		in, out := &in.FeatureGates, &out.FeatureGates
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Admission != nil {
		in, out := &in.Admission, &out.Admission
		*out = new(APIServerAdmissionConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.GoAwayChance != nil {
		in, out := &in.GoAwayChance, &out.GoAwayChance
		*out = new(float32)
		**out = **in
	}
	if in.Http2MaxStreamsPerConnection != nil {
		in, out := &in.Http2MaxStreamsPerConnection, &out.Http2MaxStreamsPerConnection
		*out = new(int32)
		**out = **in
	}
	if in.ShutdownDelayDuration != nil {
		in, out := &in.ShutdownDelayDuration, &out.ShutdownDelayDuration
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.Requests != nil {
		in, out := &in.Requests, &out.Requests
		*out = new(APIServerRequests)
		(*in).DeepCopyInto(*out)
	}
	if in.WatchCacheSize != nil {
		in, out := &in.WatchCacheSize, &out.WatchCacheSize
		*out = new(APIServerWatchCacheConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Audit != nil {
		in, out := &in.Audit, &out.Audit
		*out = new(APIServerAuditConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerComponentConfiguration.
func (in *APIServerComponentConfiguration) DeepCopy() *APIServerComponentConfiguration {
	if in == nil {
		return nil
	}
	out := new(APIServerComponentConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerDeploymentConfiguration) DeepCopyInto(out *APIServerDeploymentConfiguration) {
	*out = *in
	in.CommonDeploymentConfiguration.DeepCopyInto(&out.CommonDeploymentConfiguration)
	if in.LivenessProbe != nil {
		in, out := &in.LivenessProbe, &out.LivenessProbe
		*out = new(corev1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.ReadinessProbe != nil {
		in, out := &in.ReadinessProbe, &out.ReadinessProbe
		*out = new(corev1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.MinReadySeconds != nil {
		in, out := &in.MinReadySeconds, &out.MinReadySeconds
		*out = new(int32)
		**out = **in
	}
	if in.Hvpa != nil {
		in, out := &in.Hvpa, &out.Hvpa
		*out = new(HVPAConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerDeploymentConfiguration.
func (in *APIServerDeploymentConfiguration) DeepCopy() *APIServerDeploymentConfiguration {
	if in == nil {
		return nil
	}
	out := new(APIServerDeploymentConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerEtcdConfiguration) DeepCopyInto(out *APIServerEtcdConfiguration) {
	*out = *in
	if in.CABundle != nil {
		in, out := &in.CABundle, &out.CABundle
		*out = new(string)
		**out = **in
	}
	if in.ClientCert != nil {
		in, out := &in.ClientCert, &out.ClientCert
		*out = new(string)
		**out = **in
	}
	if in.ClientKey != nil {
		in, out := &in.ClientKey, &out.ClientKey
		*out = new(string)
		**out = **in
	}
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(corev1.SecretReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerEtcdConfiguration.
func (in *APIServerEtcdConfiguration) DeepCopy() *APIServerEtcdConfiguration {
	if in == nil {
		return nil
	}
	out := new(APIServerEtcdConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerRequests) DeepCopyInto(out *APIServerRequests) {
	*out = *in
	if in.MaxNonMutatingInflight != nil {
		in, out := &in.MaxNonMutatingInflight, &out.MaxNonMutatingInflight
		*out = new(int)
		**out = **in
	}
	if in.MaxMutatingInflight != nil {
		in, out := &in.MaxMutatingInflight, &out.MaxMutatingInflight
		*out = new(int)
		**out = **in
	}
	if in.MinTimeout != nil {
		in, out := &in.MinTimeout, &out.MinTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(metav1.Duration)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerRequests.
func (in *APIServerRequests) DeepCopy() *APIServerRequests {
	if in == nil {
		return nil
	}
	out := new(APIServerRequests)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerWatchCacheConfiguration) DeepCopyInto(out *APIServerWatchCacheConfiguration) {
	*out = *in
	if in.DefaultSize != nil {
		in, out := &in.DefaultSize, &out.DefaultSize
		*out = new(int32)
		**out = **in
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]WatchCacheSizeResource, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerWatchCacheConfiguration.
func (in *APIServerWatchCacheConfiguration) DeepCopy() *APIServerWatchCacheConfiguration {
	if in == nil {
		return nil
	}
	out := new(APIServerWatchCacheConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AdmissionControllerComponentConfiguration) DeepCopyInto(out *AdmissionControllerComponentConfiguration) {
	*out = *in
	if in.CA != nil {
		in, out := &in.CA, &out.CA
		*out = new(CA)
		(*in).DeepCopyInto(*out)
	}
	if in.TLS != nil {
		in, out := &in.TLS, &out.TLS
		*out = new(TLSServer)
		(*in).DeepCopyInto(*out)
	}
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = new(Configuration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AdmissionControllerComponentConfiguration.
func (in *AdmissionControllerComponentConfiguration) DeepCopy() *AdmissionControllerComponentConfiguration {
	if in == nil {
		return nil
	}
	out := new(AdmissionControllerComponentConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Alerting) DeepCopyInto(out *Alerting) {
	*out = *in
	if in.Url != nil {
		in, out := &in.Url, &out.Url
		*out = new(string)
		**out = **in
	}
	if in.ToEmailAddress != nil {
		in, out := &in.ToEmailAddress, &out.ToEmailAddress
		*out = new(string)
		**out = **in
	}
	if in.FromEmailAddress != nil {
		in, out := &in.FromEmailAddress, &out.FromEmailAddress
		*out = new(string)
		**out = **in
	}
	if in.Smarthost != nil {
		in, out := &in.Smarthost, &out.Smarthost
		*out = new(string)
		**out = **in
	}
	if in.AuthUsername != nil {
		in, out := &in.AuthUsername, &out.AuthUsername
		*out = new(string)
		**out = **in
	}
	if in.AuthIdentity != nil {
		in, out := &in.AuthIdentity, &out.AuthIdentity
		*out = new(string)
		**out = **in
	}
	if in.AuthPassword != nil {
		in, out := &in.AuthPassword, &out.AuthPassword
		*out = new(string)
		**out = **in
	}
	if in.Username != nil {
		in, out := &in.Username, &out.Username
		*out = new(string)
		**out = **in
	}
	if in.Password != nil {
		in, out := &in.Password, &out.Password
		*out = new(string)
		**out = **in
	}
	if in.CaCert != nil {
		in, out := &in.CaCert, &out.CaCert
		*out = new(string)
		**out = **in
	}
	if in.TlsCert != nil {
		in, out := &in.TlsCert, &out.TlsCert
		*out = new(string)
		**out = **in
	}
	if in.TlsKey != nil {
		in, out := &in.TlsKey, &out.TlsKey
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Alerting.
func (in *Alerting) DeepCopy() *Alerting {
	if in == nil {
		return nil
	}
	out := new(Alerting)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CA) DeepCopyInto(out *CA) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(corev1.SecretReference)
		**out = **in
	}
	if in.Crt != nil {
		in, out := &in.Crt, &out.Crt
		*out = new(string)
		**out = **in
	}
	if in.Key != nil {
		in, out := &in.Key, &out.Key
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CA.
func (in *CA) DeepCopy() *CA {
	if in == nil {
		return nil
	}
	out := new(CA)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CertificateRotation) DeepCopyInto(out *CertificateRotation) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CertificateRotation.
func (in *CertificateRotation) DeepCopy() *CertificateRotation {
	if in == nil {
		return nil
	}
	out := new(CertificateRotation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CommonDeploymentConfiguration) DeepCopyInto(out *CommonDeploymentConfiguration) {
	*out = *in
	if in.ReplicaCount != nil {
		in, out := &in.ReplicaCount, &out.ReplicaCount
		*out = new(int32)
		**out = **in
	}
	if in.ServiceAccountName != nil {
		in, out := &in.ServiceAccountName, &out.ServiceAccountName
		*out = new(string)
		**out = **in
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(corev1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.PodLabels != nil {
		in, out := &in.PodLabels, &out.PodLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.PodAnnotations != nil {
		in, out := &in.PodAnnotations, &out.PodAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.VPA != nil {
		in, out := &in.VPA, &out.VPA
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CommonDeploymentConfiguration.
func (in *CommonDeploymentConfiguration) DeepCopy() *CommonDeploymentConfiguration {
	if in == nil {
		return nil
	}
	out := new(CommonDeploymentConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Configuration) DeepCopyInto(out *Configuration) {
	*out = *in
	if in.ComponentConfiguration != nil {
		out.ComponentConfiguration = in.ComponentConfiguration.DeepCopyObject()
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Configuration.
func (in *Configuration) DeepCopy() *Configuration {
	if in == nil {
		return nil
	}
	out := new(Configuration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerManagerComponentConfiguration) DeepCopyInto(out *ControllerManagerComponentConfiguration) {
	*out = *in
	if in.TLS != nil {
		in, out := &in.TLS, &out.TLS
		*out = new(TLSServer)
		(*in).DeepCopyInto(*out)
	}
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = new(Configuration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerManagerComponentConfiguration.
func (in *ControllerManagerComponentConfiguration) DeepCopy() *ControllerManagerComponentConfiguration {
	if in == nil {
		return nil
	}
	out := new(ControllerManagerComponentConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerManagerDeploymentConfiguration) DeepCopyInto(out *ControllerManagerDeploymentConfiguration) {
	*out = *in
	if in.CommonDeploymentConfiguration != nil {
		in, out := &in.CommonDeploymentConfiguration, &out.CommonDeploymentConfiguration
		*out = new(CommonDeploymentConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.AdditionalVolumes != nil {
		in, out := &in.AdditionalVolumes, &out.AdditionalVolumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AdditionalVolumeMounts != nil {
		in, out := &in.AdditionalVolumeMounts, &out.AdditionalVolumeMounts
		*out = make([]corev1.VolumeMount, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerManagerDeploymentConfiguration.
func (in *ControllerManagerDeploymentConfiguration) DeepCopy() *ControllerManagerDeploymentConfiguration {
	if in == nil {
		return nil
	}
	out := new(ControllerManagerDeploymentConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DNS) DeepCopyInto(out *DNS) {
	*out = *in
	if in.Credentials != nil {
		in, out := &in.Credentials, &out.Credentials
		*out = make(json.RawMessage, len(*in))
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
func (in *GardenerAPIServer) DeepCopyInto(out *GardenerAPIServer) {
	*out = *in
	if in.DeploymentConfiguration != nil {
		in, out := &in.DeploymentConfiguration, &out.DeploymentConfiguration
		*out = new(APIServerDeploymentConfiguration)
		(*in).DeepCopyInto(*out)
	}
	in.ComponentConfiguration.DeepCopyInto(&out.ComponentConfiguration)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenerAPIServer.
func (in *GardenerAPIServer) DeepCopy() *GardenerAPIServer {
	if in == nil {
		return nil
	}
	out := new(GardenerAPIServer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenerAdmissionController) DeepCopyInto(out *GardenerAdmissionController) {
	*out = *in
	if in.SeedRestriction != nil {
		in, out := &in.SeedRestriction, &out.SeedRestriction
		*out = new(SeedRestriction)
		**out = **in
	}
	if in.DeploymentConfiguration != nil {
		in, out := &in.DeploymentConfiguration, &out.DeploymentConfiguration
		*out = new(CommonDeploymentConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.ComponentConfiguration != nil {
		in, out := &in.ComponentConfiguration, &out.ComponentConfiguration
		*out = new(AdmissionControllerComponentConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenerAdmissionController.
func (in *GardenerAdmissionController) DeepCopy() *GardenerAdmissionController {
	if in == nil {
		return nil
	}
	out := new(GardenerAdmissionController)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenerControllerManager) DeepCopyInto(out *GardenerControllerManager) {
	*out = *in
	if in.DeploymentConfiguration != nil {
		in, out := &in.DeploymentConfiguration, &out.DeploymentConfiguration
		*out = new(ControllerManagerDeploymentConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.ComponentConfiguration != nil {
		in, out := &in.ComponentConfiguration, &out.ComponentConfiguration
		*out = new(ControllerManagerComponentConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenerControllerManager.
func (in *GardenerControllerManager) DeepCopy() *GardenerControllerManager {
	if in == nil {
		return nil
	}
	out := new(GardenerControllerManager)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenerScheduler) DeepCopyInto(out *GardenerScheduler) {
	*out = *in
	if in.DeploymentConfiguration != nil {
		in, out := &in.DeploymentConfiguration, &out.DeploymentConfiguration
		*out = new(CommonDeploymentConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.ComponentConfiguration != nil {
		in, out := &in.ComponentConfiguration, &out.ComponentConfiguration
		*out = new(SchedulerComponentConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenerScheduler.
func (in *GardenerScheduler) DeepCopy() *GardenerScheduler {
	if in == nil {
		return nil
	}
	out := new(GardenerScheduler)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HVPAConfiguration) DeepCopyInto(out *HVPAConfiguration) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	if in.MaintenanceTimeWindow != nil {
		in, out := &in.MaintenanceTimeWindow, &out.MaintenanceTimeWindow
		*out = new(apiv1alpha1.MaintenanceTimeWindow)
		**out = **in
	}
	if in.HVPAConfigurationHPA != nil {
		in, out := &in.HVPAConfigurationHPA, &out.HVPAConfigurationHPA
		*out = new(HVPAConfigurationHPA)
		(*in).DeepCopyInto(*out)
	}
	if in.HVPAConfigurationVPA != nil {
		in, out := &in.HVPAConfigurationVPA, &out.HVPAConfigurationVPA
		*out = new(HVPAConfigurationVPA)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HVPAConfiguration.
func (in *HVPAConfiguration) DeepCopy() *HVPAConfiguration {
	if in == nil {
		return nil
	}
	out := new(HVPAConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HVPAConfigurationHPA) DeepCopyInto(out *HVPAConfigurationHPA) {
	*out = *in
	if in.MinReplicas != nil {
		in, out := &in.MinReplicas, &out.MinReplicas
		*out = new(int32)
		**out = **in
	}
	if in.MaxReplicas != nil {
		in, out := &in.MaxReplicas, &out.MaxReplicas
		*out = new(int32)
		**out = **in
	}
	if in.TargetAverageUtilizationCpu != nil {
		in, out := &in.TargetAverageUtilizationCpu, &out.TargetAverageUtilizationCpu
		*out = new(int32)
		**out = **in
	}
	if in.TargetAverageUtilizationMemory != nil {
		in, out := &in.TargetAverageUtilizationMemory, &out.TargetAverageUtilizationMemory
		*out = new(int32)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HVPAConfigurationHPA.
func (in *HVPAConfigurationHPA) DeepCopy() *HVPAConfigurationHPA {
	if in == nil {
		return nil
	}
	out := new(HVPAConfigurationHPA)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HVPAConfigurationVPA) DeepCopyInto(out *HVPAConfigurationVPA) {
	*out = *in
	if in.ScaleUpMode != nil {
		in, out := &in.ScaleUpMode, &out.ScaleUpMode
		*out = new(string)
		**out = **in
	}
	if in.ScaleDownMode != nil {
		in, out := &in.ScaleDownMode, &out.ScaleDownMode
		*out = new(string)
		**out = **in
	}
	if in.ScaleUpStabilization != nil {
		in, out := &in.ScaleUpStabilization, &out.ScaleUpStabilization
		*out = new(apiv1alpha1.ScaleType)
		(*in).DeepCopyInto(*out)
	}
	if in.ScaleDownStabilization != nil {
		in, out := &in.ScaleDownStabilization, &out.ScaleDownStabilization
		*out = new(apiv1alpha1.ScaleType)
		(*in).DeepCopyInto(*out)
	}
	if in.LimitsRequestsGapScaleParams != nil {
		in, out := &in.LimitsRequestsGapScaleParams, &out.LimitsRequestsGapScaleParams
		*out = new(apiv1alpha1.ScaleParams)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HVPAConfigurationVPA.
func (in *HVPAConfigurationVPA) DeepCopy() *HVPAConfigurationVPA {
	if in == nil {
		return nil
	}
	out := new(HVPAConfigurationVPA)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Imports) DeepCopyInto(out *Imports) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Identity != nil {
		in, out := &in.Identity, &out.Identity
		*out = new(string)
		**out = **in
	}
	in.RuntimeCluster.DeepCopyInto(&out.RuntimeCluster)
	if in.VirtualGarden != nil {
		in, out := &in.VirtualGarden, &out.VirtualGarden
		*out = new(VirtualGarden)
		(*in).DeepCopyInto(*out)
	}
	in.InternalDomain.DeepCopyInto(&out.InternalDomain)
	if in.DefaultDomains != nil {
		in, out := &in.DefaultDomains, &out.DefaultDomains
		*out = make([]DNS, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Alerting != nil {
		in, out := &in.Alerting, &out.Alerting
		*out = make([]Alerting, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.OpenVPNDiffieHellmanKey != nil {
		in, out := &in.OpenVPNDiffieHellmanKey, &out.OpenVPNDiffieHellmanKey
		*out = new(string)
		**out = **in
	}
	in.GardenerAPIServer.DeepCopyInto(&out.GardenerAPIServer)
	if in.GardenerControllerManager != nil {
		in, out := &in.GardenerControllerManager, &out.GardenerControllerManager
		*out = new(GardenerControllerManager)
		(*in).DeepCopyInto(*out)
	}
	if in.GardenerScheduler != nil {
		in, out := &in.GardenerScheduler, &out.GardenerScheduler
		*out = new(GardenerScheduler)
		(*in).DeepCopyInto(*out)
	}
	if in.GardenerAdmissionController != nil {
		in, out := &in.GardenerAdmissionController, &out.GardenerAdmissionController
		*out = new(GardenerAdmissionController)
		(*in).DeepCopyInto(*out)
	}
	if in.Rbac != nil {
		in, out := &in.Rbac, &out.Rbac
		*out = new(Rbac)
		(*in).DeepCopyInto(*out)
	}
	if in.CertificateRotation != nil {
		in, out := &in.CertificateRotation, &out.CertificateRotation
		*out = new(CertificateRotation)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Imports.
func (in *Imports) DeepCopy() *Imports {
	if in == nil {
		return nil
	}
	out := new(Imports)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Imports) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Rbac) DeepCopyInto(out *Rbac) {
	*out = *in
	if in.SeedAuthorizer != nil {
		in, out := &in.SeedAuthorizer, &out.SeedAuthorizer
		*out = new(SeedAuthorizer)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Rbac.
func (in *Rbac) DeepCopy() *Rbac {
	if in == nil {
		return nil
	}
	out := new(Rbac)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchedulerComponentConfiguration) DeepCopyInto(out *SchedulerComponentConfiguration) {
	*out = *in
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = new(Configuration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchedulerComponentConfiguration.
func (in *SchedulerComponentConfiguration) DeepCopy() *SchedulerComponentConfiguration {
	if in == nil {
		return nil
	}
	out := new(SchedulerComponentConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SeedAuthorizer) DeepCopyInto(out *SeedAuthorizer) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SeedAuthorizer.
func (in *SeedAuthorizer) DeepCopy() *SeedAuthorizer {
	if in == nil {
		return nil
	}
	out := new(SeedAuthorizer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SeedRestriction) DeepCopyInto(out *SeedRestriction) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SeedRestriction.
func (in *SeedRestriction) DeepCopy() *SeedRestriction {
	if in == nil {
		return nil
	}
	out := new(SeedRestriction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TLSServer) DeepCopyInto(out *TLSServer) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(corev1.SecretReference)
		**out = **in
	}
	if in.Crt != nil {
		in, out := &in.Crt, &out.Crt
		*out = new(string)
		**out = **in
	}
	if in.Key != nil {
		in, out := &in.Key, &out.Key
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TLSServer.
func (in *TLSServer) DeepCopy() *TLSServer {
	if in == nil {
		return nil
	}
	out := new(TLSServer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualGarden) DeepCopyInto(out *VirtualGarden) {
	*out = *in
	if in.Kubeconfig != nil {
		in, out := &in.Kubeconfig, &out.Kubeconfig
		*out = new(v1alpha1.Target)
		(*in).DeepCopyInto(*out)
	}
	if in.ClusterIP != nil {
		in, out := &in.ClusterIP, &out.ClusterIP
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualGarden.
func (in *VirtualGarden) DeepCopy() *VirtualGarden {
	if in == nil {
		return nil
	}
	out := new(VirtualGarden)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WatchCacheSizeResource) DeepCopyInto(out *WatchCacheSizeResource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WatchCacheSizeResource.
func (in *WatchCacheSizeResource) DeepCopy() *WatchCacheSizeResource {
	if in == nil {
		return nil
	}
	out := new(WatchCacheSizeResource)
	in.DeepCopyInto(out)
	return out
}
