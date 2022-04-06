package v1alpha1

import (
	"context"
	"k8s.io/api/storage/v1alpha1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextcs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"log"
	"reflect"
)

const (
	ClusterNSCRDPlural   string = "clusternamespaces"
	ClusterNSCRDGroup    string = "nrp-nautilus.io"
	ClusterNSCRDVersion  string = "v1alpha1"
	ClusterNSFullCRDName string = ClusterNSCRDPlural + "." + ClusterNSCRDGroup
	//	ClusterNSListFullCRDName string = ClusterNSListCRDPlural + "." + ClusterNSCRDGroup
)

// Create the CRD resource, ignore error if it already exists

func CreateNSCRD(ctx context.Context, clientset apiextcs.Interface) error {
	log.Printf("CreateNSCRD")
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
										"organization": {Type: "string", Format: "str"},
										"namespace":    {Type: "string", Format: "str"},
									},
									Required: []string{"organization", "namespace"},
								},
							},
						},
					},
				},
			},
			Scope: apiextv1.NamespaceScoped,
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural: ClusterNSCRDPlural,
				Kind:   reflect.TypeOf(ClusterNamespace{}).Name(),
			},
		},
	}

	_, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, &crd, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) {
		return nil
	}
	log.Printf("CreateNSCRD done")
	return err
}

// CreateNSListCRD creates the ClusterNamespaceList CRD on the k8s cluster
//func CreateNSListCRD(ctx context.Context, clientset apiextcs.Interface) error {
//	crd := apiextv1.CustomResourceDefinition{
//		ObjectMeta: metav1.ObjectMeta{Name: ClusterNSListFullCRDName},
//		Spec: apiextv1.CustomResourceDefinitionSpec{
//			Group: ClusterNSCRDGroup,
//			Versions: []apiextv1.CustomResourceDefinitionVersion{
//				{
//					Name:    ClusterCRDVersion,
//					Served:  true,
//					Storage: true,
//					Schema: &apiextv1.CustomResourceValidation{
//						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
//							Type: "object",
//							Properties: map[string]apiextv1.JSONSchemaProps{
//								"spec": {
//									Type: "object",
//									Properties: map[string]apiextv1.JSONSchemaProps{
//										"items": {Type: "array"},
//									},
//									Required: []string{"items"},
//								},
//							},
//						},
//					},
//				},
//			},
//			Scope: apiextv1.NamespaceScoped,
//			Names: apiextv1.CustomResourceDefinitionNames{
//				Plural: ClusterNSListCRDPlural,
//				Kind:   reflect.TypeOf(ClusterNamespaceList{}).Name(),
//			},
//		},
//	}
//
//	_, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, &crd, metav1.CreateOptions{})
//	if err != nil && apierrors.IsAlreadyExists(err) {
//		return nil
//	}
//	return err
//}

func NewClient(cfg *rest.Config) (*rest.RESTClient, *runtime.Scheme, error) {
	log.Printf("new cluster client config")
	scheme := runtime.NewScheme()
	log.Printf("new scheme builder call")
	SchemeBuilder := runtime.NewSchemeBuilder(addKnownTypes)
	if err := SchemeBuilder.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}
	log.Printf("new cluster config, scheme: %+v", scheme.KnownTypes(SchemeGroupVersion))

	config := *cfg
	config.GroupVersion = &SchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	v1alpha1.AddToScheme(clientscheme.Scheme)
	//codec := runtime.NoopEncoder{Decoder: apiserver.Codecs.UniversalDecoder()}
	//config.NegotiatedSerializer = serializer.NegotiatedSerializerWrapper(runtime.SerializerInfo{Serializer: codec})
	config.NegotiatedSerializer = serializer.NewCodecFactory(clientscheme.Scheme)
	//client, err := rest.RESTClientFor(&config)
	client, err := rest.UnversionedRESTClientFor(&config)
	//log.Printf("config: %+v", config)

	if err != nil {
		return nil, nil, err
	}
	return client, scheme, nil
}

func MakeClusterNSCrdClient(cl *rest.RESTClient, scheme *runtime.Scheme) *ClusterNSCrdClient {
	return &ClusterNSCrdClient{cl: cl, plural: ClusterNSCRDPlural,
		codec: runtime.NewParameterCodec(scheme)}
}

// +k8s:deepcopy-gen=false

type ClusterNSCrdClient struct {
	cl     *rest.RESTClient
	plural string
	codec  runtime.ParameterCodec
}

func (f *ClusterNSCrdClient) Create(ctx context.Context, obj *ClusterNamespace) (*ClusterNamespace, error) {
	var result ClusterNamespace
	err := f.cl.Post().
		Namespace(obj.Namespace).Resource(f.plural).
		Body(obj).Do(ctx).Into(&result)
	return &result, err
}

func (f *ClusterNSCrdClient) Update(ctx context.Context, obj *ClusterNamespace) (*ClusterNamespace, error) {
	var result ClusterNamespace
	err := f.cl.Put().
		Namespace(obj.Namespace).Resource(f.plural).Name(obj.Name).
		Body(obj).Do(ctx).Into(&result)
	return &result, err
}

func (f *ClusterNSCrdClient) Delete(ctx context.Context, name string, namespace string, options *metav1.DeleteOptions) error {
	return f.cl.Delete().
		Namespace(namespace).Resource(f.plural).
		Name(name).Body(options).Do(ctx).
		Error()
}

func (f *ClusterNSCrdClient) Get(ctx context.Context, name string, namespace string) (*ClusterNamespace, error) {
	var result ClusterNamespace
	log.Printf("Getting cluster namespace")
	err := f.cl.Get().
		Namespace(namespace).Resource(f.plural).
		Name(name).Do(ctx).Into(&result)
	return &result, err
}

func (f *ClusterNSCrdClient) List(ctx context.Context, namespace string, opts metav1.ListOptions) (*ClusterNamespaceList, error) {
	log.Println("NS List func")
	var result ClusterNamespaceList
	err := f.cl.
		Get().
		Namespace(namespace).
		Resource(f.plural).
		VersionedParams(&opts, f.codec).
		Do(ctx).
		Into(&result)
	log.Println("NS List func done")
	return &result, err
}

func (f *ClusterNSCrdClient) Watch(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	log.Println("NS Watch func")
	//reqCtx, cancel := context.WithCancel(ctx)
	//defer cancel()
	reqCtx := context.TODO()
	log.Println("NS Watch")
	return f.cl.
		Get().
		Namespace(namespace).
		Resource(f.plural).
		VersionedParams(&opts, f.codec).
		Watch(reqCtx)
}

// Create a new List watch for our TPR

func (f *ClusterNSCrdClient) NewListWatch(namespace string) *cache.ListWatch {
	log.Println("NewListWatch cache NS")
	return cache.NewListWatchFromClient(f.cl, f.plural, namespace, fields.Everything())
}
