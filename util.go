package main

import (
	"context"
	fedv1alpha2 "github.com/slateci/federation-controller/pkg/apis/federationcontroller/v1alpha2"
	nrpv1alpha1 "github.com/slateci/federation-controller/pkg/apis/nrpcontroller/v1alpha1"
	"github.com/slateci/federation-controller/pkg/generated/clientset/versioned"
	clientset "github.com/slateci/federation-controller/pkg/generated/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func getKubeClientSet() *kubernetes.Clientset {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}
	return kubeClient
}

func getClusterClientSet() *versioned.Clientset {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	clusterClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building clientset: %s", err.Error())
	}

	return clusterClient
}

func getOldSchemaClient() *rest.RESTClient {
	config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	nrpv1alpha1.AddToScheme(scheme.Scheme)

	crdConfig := *config
	crdConfig.ContentConfig.GroupVersion = &schema.GroupVersion{Group: nrpv1alpha1.ClusterCRDGroup, Version: nrpv1alpha1.ClusterCRDVersion}
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	clusterRestClient, err := rest.UnversionedRESTClientFor(&crdConfig)
	if err != nil {
		klog.Fatalf("Got err while getting http client for old CRD: %s", err.Error())
	}
	return clusterRestClient
}

func getOldClusterCRDs() fedv1alpha2.ClusterList {

	clusterRestClient := getOldSchemaClient()
	oldClusterList := nrpv1alpha1.ClusterList{}

	err := clusterRestClient.Get().Resource("clusters").Do(context.TODO()).Into(&oldClusterList)
	if errors.IsNotFound(err) {
		klog.V(4).Info("No CRDs present, upgrade not needed")
		return fedv1alpha2.ClusterList{}
	} else if err != nil {
		klog.Fatalf("Can't get old cluster CRD: %s", err.Error())
	}
	newClusterList := []fedv1alpha2.Cluster{}
	for _, cluster := range oldClusterList.Items {
		newClusterList = append(newClusterList, fedv1alpha2.Cluster{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:        cluster.Spec.Namespace,
				Annotations: map[string]string{"upgraded-by": "federation-controller"},
			},
			Spec: fedv1alpha2.ClusterSpec{
				Organization: cluster.Spec.Organization,
				Namespace:    cluster.Spec.Namespace,
			},
		})
	}
	finalList := fedv1alpha2.ClusterList{Items: newClusterList}
	return finalList
}

func getOldClusterNSCRDs(namespaces []string) fedv1alpha2.ClusterNSList {
	clusterRestClient := getOldSchemaClient()
	oldClusterNamespaceList := nrpv1alpha1.ClusterNamespaceList{}

	for _, namespace := range namespaces {
		klog.V(4).Infof("Looking for clusternamespaces in namespace %s", namespace)
		tmpNSList := nrpv1alpha1.ClusterNamespaceList{}
		err := clusterRestClient.Get().Namespace(namespace).Resource("clusternamespaces").Do(context.TODO()).Into(&tmpNSList)
		if errors.IsNotFound(err) {
			continue
		} else if err != nil {
			klog.Fatalf("Can't get old ClusterNamespace CRD: %s", err.Error())
		}
		oldClusterNamespaceList.Items = append(oldClusterNamespaceList.Items, tmpNSList.Items...)
	}

	if len(oldClusterNamespaceList.Items) == 0 {
		klog.V(4).Info("No CRDs present, upgrade not needed")
		return fedv1alpha2.ClusterNSList{}
	}

	newClusterNSList := []fedv1alpha2.ClusterNS{}
	for _, clusterNS := range oldClusterNamespaceList.Items {
		newClusterNSList = append(newClusterNSList, fedv1alpha2.ClusterNS{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:        clusterNS.Name,
				Namespace:   clusterNS.Namespace,
				Annotations: map[string]string{"upgraded-by": "federation-controller"},
			},
			Spec: fedv1alpha2.ClusterSpec{
				Organization: clusterNS.Spec.Organization,
				Namespace:    clusterNS.Spec.Namespace,
			},
		})
	}
	finalList := fedv1alpha2.ClusterNSList{Items: newClusterNSList}
	return finalList
}

func upgradeOldCRDS() {
	newClusters := getOldClusterCRDs()
	if len(newClusters.Items) == 0 {
		klog.V(4).Info("No Cluster upgrade needed")
		return
	}
	clusterClient := getClusterClientSet()
	// namespaces should have a list of namespaces where clusterns crds may live
	var namespaces []string
	for _, cluster := range newClusters.Items {
		namespaces = append(namespaces, cluster.Spec.Namespace)
		_, err := clusterClient.FederationcontrollerV1alpha2().Clusters(cluster.Spec.Namespace).Create(context.TODO(), &cluster, metav1.CreateOptions{})
		// if the cluster already exists, it was created on a prior run and is not an error
		if err != nil && !errors.IsAlreadyExists(err) {
			klog.Fatalf("Can't update cluster %s: %s", cluster.Name, err.Error())
		}

	}
	newClusterNS := getOldClusterNSCRDs(namespaces)
	if len(newClusterNS.Items) == 0 {
		klog.V(4).Info("No ClusterNamespace upgrade needed")
		return
	}
	for _, clusterNS := range newClusterNS.Items {
		_, err := clusterClient.FederationcontrollerV1alpha2().ClusterNSs(clusterNS.Namespace).Create(context.TODO(), &clusterNS, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			klog.Fatalf("Can't update clusterNS %s: %s", clusterNS.Name, err.Error())
		}
	}
}
