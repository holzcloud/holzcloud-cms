package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

// The Model Context Protocol, as much of it as an assistant needs.
//
// It is JSON-RPC 2.0 over one HTTP address. A client opens with "initialize",
// asks "tools/list" for what is on offer, and then sends "tools/call" for each
// thing it wants done. That is the whole conversation; the rest of the
// specification — sampling, prompts, resource subscriptions — is for servers
// that push, and this one only answers.
//
// Written out rather than pulled in. A dependency here would be one more thing
// that has to survive a static cross-compile without cgo, one more thing to
// keep current, and one more place where a protocol detail is somebody else's
// opinion. What is here is small enough to read in one sitting.

// ProtocolVersion is the revision of MCP this speaks. A client that wants
// another one is told what it gets rather than being refused: the parts used
// here have been stable across revisions, and refusing would break a working
// assistant over a date.
const ProtocolVersion = "2025-06-18"

// MaxRequestBytes bounds one call.
//
// A page's text arrives through here, so it cannot be small — but an assistant
// that goes wrong should not be able to push a hundred megabytes into the
// server's memory before anything looks at it.
const MaxRequestBytes = 4 << 20

// rpcRequest is one JSON-RPC message.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse is the answer. Result and Error are mutually exclusive.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC error codes, as the specification numbers them.
const (
	codeParse       = -32700
	codeInvalidReq  = -32600
	codeNoSuchThing = -32601
	codeBadParams   = -32602
	codeInternal    = -32603
)

// Tool is one thing an assistant can do.
type Tool struct {
	Name string `json:"name"`
	// Description is read by the assistant when it decides what to call, and by
	// the operator when they wonder what they just allowed. It is written for
	// both: plainly, and saying what the thing does rather than what it is.
	Description string `json:"description"`
	InputSchema Schema `json:"inputSchema"`
	// Writes marks a tool that changes something. It is not part of the
	// protocol; it is what the read-only key is checked against, in one place,
	// so no tool can forget to ask.
	Writes bool                        `json:"-"`
	Run    func(ctx Call) (any, error) `json:"-"`
}

// Schema is a small JSON Schema — enough for what these tools take.
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property is one argument.
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// Server answers MCP over HTTP.
type Server struct {
	tokens *Store
	tools  map[string]Tool
	log    *slog.Logger
	// name is what the assistant shows the operator as the connection's name.
	name string
}

// NewServer wires the tools up.
func NewServer(tokens *Store, name string, log *slog.Logger, tools []Tool) *Server {
	if log == nil {
		log = slog.Default()
	}
	byName := make(map[string]Tool, len(tools))
	for _, t := range tools {
		byName[t.Name] = t
	}
	return &Server{tokens: tokens, tools: byName, log: log, name: name}
}

// ServeHTTP handles one JSON-RPC message.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A browser must never be able to reach this by accident, so no cookies are
	// read and no cross-origin headers are offered. The only way in is a header
	// that a form cannot set.
	w.Header().Set("Cache-Control", "no-store")

	if r.Method != http.MethodPost {
		// GET is what a client tries when it expects a server-sent-event
		// stream. Saying so beats a 404 that looks like a wrong address.
		http.Error(w, "Diese Adresse nimmt nur POST mit JSON-RPC entgegen.", http.StatusMethodNotAllowed)
		return
	}

	scope, err := s.authenticate(r)
	if err != nil {
		// The same answer for a missing and for a wrong key: telling the
		// difference is telling someone whether they guessed a real one.
		w.Header().Set("WWW-Authenticate", `Bearer realm="Holzcloud"`)
		http.Error(w, "Zugang verweigert", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBytes))
	if err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{codeParse, "Anfrage nicht lesbar"}})
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{codeParse, "kein gültiges JSON"}})
		return
	}
	if req.JSONRPC != "2.0" {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: req.ID,
			Error: &rpcError{codeInvalidReq, "erwartet wird JSON-RPC 2.0"}})
		return
	}

	// A notification has no id and wants no answer at all. "initialized" is one,
	// and answering it makes some clients complain.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	s.tokens.Touch(r.Context(), scope.TokenID)
	writeRPC(w, s.dispatch(r, scope, req))
}

// authenticate reads the bearer token.
func (s *Server) authenticate(r *http.Request) (Scope, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return Scope{}, ErrNoToken
	}
	return s.tokens.Verify(r.Context(), strings.TrimSpace(header[len("bearer "):]))
}

// dispatch answers one method.
func (s *Server) dispatch(r *http.Request, scope Scope, req rpcRequest) rpcResponse {
	answer := func(result any) rpcResponse {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	}
	fail := func(code int, msg string) rpcResponse {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{code, msg}}
	}

	switch req.Method {
	case "initialize":
		return answer(map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.name, "version": "1"},
			// Shown by some clients before the first call. It is the place to
			// say the one thing an assistant should know before it starts.
			"instructions": "Dies ist ein Holzcloud-CMS. Neue Seiten entstehen als Entwurf; " +
				"veröffentliche nur, wenn du ausdrücklich darum gebeten wurdest.",
		})

	case "ping":
		return answer(map[string]any{})

	case "tools/list":
		names := make([]string, 0, len(s.tools))
		for name := range s.tools {
			names = append(names, name)
		}
		sort.Strings(names)

		list := make([]Tool, 0, len(names))
		for _, name := range names {
			t := s.tools[name]
			// A read-only key is not offered what it cannot use. An assistant
			// that sees a tool and is then refused will try again, differently,
			// and report a failure the operator has to decode.
			if t.Writes && !scope.CanWrite {
				continue
			}
			list = append(list, t)
		}
		return answer(map[string]any{"tools": list})

	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return fail(codeBadParams, "die Aufrufparameter sind nicht lesbar")
		}
		tool, ok := s.tools[params.Name]
		if !ok {
			return fail(codeNoSuchThing, fmt.Sprintf("das Werkzeug %q gibt es nicht", params.Name))
		}
		if tool.Writes {
			if err := scope.MayWrite(); err != nil {
				return answer(toolError(err.Error()))
			}
		}

		out, err := tool.Run(Call{
			Ctx: r.Context(), Scope: scope, Args: params.Arguments, Log: s.log,
		})
		if err != nil {
			// A failing tool answers inside the result rather than as a
			// protocol error: the assistant is meant to read the reason and try
			// something else, and a JSON-RPC error is a signal that the
			// connection itself went wrong.
			s.log.Warn("ai tool failed", "tool", params.Name, "key", scope.Name, "err", err)
			return answer(toolError(err.Error()))
		}
		return answer(toolResult(out))
	}
	return fail(codeNoSuchThing, fmt.Sprintf("die Methode %q gibt es nicht", req.Method))
}

// Call is what a tool is handed.
type Call struct {
	Ctx   context.Context
	Scope Scope
	Args  json.RawMessage
	Log   *slog.Logger
}

// Into unmarshals the arguments, or says plainly what was wrong.
func (c Call) Into(v any) error {
	if len(c.Args) == 0 {
		return nil
	}
	if err := json.Unmarshal(c.Args, v); err != nil {
		return fmt.Errorf("die Angaben sind nicht lesbar: %w", err)
	}
	return nil
}

// toolResult wraps a value the way MCP expects: a list of content blocks.
//
// JSON in a text block rather than a structured result. Every client renders
// text, structured output is newer than some of them, and an assistant reads
// JSON perfectly well either way.
func toolResult(v any) map[string]any {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolError("die Antwort lässt sich nicht darstellen")
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(raw)}},
	}
}

func toolError(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": msg}},
		"isError": true,
	}
}

func writeRPC(w http.ResponseWriter, res rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
