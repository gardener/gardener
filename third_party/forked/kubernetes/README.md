# Info

Forked from https://github.com/kubernetes/kubernetes/tree/v1.18.8 to support [Resource Quotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/)
for Gardener offered resources, e.g. group `core.gardener.cloud`. This local copy will be obsolete after vendoring Gardener to Kubernetes 1.20.x with [k8s.io/kubernetes#93537](https://github.com/kubernetes/kubernetes/pull/93537).

# Adjustments

## ./kubernetes/pkg/kubeapiserver/admission/initializer.go

- `WantsQuotaConfiguration` is kept for dependency reasons.

Diff
```
@@ -17,65 +17,14 @@ limitations under the License.
 package admission
 
 import (
-	"k8s.io/apimachinery/pkg/api/meta"
 	"k8s.io/apiserver/pkg/admission"
 	quota "k8s.io/kubernetes/pkg/quota/v1"
 )
 
 // TODO add a `WantsToRun` which takes a stopCh.  Might make it generic.
 
-// WantsCloudConfig defines a function which sets CloudConfig for admission plugins that need it.
-type WantsCloudConfig interface {
-	SetCloudConfig([]byte)
-}
-
-// WantsRESTMapper defines a function which sets RESTMapper for admission plugins that need it.
-type WantsRESTMapper interface {
-	SetRESTMapper(meta.RESTMapper)
-}
-
 // WantsQuotaConfiguration defines a function which sets quota configuration for admission plugins that need it.
 type WantsQuotaConfiguration interface {
 	SetQuotaConfiguration(quota.Configuration)
 	admission.InitializationValidator
 }
-
-// PluginInitializer is used for initialization of the Kubernetes specific admission plugins.
-type PluginInitializer struct {
-	cloudConfig        []byte
-	restMapper         meta.RESTMapper
-	quotaConfiguration quota.Configuration
-}
-
-var _ admission.PluginInitializer = &PluginInitializer{}
-
-// NewPluginInitializer constructs new instance of PluginInitializer
-// TODO: switch these parameters to use the builder pattern or just make them
-// all public, this construction method is pointless boilerplate.
-func NewPluginInitializer(
-	cloudConfig []byte,
-	restMapper meta.RESTMapper,
-	quotaConfiguration quota.Configuration,
-) *PluginInitializer {
-	return &PluginInitializer{
-		cloudConfig:        cloudConfig,
-		restMapper:         restMapper,
-		quotaConfiguration: quotaConfiguration,
-	}
-}
-
-// Initialize checks the initialization interfaces implemented by each plugin
-// and provide the appropriate initialization data
-func (i *PluginInitializer) Initialize(plugin admission.Interface) {
-	if wants, ok := plugin.(WantsCloudConfig); ok {
-		wants.SetCloudConfig(i.cloudConfig)
-	}
-
-	if wants, ok := plugin.(WantsRESTMapper); ok {
-		wants.SetRESTMapper(i.restMapper)
-	}
-
-	if wants, ok := plugin.(WantsQuotaConfiguration); ok {
-		wants.SetQuotaConfiguration(i.quotaConfiguration)
-	}
-}
```


