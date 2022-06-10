VERSION := 0.0.0

default: buildgo

buildgo:
	go build 

builddocker:
	docker build -t hub.opensciencegrid.org/slate/federation-controller:$(VERSION) -f resources/building/Dockerfile

pushminikube: 
	minikube image  build . -t hub.opensciencegrid.org/slate/federation-controller:$(VERSION)
	
pushdocker:
	docker push hub.opensciencegrid.org/slate/federation-controller:$(VERSION)

cleanup:
	rm federation-controller

buildrelease: buildgo builddocker pushdocker cleanup

buildminikube: buildgo pushminikube
