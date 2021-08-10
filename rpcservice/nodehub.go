package rpcservice

import (
	"github.com/lexkong/log"
	"sync"
)

type NodeHub struct {
	nodes map[string]*Node
	mu   sync.RWMutex
}

var nodeHub *NodeHub

func NewNodeHub() *NodeHub {
	n := NodeHub{
		nodes: make(map[string]*Node),
	}
	nodeHub = &n
	return &n
}

func GetNode(addr string) (*Node, bool) {
	return nodeHub.Get(addr)
}

func (n *NodeHub) Delete(addr string) {
	log.Warnf("NodeHub delete %s", addr)
	n.mu.Lock()
	if node, ok := n.nodes[addr]; ok {
		node.Released = true
		node.connPool.Shutdown()
	}
	delete(n.nodes, addr)
	n.mu.Unlock()
}

func (n *NodeHub) Add(addr string, peer *Node) {
	log.Infof("NodeHub add %s", addr)
	n.nodes[addr] = peer
}

func (n *NodeHub) Get(addr string) (*Node, bool) {
	n.mu.RLock()
	var err error
	node, ok := n.nodes[addr]
	if !ok {
		log.Infof("New Node %s", addr)
		node, err = NewNode(addr)
		if err != nil {
			log.Error("NewNode", err)
			return nil, false
		}
		ok = true
		n.Add(addr, node)
		node.StartHeartbeat()
	}
	n.mu.RUnlock()
	return node, ok
}

func (n *NodeHub) GetAll() map[string]*Node {
	//log.Infof("NodeHub GetAll %d", len(n.node))
	return n.nodes
}

func (n *NodeHub) Clear() {
	log.Infof("NodeHub clear")
	n.mu.Lock()
	for _, node := range n.nodes {
		node.connPool.Shutdown()
	}
	n.nodes = make(map[string]*Node)
	n.mu.Unlock()
}


