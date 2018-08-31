package v1alpha1

import (
	"strings"

	authv1 "k8s.io/api/authorization/v1"

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
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PRPUserList is a list of PRP users
type ClusterList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []Cluster `json:"items"`
}

func (cluster Cluster) GetClusterClientset() (*kubernetes.Clientset, error) {
	clusterk8sconfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clusterk8sconfig.Impersonate = rest.ImpersonationConfig{
		ClusterName: cluster.Name,
	}

	return kubernetes.NewForConfig(clusterk8sconfig)

}
