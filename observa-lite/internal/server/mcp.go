package server

import (
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net/http"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (s *Server) mcp(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	if !decode(w, r, &req) {
		return
	}
	if req.JSONRPC != "2.0" {
		fail(w, 400, fmt.Errorf("expected JSON-RPC 2.0"))
		return
	}
	if req.Method == "notifications/initialized" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result interface{}
	var rpcErr interface{}
	switch req.Method {
	case "initialize":
		result = map[string]interface{}{"protocolVersion": "2025-03-26", "capabilities": map[string]interface{}{"tools": map[string]bool{"listChanged": false}}, "serverInfo": map[string]string{"name": "observa-lite", "version": "0.1.0"}}
	case "ping":
		result = map[string]interface{}{}
	case "tools/list":
		result = map[string]interface{}{"tools": []map[string]interface{}{
			{"name": "list_hosts", "description": "List discovered Linux hosts and their freshness", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
			{"name": "list_services", "description": "List automatically discovered service nodes", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
			{"name": "service_dependencies", "description": "List observed TCP application-to-application relationships", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
			{"name": "network_flows", "description": "List observed host-to-host flows (not physical topology)", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
			{"name": "list_incidents", "description": "List evidence-based host disappearance and template failures", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
		}}
	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &params)
		snap := s.Store.Snapshot()
		var data interface{}
		switch params.Name {
		case "list_hosts":
			data = snap.Hosts
		case "list_services":
			data = snap.ServiceMap.Nodes
		case "service_dependencies":
			data = snap.ServiceMap.Edges
		case "network_flows":
			data = snap.NetworkMap
		case "list_incidents":
			data = snap.Events
		default:
			rpcErr = map[string]interface{}{"code": -32602, "message": "unknown tool"}
		}
		if rpcErr == nil {
			b, _ := json.Marshal(data)
			result = map[string]interface{}{"content": []map[string]string{{"type": "text", "text": string(b)}}}
		}
	default:
		rpcErr = map[string]interface{}{"code": -32601, "message": "method not found"}
	}
	reply := map[string]interface{}{"jsonrpc": "2.0", "id": req.ID}
	if rpcErr != nil {
		reply["error"] = rpcErr
	} else {
		reply["result"] = result
	}
	write(w, 200, reply)
}

// Compile-time safeguard: MCP only exposes derived, non-mutating model types.
var _ = model.Snapshot{}
