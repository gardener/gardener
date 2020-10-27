# Info

Forked from https://github.com/kubernetes/kubernetes/tree/v1.18.8 to support [Resource Quotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/)
for Gardener offered resources, e.g. group `core.gardener.cloud`. This local copy will be obsolete after vendoring Gardener to Kubernetes 1.20.x with [k8s.io/kubernetes#93537](https://github.com/kubernetes/kubernetes/pull/93537).

# Adjustments

## ./kubernetes/pkg/kubeapiserver/admission/initializer.go

`WantsQuotaConfiguration` interface is kept for dependency reasons.

```
@@ -17,65 +17,14 @@ limitations under the License.
 package admission
 
 import (
+   quota "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1"
-	"k8s.io/apimachinery/pkg/api/meta"
 	"k8s.io/apiserver/pkg/admission"
-	quota "k8s.io/kubernetes/pkg/quota/v1"
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
 
## Import paths
``` 
diff --git a/third_party/forked/kubernetes/pkg/kubeapiserver/admission/initializer.go b/third_party/forked/kubernetes/pkg/kubeapiserver/admission/initializer.go
index e91db20f0..b793f5225 100644
--- a/third_party/forked/kubernetes/pkg/kubeapiserver/admission/initializer.go
+++ b/third_party/forked/kubernetes/pkg/kubeapiserver/admission/initializer.go
@@ -17,8 +17,8 @@ limitations under the License.
 package admission
 
 import (
+	quota "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1"
 	"k8s.io/apiserver/pkg/admission"
-	quota "k8s.io/kubernetes/pkg/quota/v1"
 )
 
 // TODO add a `WantsToRun` which takes a stopCh.  Might make it generic.
diff --git a/third_party/forked/kubernetes/pkg/quota/v1/generic/configuration.go b/third_party/forked/kubernetes/pkg/quota/v1/generic/configuration.go
index 1a1acc441..637c58892 100644
--- a/third_party/forked/kubernetes/pkg/quota/v1/generic/configuration.go
+++ b/third_party/forked/kubernetes/pkg/quota/v1/generic/configuration.go
@@ -18,7 +18,7 @@ package generic
 
 import (
 	"k8s.io/apimachinery/pkg/runtime/schema"
-	quota "k8s.io/kubernetes/pkg/quota/v1"
+	quota "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1"
 )
 
 // implements a basic configuration
diff --git a/third_party/forked/kubernetes/pkg/quota/v1/generic/evaluator.go b/third_party/forked/kubernetes/pkg/quota/v1/generic/evaluator.go
index f49e2decd..7145e2c08 100644
--- a/third_party/forked/kubernetes/pkg/quota/v1/generic/evaluator.go
+++ b/third_party/forked/kubernetes/pkg/quota/v1/generic/evaluator.go
@@ -28,7 +28,7 @@ import (
 	"k8s.io/apiserver/pkg/admission"
 	"k8s.io/client-go/informers"
 	"k8s.io/client-go/tools/cache"
-	quota "k8s.io/kubernetes/pkg/quota/v1"
+	quota "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1"
 )
 
 // InformerForResourceFunc knows how to provision an informer
diff --git a/third_party/forked/kubernetes/pkg/quota/v1/generic/registry.go b/third_party/forked/kubernetes/pkg/quota/v1/generic/registry.go
index 10404a3f2..f96c2bae7 100644
--- a/third_party/forked/kubernetes/pkg/quota/v1/generic/registry.go
+++ b/third_party/forked/kubernetes/pkg/quota/v1/generic/registry.go
@@ -20,7 +20,7 @@ import (
 	"sync"
 
 	"k8s.io/apimachinery/pkg/runtime/schema"
-	quota "k8s.io/kubernetes/pkg/quota/v1"
+	quota "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1"
 )
 
 // implements a basic registry
diff --git a/third_party/forked/kubernetes/pkg/quota/v1/install/registry.go b/third_party/forked/kubernetes/pkg/quota/v1/install/registry.go
index 239fff3c0..f5420cf35 100644
--- a/third_party/forked/kubernetes/pkg/quota/v1/install/registry.go
+++ b/third_party/forked/kubernetes/pkg/quota/v1/install/registry.go
@@ -18,8 +18,8 @@ package install
 
 import (
 	"k8s.io/apimachinery/pkg/runtime/schema"
-	quota "k8s.io/kubernetes/pkg/quota/v1"
-	generic "k8s.io/kubernetes/pkg/quota/v1/generic"
+	quota "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1"
+	generic "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1/generic"
 )
 
 // NewQuotaConfigurationForAdmission returns a quota configuration for admission control.
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/admission.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/admission.go
index 0d3eccbfe..0ab9893b7 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/admission.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/admission.go
@@ -27,11 +27,11 @@ import (
 	genericadmissioninitializer "k8s.io/apiserver/pkg/admission/initializer"
 	"k8s.io/client-go/informers"
 	"k8s.io/client-go/kubernetes"
-	kubeapiserveradmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
-	quota "k8s.io/kubernetes/pkg/quota/v1"
-	"k8s.io/kubernetes/pkg/quota/v1/generic"
-	resourcequotaapi "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
-	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation"
+	kubeapiserveradmission "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/kubeapiserver/admission"
+	quota "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1"
+	"github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1/generic"
+	resourcequotaapi "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+	"github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation"
 )
 
 // PluginName is a string with the name of the plugin
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/doc.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/doc.go
index 5a514f605..6a3ca86bb 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/doc.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/doc.go
@@ -16,4 +16,4 @@ limitations under the License.
 
 // +k8s:deepcopy-gen=package
 
-package resourcequota // import "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+package resourcequota // import "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/install/install.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/install/install.go
index d0d903307..e3e1fb9b2 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/install/install.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/install/install.go
@@ -21,10 +21,10 @@ package install
 import (
 	"k8s.io/apimachinery/pkg/runtime"
 	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
-	resourcequotaapi "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
-	resourcequotav1 "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1"
-	resourcequotav1alpha1 "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1"
-	resourcequotav1beta1 "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1"
+	resourcequotaapi "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+	resourcequotav1 "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1"
+	resourcequotav1alpha1 "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1"
+	resourcequotav1beta1 "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1"
 )
 
 // Install registers the API group and adds types to a scheme
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1/doc.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1/doc.go
index f0af6d369..bab661ac3 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1/doc.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1/doc.go
@@ -20,4 +20,4 @@ limitations under the License.
 // +groupName=resourcequota.admission.k8s.io
 
 // Package v1 is the v1 version of the API.
-package v1 // import "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1"
+package v1 // import "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1"
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1/zz_generated.conversion.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1/zz_generated.conversion.go
index 3d4276ee2..707a6b3db 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1/zz_generated.conversion.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1/zz_generated.conversion.go
@@ -26,7 +26,7 @@ import (
 	corev1 "k8s.io/api/core/v1"
 	conversion "k8s.io/apimachinery/pkg/conversion"
 	runtime "k8s.io/apimachinery/pkg/runtime"
-	resourcequota "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+	resourcequota "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
 )
 
 func init() {
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1/doc.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1/doc.go
index 5e8dc0975..566679399 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1/doc.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1/doc.go
@@ -20,4 +20,4 @@ limitations under the License.
 // +groupName=resourcequota.admission.k8s.io
 
 // Package v1alpha1 is the v1alpha1 version of the API.
-package v1alpha1 // import "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1"
+package v1alpha1 // import "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1"
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1/zz_generated.conversion.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1/zz_generated.conversion.go
index 3ca9511b3..57609e68f 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1/zz_generated.conversion.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1alpha1/zz_generated.conversion.go
@@ -26,7 +26,7 @@ import (
 	v1 "k8s.io/api/core/v1"
 	conversion "k8s.io/apimachinery/pkg/conversion"
 	runtime "k8s.io/apimachinery/pkg/runtime"
-	resourcequota "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+	resourcequota "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
 )
 
 func init() {
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1/doc.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1/doc.go
index 3f4dd11b2..a2b05329c 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1/doc.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1/doc.go
@@ -20,4 +20,4 @@ limitations under the License.
 // +groupName=resourcequota.admission.k8s.io
 
 // Package v1beta1 is the v1beta1 version of the API.
-package v1beta1 // import "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1"
+package v1beta1 // import "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1"
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1/zz_generated.conversion.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1/zz_generated.conversion.go
index bff5582a6..08e38bd9f 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1/zz_generated.conversion.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1beta1/zz_generated.conversion.go
@@ -26,7 +26,7 @@ import (
 	v1 "k8s.io/api/core/v1"
 	conversion "k8s.io/apimachinery/pkg/conversion"
 	runtime "k8s.io/apimachinery/pkg/runtime"
-	resourcequota "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+	resourcequota "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
 )
 
 func init() {
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation/validation.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation/validation.go
index 8e3581151..bbd8d77e1 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation/validation.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation/validation.go
@@ -19,7 +19,7 @@ package validation
 import (
 	"k8s.io/apimachinery/pkg/util/validation/field"
 
-	resourcequotaapi "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+	resourcequotaapi "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
 )
 
 // ValidateConfiguration validates the configuration.
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation/validation_test.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation/validation_test.go
index 8c8d18b07..afd8ce059 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation/validation_test.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/validation/validation_test.go
@@ -19,7 +19,7 @@ package validation
 import (
 	"testing"
 
-	resourcequotaapi "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+	resourcequotaapi "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
 )
 
 func TestValidateConfiguration(t *testing.T) {
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/config.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/config.go
index 979ffba13..b0de321a6 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/config.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/config.go
@@ -23,9 +23,9 @@ import (
 
 	"k8s.io/apimachinery/pkg/runtime"
 	"k8s.io/apimachinery/pkg/runtime/serializer"
-	resourcequotaapi "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
-	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/install"
-	resourcequotav1 "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1"
+	resourcequotaapi "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+	"github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/install"
+	resourcequotav1 "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota/v1"
 )
 
 var (
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/controller.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/controller.go
index ae6807edd..4bb829ae2 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/controller.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/controller.go
@@ -35,9 +35,9 @@ import (
 	"k8s.io/apimachinery/pkg/util/wait"
 	"k8s.io/apiserver/pkg/admission"
 	"k8s.io/client-go/util/workqueue"
-	quota "k8s.io/kubernetes/pkg/quota/v1"
-	"k8s.io/kubernetes/pkg/quota/v1/generic"
-	resourcequotaapi "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
+	quota "github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1"
+	"github.com/gardener/gardener/third_party/forked/kubernetes/pkg/quota/v1/generic"
+	resourcequotaapi "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"
 )
 
 // Evaluator is used to see if quota constraints are satisfied.
diff --git a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/doc.go b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/doc.go
index 42bee9fba..be78ecfb5 100644
--- a/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/doc.go
+++ b/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota/doc.go
@@ -16,4 +16,4 @@ limitations under the License.
 
 // Package resourcequota enforces all incoming requests against any applied quota
 // in the namespace context of the request
-package resourcequota // import "k8s.io/kubernetes/plugin/pkg/admission/resourcequota"
+package resourcequota // import "github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota"
```


