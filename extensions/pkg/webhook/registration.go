// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
)

const (
	// NamePrefix is the prefix used for {Valida,Muta}tingWebhookConfigurations of extensions.
	NamePrefix = "gardener-extension-"
	// NameSuffixShoot is the suffix used for {Valida,Muta}tingWebhookConfigurations of extensions targeting a shoot.
	NameSuffixShoot = "-shoot"
	// ModeService is a constant for the webhook mode indicating that the controller is running inside of the Kubernetes cluster it
	// is serving.
	ModeService = "service"
	// ModeURL is a constant for the webhook mode indicating that the controller is running outside of the Kubernetes cluster it
	// is serving. If this is set then a URL is required for configuration.
	ModeURL = "url"
	// ModeURLWithServiceName is a constant for the webhook mode indicating that the controller is running outside of the Kubernetes cluster it
	// is serving but in the same cluster like the kube-apiserver. If this is set then a URL is required for configuration.
	ModeURLWithServiceName = "url-service"
)

// PrefixedName does not prefix the component name if it starts with "gardener-". Otherwise, it prefixes it with
// "gardener-extension-".
func PrefixedName(componentName string) string {
	if !strings.HasPrefix(componentName, "gardener-") {
		return NamePrefix + componentName
	}
	return componentName
}

// Configs contains mutating and validating webhook configurations.
type Configs struct {
	MutatingWebhookConfig   *admissionregistrationv1.MutatingWebhookConfiguration
	ValidatingWebhookConfig *admissionregistrationv1.ValidatingWebhookConfiguration
}

// GetWebhookConfigs returns a slice of webhook configurations.
func (c *Configs) GetWebhookConfigs() []client.Object {
	configs := make([]client.Object, 0, 2)
	if c.MutatingWebhookConfig != nil {
		configs = append(configs, c.MutatingWebhookConfig)
	}
	if c.ValidatingWebhookConfig != nil {
		configs = append(configs, c.ValidatingWebhookConfig)
	}
	return configs
}

// DeepCopy returns a deep copy of the 'Configs' object.
func (c *Configs) DeepCopy() *Configs {
	deepCopy := Configs{}
	if c.MutatingWebhookConfig != nil {
		deepCopy.MutatingWebhookConfig = c.MutatingWebhookConfig.DeepCopy()
	}
	if c.ValidatingWebhookConfig != nil {
		deepCopy.ValidatingWebhookConfig = c.ValidatingWebhookConfig.DeepCopy()
	}
	return &deepCopy
}

// HasWebhookConfig returns true if 'Configs' contains at least one webhook configuration.
func (c *Configs) HasWebhookConfig() bool {
	return c.MutatingWebhookConfig != nil || c.ValidatingWebhookConfig != nil
}

// BuildWebhookConfigs builds webhook.Configs for seed and shoot from the given webhooks slice.
func BuildWebhookConfigs(
	webhooks []*Webhook,
	c client.Client,
	namespace, providerName string,
	servicePort int,
	mode, url string,
	caBundle []byte,
) (
	seedWebhookConfigs Configs,
	shootWebhookConfigs Configs,
	err error,
) {
	var (
		exact       = admissionregistrationv1.Exact
		sideEffects = admissionregistrationv1.SideEffectClassNone
		shootMode   = ModeURLWithServiceName
	)

	if mode == ModeURL {
		shootMode = ModeURL
	}

	for _, webhook := range webhooks {
		var (
			name  = NamePrefix + providerName
			rules []admissionregistrationv1.RuleWithOperations
		)

		for _, t := range webhook.Types {
			rule, err := buildRule(c, t)
			if err != nil {
				return seedWebhookConfigs, shootWebhookConfigs, err
			}
			rules = append(rules, *rule)
		}

		switch webhook.Target {
		case TargetSeed:
			// if all webhooks for one target are removed in a new version, extensions need to explicitly delete the respective
			// webhook config
			createAndAddToWebhookConfig(
				&seedWebhookConfigs,
				name,
				*webhook,
				providerName,
				rules,
				getFailurePolicy(admissionregistrationv1.Fail, webhook.FailurePolicy),
				&exact,
				BuildClientConfigFor(webhook.Path, namespace, providerName, servicePort, mode, url, caBundle),
				&sideEffects,
			)

		case TargetShoot:
			createAndAddToWebhookConfig(
				&shootWebhookConfigs,
				name+NameSuffixShoot,
				*webhook,
				providerName,
				rules,
				getFailurePolicy(admissionregistrationv1.Ignore, webhook.FailurePolicy),
				&exact,
				BuildClientConfigFor(webhook.Path, namespace, providerName, servicePort, shootMode, url, caBundle),
				&sideEffects,
			)
		default:
			return seedWebhookConfigs, shootWebhookConfigs, fmt.Errorf("invalid webhook target: %s", webhook.Target)
		}
	}

	return seedWebhookConfigs, shootWebhookConfigs, nil
}

