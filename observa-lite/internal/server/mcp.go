package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func schemaFor(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": schemaFor(t.Elem())}
	case reflect.Struct:
		props := map[string]any{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" || name == "updated_at" {
				continue
			}
			props[name] = schemaFor(f.Type)
		}
		return map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	default:
		return map[string]any{"type": "string"}
	}
}
func mcpTools() []map[string]any {
	text := map[string]any{"type": "string"}
	integer := map[string]any{"type": "integer"}
	apply := map[string]any{"type": "boolean", "default": false, "description": "false previews a proposed change; true applies the explicitly requested change"}
	tools := []map[string]any{}
	add := func(name, description string, write, external bool, props map[string]any, required ...string) {
		schema := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
		if len(required) > 0 {
			schema["required"] = required
		}
		tools = append(tools, map[string]any{"name": name, "description": description, "inputSchema": schema, "annotations": map[string]bool{"readOnlyHint": !write, "destructiveHint": write, "idempotentHint": name != "run_operation", "openWorldHint": external}})
	}
	for _, entry := range [][2]string{{"list_hosts", "List discovered hosts without executable command lines or credentials"}, {"list_services", "List services observed in spans"}, {"service_dependencies", "Observed cross-service span relationships"}, {"network_flows", "Observed IP flows, not physical network topology"}, {"list_incidents", "Monitor events"}, {"list_frontends", "Agent-discovered HTML listeners; discovery does not imply RUM installed"}, {"list_databases", "Database inventory from observed processes and OTLP attributes"}} {
		add(entry[0], entry[1], false, false, map[string]any{})
	}
	add("run_query", "Execute read-only VQL: fetch hosts | filter os contains \"Linux\" | fields name, os | limit 50. Sources: hosts, processes, services, spans, logs, metrics, rum, frontends, databases, vulnerabilities, events. Stages: filter, fields, sort, limit, summarize count()/avg(field)/sum(field)/min(field)/max(field) by field. Not Dynatrace DQL or SQL.", false, false, map[string]any{"query": text, "minutes": integer}, "query")
	add("list_workspace", "List notebooks, workflows, database profiles or software inventories. Contains environment variable names, never DSN values.", false, false, map[string]any{"kind": map[string]any{"type": "string", "enum": []string{"notebook", "workflow", "database", "software"}}})
	add("list_operation_runs", "Read retained execution results; errors and partial scan coverage are explicit.", false, false, map[string]any{"object_id": text})
	add("configure_workspace", "Create/update notebook, workflow, PostgreSQL profile or software inventory. Workflow step kinds: query, condition, database_check, vulnerability_scan. SQL credentials use OBSERVA_DB_NAME_DSN environment names, never secrets. Preview by default; apply=true commits.", true, false, map[string]any{"object": schemaFor(reflect.TypeOf(model.WorkspaceObject{})), "apply": apply}, "object")
	add("configure_rum", "Create/update frontend privacy, sampling and Apdex. Origins must be the exact public origins. Requires browser instrumentation; cannot inject arbitrary runtimes. Preview by default.", true, false, map[string]any{"application": schemaFor(reflect.TypeOf(model.RUMApplication{})), "apply": apply}, "application")
	add("configure_host", "Change host display name/group or process alias. Group is created when assigned. Preview by default; executable names on the host are not modified.", true, false, map[string]any{"host_id": text, "settings": schemaFor(reflect.TypeOf(store.InventoryEdit{})), "apply": apply}, "host_id", "settings")
	add("run_operation", "Start an existing workflow, read-only PostgreSQL check or OSV package scan. OSV sends package names and versions to api.osv.dev. No shell, arbitrary SQL or arbitrary outbound webhook execution. Preview unless apply=true.", true, true, map[string]any{"object_id": text, "apply": apply}, "object_id")
	return tools
}
func typedArgument(args map[string]json.RawMessage, key string, dest any) error {
	b, ok := args[key]
	if !ok {
		return fmt.Errorf("argumento obrigatório: %s", key)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("argumento inválido: %s", key)
	}
	return nil
}
func (s *Server) callMCP(ctx context.Context, name string, args map[string]json.RawMessage) (any, error) {
	known := false
	for _, tool := range mcpTools() {
		if tool["name"] == name {
			known = true
			props := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
			for k := range args {
				if _, ok := props[k]; !ok {
					return nil, fmt.Errorf("argumento não suportado: %s", k)
				}
			}
			break
		}
	}
	if !known {
		return nil, fmt.Errorf("ferramenta desconhecida")
	}
	snap := s.Store.Snapshot()
	switch name {
	case "list_hosts":
		return s.workspaceRows("hosts", 60), nil
	case "list_services":
		return s.Store.APMServices(snap.GeneratedAt.Add(-24*time.Hour), snap.GeneratedAt), nil
	case "service_dependencies":
		return snap.ServiceMap.Edges, nil
	case "network_flows":
		return snap.NetworkMap, nil
	case "list_incidents":
		return snap.Events, nil
	case "list_frontends":
		return s.frontendCandidates(), nil
	case "list_databases":
		return s.databaseInventory(), nil
	case "run_query":
		var q string
		if e := typedArgument(args, "query", &q); e != nil {
			return nil, e
		}
		minutes := 60
		if raw, ok := args["minutes"]; ok {
			if json.Unmarshal(raw, &minutes) != nil {
				return nil, fmt.Errorf("janela inválida")
			}
		}
		return s.executeVQL(q, minutes)
	case "list_workspace":
		var kind string
		if raw, ok := args["kind"]; ok {
			if json.Unmarshal(raw, &kind) != nil {
				return nil, fmt.Errorf("tipo inválido")
			}
		}
		return s.Store.WorkspaceObjects(kind), nil
	case "list_operation_runs":
		var id string
		if raw, ok := args["object_id"]; ok {
			if json.Unmarshal(raw, &id) != nil {
				return nil, fmt.Errorf("ID inválido")
			}
		}
		return s.Store.OperationRuns(id), nil
	}
	apply := false
	if raw, ok := args["apply"]; ok {
		if json.Unmarshal(raw, &apply) != nil {
			return nil, fmt.Errorf("apply deve ser booleano")
		}
	}
	switch name {
	case "configure_workspace":
		var o model.WorkspaceObject
		if e := typedArgument(args, "object", &o); e != nil {
			return nil, e
		}
		if e := store.ValidateWorkspaceObject(o); e != nil {
			return nil, e
		}
		if !apply {
			return map[string]any{"preview": true, "object": o}, nil
		}
		if e := s.Store.PutWorkspace(o); e != nil {
			return nil, e
		}
		return map[string]bool{"saved": true}, nil
	case "configure_rum":
		var a model.RUMApplication
		if e := typedArgument(args, "application", &a); e != nil {
			return nil, e
		}
		if e := store.ValidateRUMApplication(a); e != nil {
			return nil, e
		}
		if !apply {
			return map[string]any{"preview": true, "application": a}, nil
		}
		if e := s.Store.PutRUMApplication(a); e != nil {
			return nil, e
		}
		return map[string]bool{"saved": true}, nil
	case "configure_host":
		var id string
		var settings store.InventoryEdit
		if e := typedArgument(args, "host_id", &id); e != nil {
			return nil, e
		}
		if e := typedArgument(args, "settings", &settings); e != nil {
			return nil, e
		}
		found := false
		for _, h := range snap.Hosts {
			if h.ID == id {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("host não encontrado")
		}
		if !apply {
			return map[string]any{"preview": true, "host_id": id, "settings": settings}, nil
		}
		if e := s.Store.EditInventory(id, settings); e != nil {
			return nil, e
		}
		return map[string]bool{"saved": true}, nil
	case "run_operation":
		var id string
		if e := typedArgument(args, "object_id", &id); e != nil {
			return nil, e
		}
		o, ok := s.Store.WorkspaceObject(id)
		if !ok || o.Kind == "notebook" {
			return nil, fmt.Errorf("operação não encontrada")
		}
		if !apply {
			return map[string]any{"preview": true, "object_id": id, "kind": o.Kind}, nil
		}
		return s.startOperation(ctx, id)
	}
	return nil, fmt.Errorf("ferramenta desconhecida")
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
		w.WriteHeader(202)
		return
	}
	var result any
	var rpcErr any
	switch req.Method {
	case "initialize":
		result = map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{"tools": map[string]bool{"listChanged": false}}, "serverInfo": map[string]string{"name": "vetra-observability", "version": "0.4.0"}, "instructions": "Administrative API-token authentication. Read tools query retained observations. Configuration tools preview unless apply=true; applications remain subject to the same validation as the UI. Do not put database passwords or API tokens in tool arguments."}
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result = map[string]any{"tools": mcpTools()}
	case "tools/call":
		var p struct {
			Name      string                     `json:"name"`
			Arguments map[string]json.RawMessage `json:"arguments"`
		}
		if json.Unmarshal(req.Params, &p) != nil {
			rpcErr = map[string]any{"code": -32602, "message": "invalid tool parameters"}
			break
		}
		data, e := s.callMCP(r.Context(), p.Name, p.Arguments)
		if e != nil {
			result = map[string]any{"isError": true, "content": []map[string]string{{"type": "text", "text": e.Error()}}}
			break
		}
		b, _ := json.Marshal(data)
		result = map[string]any{"content": []map[string]string{{"type": "text", "text": string(b)}}}
	default:
		rpcErr = map[string]any{"code": -32601, "message": "method not found"}
	}
	reply := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	if rpcErr != nil {
		reply["error"] = rpcErr
	} else {
		reply["result"] = result
	}
	write(w, 200, reply)
}
