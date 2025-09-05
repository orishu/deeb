package bootstrap

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/transport"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Params are the parameters required for generating the bootstrap info
type Params struct {
	ClusterName            string
	NodeName               string
	DirWithNodeIDFile      string
	ClusterSize            int
	GRPCPort               string
	TransportClientFactory transport.ClientFactory
	Logger                 *zap.SugaredLogger
}

// BootstrapInfo consists of the required parameters for starting a new node
type BootstrapInfo struct {
	NodeID       uint64
	IsNewCluster bool
	NodeName     string
	ClusterName  string
	Peers        []nd.NodeInfo
}

// New derives the bootstrap information for a node that is starting up in a
// Kubernetes environment, where nodes are accessible through DNS names based
// on the cluster name.
func New(ctx context.Context, p Params) (BootstrapInfo, error) {
	nodeIDFilename := p.DirWithNodeIDFile + "/node_id"
	var nodeID uint64
	peers, err := getPeers(&p)
	if err != nil {
		return BootstrapInfo{}, err
	}
	if _, err = os.Stat(nodeIDFilename); err == nil {
		data, err := os.ReadFile(nodeIDFilename)
		if err != nil {
			return BootstrapInfo{}, errors.Wrapf(err, "reading node_id file %s", nodeIDFilename)
		}
		if len(data) > 0 {
			id, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
			if err != nil {
				return BootstrapInfo{}, errors.Wrapf(err, "parsing node_id file, data: %s", string(data))
			}
			nodeID = id
		}
	}
	if (err == nil && nodeID == 0) || os.IsNotExist(err) {
		var peer *nd.NodeInfo
		for attempts := 0; attempts < 3; attempts++ {
			nodeID, peer = findNewHighNodeID(ctx, peers, &p)
			if nodeID > 0 {
				break
			}
			time.Sleep(time.Second)
		}
		if peer != nil {
			err := reportNewNode(ctx, nodeID, *peer, &p)
			if err != nil {
				return BootstrapInfo{}, errors.Wrapf(err, "reporting new node to peer %s:%s", peer.Addr, peer.Port)
			}
		}
		if nodeID > 0 {
			err = os.WriteFile(nodeIDFilename, []byte(fmt.Sprintf("%d", nodeID)), 0644)
			if err != nil {
				return BootstrapInfo{}, errors.Wrapf(err, "writing node_id file %s", nodeIDFilename)
			}
		}
	} else {
		return BootstrapInfo{}, errors.Wrapf(err, "opening node_id file %s", nodeIDFilename)
	}

	if nodeID == 0 {
		return BootstrapInfo{}, errors.New("could not find node ID to use")
	}

	return BootstrapInfo{
		NodeID:       nodeID,
		IsNewCluster: nodeID == 1,
		NodeName:     p.NodeName,
		ClusterName:  p.ClusterName,
		Peers:        peers,
	}, nil
}

func getPeers(p *Params) ([]nd.NodeInfo, error) {
	prefix, idx, err := nodeNameParts(p.NodeName)
	if err != nil {
		return nil, err
	}
	peers := make([]nd.NodeInfo, 0, p.ClusterSize-1)
	for i := 0; i < p.ClusterSize; i++ {
		if i == idx {
			continue
		}
		peers = append(peers, nd.NodeInfo{
			Addr: fmt.Sprintf("%s%d.%s", prefix, i, p.ClusterName),
			Port: p.GRPCPort,
		})
	}
	return peers, nil
}

// findNewHighNodeID returns a new node ID that is higher than the highest one
// seen by other nodes, also returns the NodeInfo of the node from which the
// highest node ID was reported.
func findNewHighNodeID(ctx context.Context, peers []nd.NodeInfo, params *Params) (uint64, *nd.NodeInfo) {
	allErrors := true
	var highestNodeID uint64
	var activePeer nd.NodeInfo
	for _, peer := range peers {
		peer := peer
		client, err := params.TransportClientFactory(ctx, peer.Addr, peer.Port)
		if err != nil {
			params.Logger.Warnf("attempted GRPC connection to peer %s:%s, got error: %+v", peer.Addr, peer.Port, err)
			continue
		}
		nodeID, err := client.GetHighestID(ctx)
		if err != nil {
			params.Logger.Warnf("attempted getting highest node ID from %s:%s, got error: %+v", peer.Addr, peer.Port, err)
			continue
		}
		allErrors = false
		if activePeer.Addr == "" {
			// Set activePeer to the first working peer
			activePeer = peer
		}
		if nodeID > highestNodeID {
			highestNodeID = nodeID
			activePeer = peer
		}
	}
	if allErrors {
		_, nodeIdx, err := nodeNameParts(params.NodeName)
		if err != nil {
			params.Logger.Errorf("parsing node name %s, got error: %+v", params.NodeName, err)
			return 0, nil
		}
		if nodeIdx != 0 {
			params.Logger.Warnf("node index >0 cannot initiate a new cluster, node name %s", params.NodeName)
			return 0, nil
		}
		return 1, nil
	}
	return highestNodeID + 1, &activePeer
}

func nodeNameParts(nodeName string) (string, int, error) {
	exp := regexp.MustCompile(`(.*-)([0-9]+)$`)
	matches := exp.FindStringSubmatch(nodeName)
	if len(matches) != 3 {
		return "", 0, errors.New("unable to parse node name: " + nodeName)
	}
	idx, err := strconv.Atoi(matches[2])
	if err != nil {
		return "", 0, errors.Wrapf(err, "parsing node index: %s", nodeName)
	}
	return matches[1], idx, nil
}

// reportNewNode tells one of the active peers to add this node to the topology
func reportNewNode(ctx context.Context, nodeID uint64, existingNode nd.NodeInfo, params *Params) error {
	client, err := params.TransportClientFactory(ctx, existingNode.Addr, existingNode.Port)
	if err != nil {
		return errors.Wrapf(err, "failed GRPC connection to active peer %s:%s", existingNode.Addr, existingNode.Port)
	}
	addr := fmt.Sprintf("%s.%s", params.NodeName, params.ClusterName)
	err = client.AddNewPeer(ctx, nodeID, addr, params.GRPCPort)
	if err != nil {
		return errors.Wrap(err, "adding new peer")
	}
	return nil
}
