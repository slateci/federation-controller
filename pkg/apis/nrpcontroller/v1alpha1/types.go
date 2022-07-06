package v1alpha1

import (
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Cluster holds information about the current cluster
type Cluster struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata"`
	Spec              ClusterSpec `json:"spec"`
}

type ClusterSpec struct {
	Organization string `json:""`
	Namespace    string `json:""`
}

// ClusterList is a list of cluster resources
type ClusterList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata"`
	Items           []Cluster `json:"items"`
}

type ClusterNamespace struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata"`
	Spec              ClusterSpec `json:"spec"`
}

type ClusterNamespaceSpec struct {
}

// ClusterNamespaceList is a list of Cluster Namespaces
type ClusterNamespaceList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata"`
	Items           []ClusterNamespace `json:"items"`
}
