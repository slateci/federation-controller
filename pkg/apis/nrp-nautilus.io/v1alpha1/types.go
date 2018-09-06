package v1alpha1

import (

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Cluster struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               ClusterSpec `json:"spec"`
}

type ClusterSpec struct {
	Organization   string `json:""`
	Namespace      string `json:""`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterNamespace struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               ClusterSpec `json:"spec"`
}

type ClusterNamespaceSpec struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []Cluster `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterNamespaceList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []ClusterNamespace `json:"items"`
}

func (cluster Cluster) GetClusterClientset() (*kubernetes.Clientset, error) {
	clusterk8sconfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clusterk8sconfig.Impersonate = rest.ImpersonationConfig{
		UserName: cluster.Name,
	}

	return kubernetes.NewForConfig(clusterk8sconfig)

}

func (cluster ClusterNamespace) GetClusterNamespaceClientset() (*kubernetes.Clientset, error) {
	clusterk8sconfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clusterk8sconfig.Impersonate = rest.ImpersonationConfig{
		UserName: cluster.Name,
	}

	return kubernetes.NewForConfig(clusterk8sconfig)

}
