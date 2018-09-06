# NRP controller
This project provides code for NRP controller - the component allowing federating kubernetes clusters on same level according to internal cluster policies.

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

To federate with another cluster, we create the Cluster CRD object. The controller is watching the cluster objects, and will create the namespace corresponding to the cluster name. If such namespace is already taken, it will prepend a number to it.

[![asciicast](https://asciinema.org/a/L0xoBr7ljntJ9h5bvZ5499QRd.png)](https://asciinema.org/a/L0xoBr7ljntJ9h5bvZ5499QRd)

Inside the namespace it will create a service account with rolebinding to cluster-federation clusterrole, and kubernetes will automatically issue a token for the service account in secret in the same namespace. This token can be used to access the resources by the federated cluster.
[![asciicast](https://asciinema.org/a/fZXrBEBu9Tr24qU3r8oGLXjVh.png)](https://asciinema.org/a/fZXrBEBu9Tr24qU3r8oGLXjVh)

