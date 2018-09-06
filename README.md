# NRP controller

This project provides code for NRP controller - the component allowing federating kubernetes clusters according to internal cluster policies.

The problem solved by this controller is giving kubernetes cluster admins ability to easily delegate to other clusters permission to deploy to this cluster and access its resources. The set of resources and policies is defined by a cluster role and can vary for different clusters.

Example: cluster_A federates with cluster_B. Admin in cluster_A creates a "cluster_B" object, which triggers autiomatic creation of all needed permissions and credentials. Credentials are then sent to cluster_B admins, and they use those to access cluster_A resources. When needed, deleting all those resources and permissions is as simple as deleting the cluster_B object. Modifying permissions is also trivial and is well described in kubernetes documentation (https://kubernetes.io/docs/reference/access-authn-authz/rbac/, https://kubernetes.io/docs/concepts/policy/pod-security-policy/)

### Developing

`go get gitlab.com/ucsd-prp/nrp-controller`

From project folder:

`dep ensure`

`./codegen.sh`

`make buildrelease`

### Installation

Download and edit the federation-role.yaml file from this repo, or apply it from web if standard admin role is fine:

`kubectl apply -f https://gitlab.com/ucsd-prp/nrp-controller/raw/master/federation-role.yaml`

Then start the controller:

`kubectl apply -f https://gitlab.com/ucsd-prp/nrp-controller/raw/master/deploy.yaml`

### Using

The controller creates 2 [CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/): 

* clusters.nrp-nautilus.io - Each CRD is an external federated cluster (or some other entity needing to federate)

* clusternamespaces.nrp-nautilus.io - the namespaces in current cluster belonging to external federated cluster

```
$ kubectl get crds | grep nrp-nautilus
clusternamespaces.nrp-nautilus.io             2018-09-06T01:42:40Z
clusters.nrp-nautilus.io                      2018-09-06T00:50:33Z
```

The clusters.nrp-nautilus.io are cluster-wide.

To federate with another cluster, we create the Cluster CRD object. The controller is watching the cluster objects, and will create the namespace corresponding to the cluster name. If such namespace is already taken, it will prepend a number to it. The cluster object will have the spec/namespace field changed to have the name of primary namespace. Inside the namespace it will create a service account with rolebinding to cluster-federation clusterrole. Once we delete the cluster, the associated namespace will be also deleted with all the contents in it.

[![asciicast](https://asciinema.org/a/BWXytQziditkuW0jAR4reGonx.png)](https://asciinema.org/a/BWXytQziditkuW0jAR4reGonx)

For new service account kubernetes automatically creates a token, which will allow federated cluster to act as this service account in the current cluster. The account by default only has access to its primary namespace. The token can be then added to a config file and securely sent to owners of the federated cluster, providing them access to this cluster according to the cluster-federation role defined by admin.

[![asciicast](https://asciinema.org/a/ZYIPVyFwqC3SkhnNNMBUmJsdI.png)](https://asciinema.org/a/ZYIPVyFwqC3SkhnNNMBUmJsdI)

To request creation of additional namespaces belonging to the same federated cluster, it can create the "clusternamespace" CRD object in its namespace. The operator will discover the new object and create the corresponding namespace with same service account in it, so that the same token will be valid to access both primary namespace and additional one(s). Deleting the clusternamespace object will result in deletion of the corresponding namespace. Deletion of the cluster object by admin will result in deletion of the primary and all additional namespaces.

[![asciicast](https://asciinema.org/a/l7pwo4kXPV4XcWYoGfNlAUEat.png)](https://asciinema.org/a/l7pwo4kXPV4XcWYoGfNlAUEat)


