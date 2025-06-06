package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rqlite/rqlite/v8/cluster/proto"
	"github.com/rqlite/rqlite/v8/store"
)

func Test_NewNodeFromServer(t *testing.T) {
	server := &store.Server{ID: "1", Addr: "192.168.1.1", Suffrage: "Voter"}
	node := NewNodeFromServer(server)

	if node.ID != server.ID || node.Addr != server.Addr || !node.Voter {
		t.Fatalf("NewNodeFromServer did not correctly initialize Node from Server")
	}
}

func Test_NewNodesFromServers(t *testing.T) {
	servers := []*store.Server{
		{ID: "1", Addr: "192.168.1.1", Suffrage: "Voter"},
		{ID: "2", Addr: "192.168.1.2", Suffrage: "Nonvoter"},
	}
	nodes := NewNodesFromServers(servers)

	if len(nodes) != len(servers) {
		t.Fatalf("NewNodesFromServers did not create the correct number of nodes")
	}
	for i, node := range nodes {
		if node.ID != servers[i].ID || node.Addr != servers[i].Addr {
			t.Fatalf("NewNodesFromServers did not correctly initialize Node %d from Server", i)
		}
	}
}

func Test_NodesVoters(t *testing.T) {
	nodes := Nodes{
		{ID: "1", Voter: true},
		{ID: "2", Voter: false},
	}
	voters := nodes.Voters()

	if len(voters) != 1 || !voters[0].Voter {
		t.Fatalf("Voters method did not correctly filter voter nodes")
	}
}

func Test_NodeTestLeader(t *testing.T) {
	node := &Node{ID: "1", Addr: "leader-raft-addr", APIAddr: "leader-api-addr"}
	mockGA := newMockGetAddresser("leader-api-addr", "1.0.0", nil)

	node.Test(mockGA, "leader-raft-addr", 0, 10*time.Second)
	if !node.Reachable || !node.Leader {
		t.Fatalf("Test method did not correctly update node status %s", asJSON(node))
	}
	if node.Version != "1.0.0" {
		t.Fatalf("Test method did not correctly update node version %s", asJSON(node))
	}
}

func Test_NodeTestNotLeader(t *testing.T) {
	node := &Node{ID: "1", Addr: "follower-raft-addr", APIAddr: "follower-api-addr"}
	mockGA := newMockGetAddresser("follower-api-addr", "2.0.0", nil)

	node.Test(mockGA, "leader-raft-addr", 0, 10*time.Second)
	if !node.Reachable || node.Leader {
		t.Fatalf("Test method did not correctly update node status %s", asJSON(node))
	}
	if node.Version != "2.0.0" {
		t.Fatalf("Test method did not correctly update node version %s", asJSON(node))
	}
}

func Test_NodeTestDouble(t *testing.T) {
	node1 := &Node{ID: "1", Addr: "leader-raft-addr", APIAddr: "leader-api-addr"}
	node2 := &Node{ID: "2", Addr: "follower-raft-addr", APIAddr: "follower-api-addr"}
	mockGA := &mockGetAddresser{}
	mockGA.getMetaFn = func(addr string, retries int, timeout time.Duration) (*proto.NodeMeta, error) {
		if addr == "leader-raft-addr" {
			return &proto.NodeMeta{
				Url:     "leader-api-addr",
				Version: "1.0.0",
			}, nil
		}
		return nil, fmt.Errorf("not reachable")
	}

	nodes := Nodes{node1, node2}
	nodes.Test(mockGA, "leader-raft-addr", 0, 10*time.Second)
	if !node1.Reachable || !node1.Leader || node1.Version != "1.0.0" || node2.Reachable || node2.Leader || node2.Error != "not reachable" {
		t.Fatalf("Test method did not correctly update node status %s", asJSON(nodes))
	}

	if !nodes.HasAddr("leader-raft-addr") {
		t.Fatalf("HasAddr method did not correctly find node")
	}
	if nodes.HasAddr("not-found") {
		t.Fatalf("HasAddr method incorrectly found node")
	}
}

func Test_NodeTestDouble_Timeout(t *testing.T) {
	node1 := &Node{ID: "1", Addr: "leader-raft-addr", APIAddr: "leader-api-addr"}
	node2 := &Node{ID: "2", Addr: "follower-raft-addr", APIAddr: "follower-api-addr"}
	mockGA := &mockGetAddresser{}
	mockGA.getMetaFn = func(addr string, retries int, timeout time.Duration) (*proto.NodeMeta, error) {
		if addr == "leader-raft-addr" {
			return &proto.NodeMeta{Url: "leader-api-addr", Version: "3.0.0"}, nil
		}
		time.Sleep(10 * time.Second) // Simulate a node just hanging when contacted.
		return nil, fmt.Errorf("not reachable")
	}

	nodes := Nodes{node1, node2}
	nodes.Test(mockGA, "leader-raft-addr", 0, 1*time.Second)
	if !node1.Reachable || !node1.Leader || node1.Version != "3.0.0" || node2.Reachable || node2.Leader || node2.Error != "timeout waiting for node to respond" {
		t.Fatalf("Test method did not correctly update node status %s", asJSON(nodes))
	}

	if !nodes.HasAddr("leader-raft-addr") {
		t.Fatalf("HasAddr method did not correctly find node")
	}
	if nodes.HasAddr("not-found") {
		t.Fatalf("HasAddr method incorrectly found node")
	}
}

