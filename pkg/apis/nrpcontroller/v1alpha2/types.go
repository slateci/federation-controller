package v1alpha2

import (
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cluster holds information about the current cluster
type Cluster struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata"`
	Spec              ClusterSpec `json:"spec"`
}

type ClusterSpec struct {
	Organization string `json:""`
	NS           string `json:""`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList is a list of cluster resources
type ClusterList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata"`
	Items           []Cluster `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterNS struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata"`
	Spec              ClusterSpec `json:"spec"`
}

type ClusterNSSpec struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterNSList is a list of Cluster Namespaces
type ClusterNSList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata"`
	Items           []ClusterNS `json:"items"`
}
