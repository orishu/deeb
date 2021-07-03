# Deeb

TODO

## Requirements

* Go
* Kubernetes

## Building on Mac OS X

Cross-compiling the controller:
```
# Install cross-compiling library if not installed yet:
brew install FiloSottile/musl-cross/musl-cros
scripts/build-for-linux.sh

# Move the Linux binary into the Docker build directory:
mv controller build/controller/controller
```

Build Docker images for the sidecar and controller:
```
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

To uninstall, run:
```
helm uninstall mytest
```
To remove persistent volumes, check the list of pvc's:
```
kubectl get pvc
```
You can delete them explicitly using `kubectl delete pvc/<name>`