// ReconcileSeedWebhookConfig reconciles the given webhook config in the seed cluster.
// If a CA bundle is given, it is injected it into all desired webhooks. If not, the CA bundle from the webhook config
// on the cluster (if any) is kept.
func ReconcileSeedWebhookConfig(ctx context.Context, c client.Client, webhookConfig client.Object, ownerNamespace string, caBundle []byte) error {
	var ownerReference *metav1.OwnerReference
	if len(ownerNamespace) > 0 {
		ns := &corev1.Namespace{}
		if err := c.Get(ctx, client.ObjectKey{Name: ownerNamespace}, ns); err != nil {
			return err
		}
		ownerReference = metav1.NewControllerRef(ns, corev1.SchemeGroupVersion.WithKind("Namespace"))
		ownerReference.BlockOwnerDeletion = ptr.To(false)
	}

	desiredWebhookConfig := webhookConfig.DeepCopyObject().(client.Object)

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c, webhookConfig, func() error {
		if ownerReference != nil {
			webhookConfig.SetOwnerReferences([]metav1.OwnerReference{*ownerReference})
		}

		if len(caBundle) == 0 {
			var err error
			// we can safely assume, that the CA bundles in all webhooks are the same, as we manage it ourselves
			caBundle, err = GetCABundleFromWebhookConfig(webhookConfig)
			if err != nil {
				return err
			}
		}

		if err := InjectCABundleIntoWebhookConfig(desiredWebhookConfig, caBundle); err != nil {
			return err
		}
		return OverwriteWebhooks(webhookConfig, desiredWebhookConfig)
	}); err != nil {
		return fmt.Errorf("error reconciling seed webhook config: %w", err)
	}

	return nil
}

// OverwriteWebhooks sets current.Webhooks to desired.Webhooks for all kinds and version of webhook configs.
func OverwriteWebhooks(current, desired client.Object) error {
	switch config := current.(type) {
	case *admissionregistrationv1.MutatingWebhookConfiguration:
		d := desired.(*admissionregistrationv1.MutatingWebhookConfiguration)
		config.Webhooks = d.DeepCopy().Webhooks
	case *admissionregistrationv1.ValidatingWebhookConfiguration:
		d := desired.(*admissionregistrationv1.ValidatingWebhookConfiguration)
		config.Webhooks = d.DeepCopy().Webhooks
	case *admissionregistrationv1beta1.MutatingWebhookConfiguration:
		d := desired.(*admissionregistrationv1beta1.MutatingWebhookConfiguration)
		config.Webhooks = d.DeepCopy().Webhooks
	case *admissionregistrationv1beta1.ValidatingWebhookConfiguration:
		d := desired.(*admissionregistrationv1beta1.ValidatingWebhookConfiguration)
		config.Webhooks = d.DeepCopy().Webhooks
	default:
		return fmt.Errorf("unexpected webhook config type: %T", current)
	}

	return nil
}

