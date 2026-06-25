package scheme

import (
	data "github.com/Kismet-Engineering/polykube/operator/api/data/v1alpha1"
	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
	routing "github.com/Kismet-Engineering/polykube/operator/api/routing/v1alpha1"
	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func New() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

func AddToScheme(scheme *runtime.Scheme) error {
	adders := []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		infrastructure.AddToScheme,
		runtimeapi.AddToScheme,
		routing.AddToScheme,
		data.AddToScheme,
	}

	for _, add := range adders {
		if err := add(scheme); err != nil {
			return err
		}
	}
	return nil
}
