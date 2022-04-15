package v1alpha1

import (
	"context"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextcs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"reflect"
)

const (
	ClusterNSCRDPlural   string = "clusternss"
	ClusterNSCRDGroup    string = "nrp-nautilus.io"
	ClusterNSCRDVersion  string = "v1alpha1"
	ClusterNSFullCRDName string = ClusterNSCRDPlural + "." + ClusterNSCRDGroup
)

// +k8s:deepcopy-gen=false

type ClusterNSCrdClient struct {
	cl     *rest.RESTClient
	plural string
	codec  runtime.ParameterCodec
}

// Create the CRD resource, ignore error if it already exists

func CreateNSCRD(clientset apiextcs.Interface) error {
	crd := apiextv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: ClusterNSFullCRDName},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: ClusterNSCRDGroup,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:    ClusterNSCRDVersion,
					Served:  true,
					Storage: true,
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"Organization": {Type: "string", Format: "str"},
										"NS":           {Type: "string", Format: "str"},
									},
									Required: []string{"Organization", "NS"},
								},
							},
						},
					},
				},
			},
			Scope: apiextv1.NamespaceScoped,
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural: ClusterNSCRDPlural,
				Kind:   reflect.TypeOf(ClusterNS{}).Name(),
			},
		},
	}

	_, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), &crd, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