// GetCABundleFromWebhookConfig finds the first non-empty Webhooks[0].ClientConfig.CABundle from the given webhook config.
func GetCABundleFromWebhookConfig(obj client.Object) ([]byte, error) {
	switch config := obj.(type) {
	case *admissionregistrationv1.MutatingWebhookConfiguration:
		for _, webhook := range config.Webhooks {
			if caBundle := webhook.ClientConfig.CABundle; len(caBundle) > 0 {
				return caBundle, nil
			}
		}
	case *admissionregistrationv1.ValidatingWebhookConfiguration:
		for _, webhook := range config.Webhooks {
			if caBundle := webhook.ClientConfig.CABundle; len(caBundle) > 0 {
				return caBundle, nil
			}
		}
	case *admissionregistrationv1beta1.MutatingWebhookConfiguration:
		for _, w := range config.Webhooks {
			if caBundle := w.ClientConfig.CABundle; len(caBundle) > 0 {
				return caBundle, nil
			}
		}
	case *admissionregistrationv1beta1.ValidatingWebhookConfiguration:
		for _, webhook := range config.Webhooks {
			if caBundle := webhook.ClientConfig.CABundle; len(caBundle) > 0 {
				return caBundle, nil
			}
		}
	default:
		return nil, fmt.Errorf("unexpected webhook config type: %T", obj)
	}

	return nil, nil
}

// InjectCABundleIntoWebhookConfig sets the given CA bundle in all webhook client config in the given webhook config.
func InjectCABundleIntoWebhookConfig(obj client.Object, caBundle []byte) error {
	switch config := obj.(type) {
	case *admissionregistrationv1.MutatingWebhookConfiguration:
		for i, w := range config.Webhooks {
			w.ClientConfig.CABundle = caBundle
			config.Webhooks[i] = w
		}
	case *admissionregistrationv1.ValidatingWebhookConfiguration:
		for i, w := range config.Webhooks {
			w.ClientConfig.CABundle = caBundle
			config.Webhooks[i] = w
		}
	case *admissionregistrationv1beta1.MutatingWebhookConfiguration:
		for i, w := range config.Webhooks {
			w.ClientConfig.CABundle = caBundle
			config.Webhooks[i] = w
		}
	case *admissionregistrationv1beta1.ValidatingWebhookConfiguration:
		for i, w := range config.Webhooks {
			w.ClientConfig.CABundle = caBundle
			config.Webhooks[i] = w
		}
	default:
		return fmt.Errorf("unexpected webhook config type: %T", obj)
	}

	return nil
}

func getFailurePolicy(def admissionregistrationv1.FailurePolicyType, overwrite *admissionregistrationv1.FailurePolicyType) *admissionregistrationv1.FailurePolicyType {
	if overwrite != nil {
		return overwrite
	}
	return &def
}

// buildRule creates and returns a RuleWithOperations for the given object type.
func buildRule(c client.Client, t Type) (*admissionregistrationv1.RuleWithOperations, error) {
	// Get GVK from the type
	gvk, err := apiutil.GVKForObject(t.Obj, c.Scheme())
	if err != nil {
		return nil, fmt.Errorf("could not get GroupVersionKind from object %v: %w", t.Obj, err)
	}

	// Get REST mapping from GVK. Don't specify a version to retrieve a mapping since this fails for '__internal' versions.
	mapping, err := c.RESTMapper().RESTMapping(gvk.GroupKind())
	if err != nil {
		return nil, fmt.Errorf("could not get REST mapping from GroupVersionKind '%s': %w", gvk.String(), err)
	}

	apiVersions := gvk.Version
	// The internal API version ('__internal') cannot be considered in the webhook rule, take all versions instead.
	if apiVersions == runtime.APIVersionInternal {
		apiVersions = "*"
	}

	resource := mapping.Resource.Resource
	if t.Subresource != nil {
		resource += fmt.Sprintf("/%s", *t.Subresource)
	}

	// Create and return RuleWithOperations
	return &admissionregistrationv1.RuleWithOperations{
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
		},
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{gvk.Group},
			APIVersions: []string{apiVersions},
			Resources:   []string{resource},
		},
	}, nil
}

