package v1alpha1

import (
	"context"
	"reflect"
	"time"

	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextcs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	ClusterCRDPlural   string = "clusters"
	ClusterCRDGroup    string = "nrpapi"
	ClusterCRDVersion  string = "v1alpha1"
	FullClusterCRDName string = ClusterCRDPlural + "." + ClusterCRDGroup
)

// Create the CRD resource, ignore error if it already exists

func CreateClusterCRD(ctx context.Context, clientset apiextcs.Interface) error {
	crd := &apiextv1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{Name: FullClusterCRDName},
		Spec: apiextv1beta1.CustomResourceDefinitionSpec{
			Group: ClusterCRDGroup,
			Versions: []apiextv1beta1.CustomResourceDefinitionVersion{
				{
					Name:    ClusterCRDVersion,
					Served:  true,
					Storage: true,
				},
			},
			Scope: apiextv1beta1.ClusterScoped,
			Names: apiextv1beta1.CustomResourceDefinitionNames{
				Plural: ClusterCRDPlural,
				Kind:   reflect.TypeOf(Cluster{}).Name(),
			},
		},
	}

	opts := metaV1.CreateOptions{}
	_, err := clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(ctx, crd, opts)
	if err != nil && apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err

	// Note the original apiextensions example adds logic to wait for creation and exception handling
}

func NewClusterClient(cfg *rest.Config) (*rest.RESTClient, *runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	SchemeBuilder := runtime.NewSchemeBuilder(addKnownTypes)
	if err := SchemeBuilder.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}
	config := *cfg
	config.GroupVersion = &SchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	// NegotiatedSerializer is not used
	// config.NegotiatedSerializer = serializer.DirectCodecFactory{
	//	CodecFactory: serializer.NewCodecFactory(scheme)}

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, nil, err
	}
	return client, scheme, nil
}

func MakeClusterCrdClient(cl *rest.RESTClient, scheme *runtime.Scheme, namespace string) *ClusterCrdClient {
	return &ClusterCrdClient{cl: cl, ns: namespace, plural: ClusterCRDPlural,
		codec: runtime.NewParameterCodec(scheme)}
}

// +k8s:deepcopy-gen=false

type ClusterCrdClient struct {
	cl     *rest.RESTClient
	ns     string
	plural string
	codec  runtime.ParameterCodec
}

func (f *ClusterCrdClient) Create(ctx context.Context, obj *Cluster) (*Cluster, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var result Cluster
	err := f.cl.Post().
		Namespace(f.ns).Resource(f.plural).
		Body(obj).Do(reqCtx).Into(&result)
	return &result, err
}

func (f *ClusterCrdClient) Update(ctx context.Context, obj *Cluster) (*Cluster, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var result Cluster
	err := f.cl.Put().
		Namespace(f.ns).Resource(f.plural).Name(obj.Name).
		Body(obj).Do(reqCtx).Into(&result)
	return &result, err
}

func (f *ClusterCrdClient) Delete(ctx context.Context, name string, options *metaV1.DeleteOptions) error {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return f.cl.Delete().
		Namespace(f.ns).Resource(f.plural).
		Name(name).Body(options).Do(reqCtx).
		Error()
}

func (f *ClusterCrdClient) Get(ctx context.Context, name string) (*Cluster, error) {
	var result Cluster
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := f.cl.Get().
		Namespace(f.ns).Resource(f.plural).
		Name(name).Do(reqCtx).Into(&result)
	return &result, err
}

func (f *ClusterCrdClient) List(ctx context.Context, namespace string, opts metaV1.ListOptions) (*ClusterList, error) {
	var result ClusterList
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := f.cl.Get().
		Namespace(namespace).Resource(f.plural).
		VersionedParams(&opts, f.codec).
		Do(reqCtx).Into(&result)
	return &result, err
}

// Create a new List watch for our TPR

func (f *ClusterCrdClient) NewListWatch() *cache.ListWatch {
	return cache.NewListWatchFromClient(f.cl, f.plural, f.ns, fields.Everything())
}