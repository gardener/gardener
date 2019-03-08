package plant

import (
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	kubernetesclientset "k8s.io/client-go/kubernetes"

	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type defaultPlantControl struct {
	k8sGardenClient kubernetes.Interface
	plantClient     client.Client
	discoveryClient *kubernetesclientset.Clientset
	plantLister     gardencorelisters.PlantLister
	secretsLister   kubecorev1listers.SecretLister
	recorder        record.EventRecorder
	config          *config.ControllerManagerConfiguration
}

type plantStatusInfo struct {
	cloudType  string
	region     string
	k8sVersion string
}