// BuildClientConfigFor builds the client config for a webhook.
func BuildClientConfigFor(webhookPath string, namespace, componentName string, servicePort int, mode, url string, caBundle []byte) admissionregistrationv1.WebhookClientConfig {
	var (
		path         = webhookPath
		clientConfig = admissionregistrationv1.WebhookClientConfig{
			// can be empty if injected later on
			CABundle: caBundle,
		}
	)

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	switch mode {
	case ModeURL:
		clientConfig.URL = ptr.To(fmt.Sprintf("https://%s%s", url, path))
	case ModeURLWithServiceName:
		clientConfig.URL = ptr.To(fmt.Sprintf("https://%s.%s:%d%s", PrefixedName(componentName), namespace, servicePort, path))
	case ModeService:
		clientConfig.Service = &admissionregistrationv1.ServiceReference{
			Namespace: namespace,
			Name:      PrefixedName(componentName),
			Path:      &path,
		}
	}

	return clientConfig
}

func createAndAddToWebhookConfig(
	webhookConfigs *Configs,
	name string,
	webhook Webhook,
	providerName string,
	rules []admissionregistrationv1.RuleWithOperations,
	failurePolicy *admissionregistrationv1.FailurePolicyType,
	matchPolicy *admissionregistrationv1.MatchPolicyType,
	clientConfig admissionregistrationv1.WebhookClientConfig,
	sideEffects *admissionregistrationv1.SideEffectClass,
) {
	// Create a validating or mutating webhook configuration based on the webhooks action. If the action is not set or
	// unknown fall back to mutating webhook since this is the safest option to pick.
	switch webhook.Action {
	case ActionValidating:
		if webhookConfigs.ValidatingWebhookConfig == nil {
			webhookConfigs.ValidatingWebhookConfig = &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: InitialWebhookConfig(name),
			}
		}
		webhookToRegister := admissionregistrationv1.ValidatingWebhook{
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			Name:                    fmt.Sprintf("%s.%s.extensions.gardener.cloud", webhook.Name, strings.TrimPrefix(providerName, "provider-")),
			NamespaceSelector:       webhook.NamespaceSelector,
			ObjectSelector:          webhook.ObjectSelector,
			Rules:                   rules,
			SideEffects:             sideEffects,
			TimeoutSeconds:          ptr.To[int32](10),
		}

		if webhook.TimeoutSeconds != nil {
			webhookToRegister.TimeoutSeconds = webhook.TimeoutSeconds
		}

		webhookToRegister.FailurePolicy = failurePolicy
		webhookToRegister.MatchPolicy = matchPolicy
		webhookToRegister.ClientConfig = clientConfig
		webhookConfigs.ValidatingWebhookConfig.Webhooks = append(webhookConfigs.ValidatingWebhookConfig.Webhooks, webhookToRegister)
	default:
		if webhookConfigs.MutatingWebhookConfig == nil {
			webhookConfigs.MutatingWebhookConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: InitialWebhookConfig(name),
			}
		}

		webhookToRegister := admissionregistrationv1.MutatingWebhook{
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			Name:                    fmt.Sprintf("%s.%s.extensions.gardener.cloud", webhook.Name, strings.TrimPrefix(providerName, "provider-")),
			NamespaceSelector:       webhook.NamespaceSelector,
			ObjectSelector:          webhook.ObjectSelector,
			Rules:                   rules,
			SideEffects:             sideEffects,
			TimeoutSeconds:          ptr.To[int32](10),
		}

		if webhook.TimeoutSeconds != nil {
			webhookToRegister.TimeoutSeconds = webhook.TimeoutSeconds
		}

		webhookToRegister.FailurePolicy = failurePolicy
		webhookToRegister.MatchPolicy = matchPolicy
		webhookToRegister.ClientConfig = clientConfig
		webhookConfigs.MutatingWebhookConfig.Webhooks = append(webhookConfigs.MutatingWebhookConfig.Webhooks, webhookToRegister)
	}
}

// InitialWebhookConfig returns the initial object meta for a webhook configuration.
func InitialWebhookConfig(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:   name,
		Labels: map[string]string{v1beta1constants.LabelExcludeWebhookFromRemediation: "true"},
	}
}
