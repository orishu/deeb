# What is Deeb?

**Deeb** is a breakthrough in distributed database architecture: **"etcd on SQL"** - a highly available cluster of MySQL servers that combines the reliability of Raft consensus with the power and familiarity of SQL.

## The Challenge

Modern applications demand **both** the ACID guarantees of relational databases **and** the high availability of distributed systems. Traditional solutions force you to choose:
- **Single MySQL instances**: Rich SQL capabilities but single points of failure
- **MySQL replication**: Complex to manage, prone to split-brain scenarios  
- **NoSQL systems**: High availability but limited query capabilities and eventual consistency

Deeb solves this fundamental trade-off.

## The Innovation

Deeb implements **distributed SQL with strong consistency** by integrating:

- **[Raft Consensus Protocol](https://raft.github.io/)**: Industry-proven algorithm used by etcd, Consul, and MongoDB for leader election and log replication
- **MySQL Storage Engine**: Full SQL capabilities with ACID transactions
- **gRPC API**: Modern, efficient inter-service communication
- **Kubernetes-Native Design**: Cloud-native deployment with StatefulSets and Helm

### Architecture Highlights

- **Sidecar Pattern**: Controller processes run alongside MySQL containers, providing distributed coordination without modifying MySQL itself
- **Consensus-Driven Operations**: Every SQL write operation goes through Raft consensus, ensuring cluster-wide consistency
- **Streaming Query Interface**: Real-time result streaming for large datasets
- **Automatic Failover**: Leader election and follower promotion with zero manual intervention
- **Cross-Platform Compatibility**: Supports both MySQL and SQLite backends for development and production

## Technical Depth

Deeb tackles several complex distributed systems challenges:

### 1. **Distributed Consensus**
- Implements etcd's Raft v3 library for battle-tested leader election
- Handles network partitions, node failures, and cluster membership changes
- Ensures linearizable consistency across all operations

### 2. **State Machine Replication**
- SQL operations are replicated as Raft log entries
- Deterministic application of database changes across all nodes
- Snapshot and restore capabilities for efficient cluster recovery

### 3. **Cloud-Native Orchestration**
- Advanced StatefulSet configurations with ordered startup
- Sophisticated readiness probes for distributed system health
- Helm charts with configurable resource limits and storage classes

### 4. **Protocol Integration**  
- Custom protobuf definitions bridging Raft messages and gRPC transport
- OpenAPI documentation with embedded Swagger UI
- HTTP/2 gateway for RESTful access alongside native gRPC

## Potential Impact

Deeb represents a new category of **Consensus-Backed Relational Databases** that could transform how we build resilient applications:

### For Developers
- **Familiar SQL Interface**: No need to learn new query languages or data models
- **Built-in High Availability**: Eliminate complex replication setup and monitoring
- **Strong Consistency**: ACID transactions across distributed nodes
- **Kubernetes Integration**: Deploy with a single Helm command

### For Operations
- **Automatic Recovery**: Self-healing clusters with no manual intervention
- **Horizontal Scaling**: Add nodes dynamically to increase availability
- **Cloud Portability**: Runs on any Kubernetes distribution
- **Observability**: Rich metrics and health endpoints for monitoring

### For the Industry
- **Proof of Concept**: Demonstrates feasibility of Raft-coordinated SQL clusters
- **Open Architecture**: Clean interfaces for extending to other storage engines  
- **Research Foundation**: Platform for exploring distributed transaction protocols
- **Educational Value**: Real-world implementation of distributed systems concepts

Deeb bridges the gap between traditional RDBMS reliability and modern distributed systems scalability, offering a glimpse into the future of database infrastructure.

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

To invoke HTTP API from the command-line, you can use kubectl:
```
# Optional: aliasing "k" to the minikube "kubectl":
alias k="minikube kubectl --"

k exec mytest-deeb-0 -c mytest-deeb-controller -- curl -s -k https://localhost:11000/api/v1/raft/progress
```

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

© 2020-2025 Ori Shalev All Rights Reserved