func Test_NodesRespEncodeStandard(t *testing.T) {
	nodes := mockNodes()
	buffer := new(bytes.Buffer)
	encoder := NewNodesRespEncoder(buffer, false)

	err := encoder.Encode(nodes)
	if err != nil {
		t.Errorf("Encode failed: %v", err)
	}

	m := make(map[string]any)
	if err := json.Unmarshal(buffer.Bytes(), &m); err != nil {
		t.Errorf("Encode failed: %v", err)
	}
	if len(m) != 1 {
		t.Errorf("unexpected number of keys")
	}
	if _, ok := m["nodes"]; !ok {
		t.Errorf("nodes key missing")
	}
	nodesArray, ok := m["nodes"].([]any)
	if !ok {
		t.Errorf("nodes key is not an array")
	}
	if len(nodesArray) != 1 {
		t.Errorf("unexpected number of nodes")
	}
	node, ok := nodesArray[0].(map[string]any)
	if !ok {
		t.Errorf("node is not a map")
	}
	checkNode(t, node)
}

func Test_NodeRespEncodeLegacy(t *testing.T) {
	nodes := mockNodes()
	buffer := new(bytes.Buffer)
	encoder := NewNodesRespEncoder(buffer, true)

	err := encoder.Encode(nodes)
	if err != nil {
		t.Errorf("Encode failed: %v", err)
	}

	m := make(map[string]any)
	if err := json.Unmarshal(buffer.Bytes(), &m); err != nil {
		t.Errorf("Encode failed: %v", err)
	}
	if len(m) != 1 {
		t.Errorf("unexpected number of keys")
	}
	if _, ok := m["1"]; !ok {
		t.Errorf("node key missing")
	}
	node, ok := m["1"].(map[string]any)
	if !ok {
		t.Errorf("nodes key is not an map")
	}
	checkNode(t, node)
}

func Test_NodesRespDecoder_Decode_ValidJSON(t *testing.T) {
	jsonInput := `{"nodes":[{"id":"1","addr":"192.168.1.1","voter":true, "version": "1.2.3"},{"id":"2","addr":"192.168.1.2","voter":false}]}`
	reader := strings.NewReader(jsonInput)
	decoder := NewNodesRespDecoder(reader)

	var nodes Nodes
	err := decoder.Decode(&nodes)
	if err != nil {
		t.Errorf("Decode failed with valid JSON: %v", err)
	}

	if len(nodes) != 2 || nodes[0].ID != "1" || nodes[1].ID != "2" {
		t.Errorf("Decode did not properly decode the JSON into Nodes")
	}
}

func Test_NodesRespDecoder_Decode_InvalidJSON(t *testing.T) {
	invalidJsonInput := `{"nodes": "invalid"}`
	reader := strings.NewReader(invalidJsonInput)
	decoder := NewNodesRespDecoder(reader)

	var nodes Nodes
	err := decoder.Decode(&nodes)
	if err == nil {
		t.Error("Decode should fail with invalid JSON")
	}
}

func Test_NodesRespDecoder_Decode_EmptyJSON(t *testing.T) {
	emptyJsonInput := `{}`
	reader := strings.NewReader(emptyJsonInput)
	decoder := NewNodesRespDecoder(reader)

	var nodes Nodes
	err := decoder.Decode(&nodes)
	if err != nil {
		t.Errorf("Decode failed with empty JSON: %v", err)
	}

	if len(nodes) != 0 {
		t.Errorf("Decode should result in an empty Nodes slice for empty JSON")
	}
}

// mockGetMetaer is a mock implementation of the GetMetaer interface.
type mockGetAddresser struct {
	apiAddr   string
	version   string
	err       error
	getMetaFn func(addr string, retries int, timeout time.Duration) (*proto.NodeMeta, error)
}

// newMockGetAddresser creates a new instance of mockGetAddresser.
// You can customize the return values for GetNodeMeta by setting apiAddr, version, and err.
func newMockGetAddresser(apiAddr, version string, err error) *mockGetAddresser {
	return &mockGetAddresser{apiAddr: apiAddr, version: version, err: err}
}

// GetNodeMeta is the mock implementation of the GetNodeMeta method.
func (m *mockGetAddresser) GetNodeMeta(addr string, retries int, timeout time.Duration) (*proto.NodeMeta, error) {
	md := &proto.NodeMeta{
		Url:     m.apiAddr,
		Version: m.version,
	}
	if m.getMetaFn != nil {
		var err error
		md, err = m.getMetaFn(addr, retries, timeout)
		if err != nil {
			return nil, err
		}
	}
	return md, nil
}

func mockNodes() Nodes {
	return Nodes{
		&Node{ID: "1", APIAddr: "http://localhost:4001", Addr: "localhost:4002", Reachable: true, Leader: true},
	}
}

func checkNode(t *testing.T, node map[string]any) {
	t.Helper()
	if _, ok := node["id"]; !ok {
		t.Errorf("node is missing id")
	}
	if node["id"] != "1" {
		t.Errorf("unexpected node id")
	}
	if _, ok := node["api_addr"]; !ok {
		t.Errorf("node is missing api_addr")
	}
	if node["api_addr"] != "http://localhost:4001" {
		t.Errorf("unexpected node api_addr")
	}
	if _, ok := node["addr"]; !ok {
		t.Errorf("node is missing addr")
	}
	if node["addr"] != "localhost:4002" {
		t.Errorf("unexpected node addr")
	}
	if _, ok := node["reachable"]; !ok {
		t.Errorf("node is missing reachable")
	}
	if node["reachable"] != true {
		t.Errorf("unexpected node reachable")
	}
	if _, ok := node["leader"]; !ok {
		t.Errorf("node is missing leader")
	}
	if node["leader"] != true {
		t.Errorf("unexpected node leader")
	}
}

func asJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to JSON marshal value: %s", err.Error()))
	}
	return string(b)
}
