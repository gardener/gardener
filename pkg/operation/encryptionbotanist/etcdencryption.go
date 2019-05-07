package encryptionbotanist

import (
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type encryptionBotanistImpl struct {
	*operation.Operation
}

func (e *encryptionBotanistImpl) StartEtcdEncryption() error {
	logger.Logger.Info("Starting Etcd Encryption")

	pl, err := e.Operation.K8sSeedClient.ListPods(e.Operation.Shoot.SeedNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range pl.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		name := pod.GetName()
		logger.Logger.Info("pod: " + name)
	}

	//deployment, err := e.Operation.K8sSeedClient.GetDeployment(e.Operation.Shoot.SeedNamespace, common.KubeAPIServerDeploymentName)
	//logger.Logger.Info("deployment command length: ", len(deployment.Spec.Template.Spec.Containers[0].Command))
	//logger.Logger.Info("deployment: ", deployment.Spec.Template.Spec.Containers[0].Command[0])
	//logger.Logger.Info("deployment: ", deployment.Spec.Template.Spec.Containers[0].Command[1])
	//var manifest = []byte("")
	//e.Operation.K8sSeedClient.Applier().ApplyManifest(context.TODO(), kubernetes.NewManifestReader(manifest), kubernetes.DefaultApplierOptions)
	//e.Operation.
	return nil
}

func New(o *operation.Operation) (EncryptionBotanist, error) {

	return &encryptionBotanistImpl{
		Operation: o,
	}, nil
}

//
func (e *encryptionBotanistImpl) EncryptionConfigExists() (bool, error) {
	return false, nil
}
