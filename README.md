# What is Deeb?

Deeb is "etcd on SQL": it's a gRPC-interfaced highly available cluster of MySQL servers. It exposes RPCs for reliably running SQL commands on a group of MySQL servers, coordinated by [Raft](https://raft.github.io/) consensus protocol.

Deeb's main component is a controller process/container that runs side by side with a MySQL container as its Raft storage backend.
Deeb's Helm chart enables runing Deeb clusters on Kubernetes.

# Building and Running

## Requirements

* Go
* Kubernetes
* Helm
* Protobuf (for protoc binary)

## Building on Mac OS X

Cross-compiling the controller using Docker:
```
# for x86_64 CPUs (amd64):
scripts/build-for-linux.sh

OR

# for Apple silicon (arm64):
scripts/build-for-linux-on-apple-silicon.sh
```
Then,
```
# Move the Linux binary into the Docker build directory:
mv controller build/controller/controller
```

Build Docker images for the sidecar and controller:
```
# If running locally with minikube, set up environment:
eval $(minikube docker-env)

cd build/sidecar
docker build -t sidecar .
cd ../controller
docker build -t deeb-controller .
cd ../..
```

## Running on Kubernetes

One time setup - create an ssh key for the components to use:
```
kubectl create secret generic test-ssh-key --from-file=id_rsa=./test-id_rsa --from-file=id_rsa.pub=./test-id_rsa.pub

```

In case you did not build the images under minikube environment, you can load them so they are recognized by the local cluster:
```
minikube image load deeb-controller:latest
minikube image load sidecar:latest
```

Install using Helm:
```
cd deployments/helm
helm install mytest .
```

After a few minutes, all pods should be up.
Start port forwarding:
```
kubectl port-forward svc/mytest-deeb 8080:11000 &
```
Then, web UI for RPC calls is available at https://localhost:8080/openapi-ui/
Enter "https://localhost:8080/openapi-ui/controller.swagger.json" in the "Explore" text input.

To uninstall, run:
```
helm uninstall mytest
```
To remove persistent volumes, check the list of pvc's:
```
kubectl get pvc
```
You can delete them explicitly using `kubectl delete pvc/<name>`

## Deeb Project Copyrights

© 2020-2021 Ori Shalev All Rights Reserved
