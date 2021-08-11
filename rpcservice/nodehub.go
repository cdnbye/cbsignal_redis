package rpcservice

import (
	"github.com/lexkong/log"
	"sync"
)

type NodeHub struct {
	nodes map[string]*Node
	mu   sync.Mutex
	selfAddr string
}

var nodeHub *NodeHub

func NewNodeHub(selfAddr string) *NodeHub {
	n := NodeHub{
		nodes: make(map[string]*Node),
		selfAddr: selfAddr,
	}
	nodeHub = &n
	return &n
}

func GetNode(addr string) (*Node, bool) {
	// 如果rpc节点是本节点
	if addr == nodeHub.selfAddr {
		return nil, false
	}
	return nodeHub.Get(addr)
}

func GetTotalNumClient() int {
	sum := 0
	for _, node := range nodeHub.nodes {
		sum += node.NumClient
	}
	return sum
}

func GetNumNode() int {
	sum := 0
	for _, node := range nodeHub.nodes {
		if node.isAlive {
			sum += 1
		}
	}
	return sum
}

func (n *NodeHub) Delete(addr string) {
	log.Warnf("NodeHub delete %s", addr)
	//n.mu.Lock()
	if node, ok := n.nodes[addr]; ok {
		node.Released = true
		node.connPool.Shutdown()
	}
	delete(n.nodes, addr)
	//n.mu.Unlock()
}

func (n *NodeHub) Add(addr string, peer *Node) {
	log.Infof("NodeHub add %s", addr)
	n.nodes[addr] = peer
}

func (n *NodeHub) Get(addr string) (*Node, bool) {
	//n.mu.RLock()
	var err error
	node, ok := n.nodes[addr]
	if !ok {
		n.mu.Lock()
		node, ok = n.nodes[addr]
		if ok {
			n.mu.Unlock()
			return node, ok
		}
		log.Infof("New Node %s", addr)
		node, err = NewNode(addr)
		if err != nil {
			log.Error("NewNode", err)
			n.mu.Unlock()
			return nil, false
		}
		ok = true
		n.Add(addr, node)
		node.StartHeartbeat()
		n.mu.Unlock()
	}
	//n.mu.RUnlock()
	return node, ok
}

func (n *NodeHub) GetAll() map[string]*Node {
	//log.Infof("NodeHub GetAll %d", len(n.node))
	return n.nodes
}

func (n *NodeHub) Clear() {
	log.Infof("NodeHub clear")
	//n.mu.Lock()
	for _, node := range n.nodes {
		node.connPool.Shutdown()
	}
	n.nodes = make(map[string]*Node)
	//n.mu.Unlock()
}


