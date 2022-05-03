package v1alpha2

import (
	"context"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextcs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
)

const (
	ClusterCRDPlural   string = "clusters"
	ClusterCRDGroup    string = "nrp-nautilus.io"
	ClusterCRDVersion  string = "v1alpha2"
	FullClusterCRDName string = ClusterCRDPlural + "." + ClusterCRDGroup
)

// CreateClusterCRD creates a new Cluster CRD
func CreateClusterCRD(clientset apiextcs.Interface) error {
	crd := apiextv1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{Name: FullClusterCRDName},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: ClusterCRDGroup,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:    ClusterCRDVersion,
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
			Scope: apiextv1.ClusterScoped,
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural: ClusterCRDPlural,
				Kind:   reflect.TypeOf(Cluster{}).Name(),
			},
		},
	}

	opts := metaV1.CreateOptions{}
	_, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), &crd, opts)
	if err != nil && apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
