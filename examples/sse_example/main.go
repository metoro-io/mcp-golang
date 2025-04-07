package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/sse"
)

type Content struct {
	Title       string  `json:"title" jsonschema:"required,description=The title to submit"`
	Description *string `json:"description" jsonschema:"description=The description to submit"`
}
type MyFunctionsArguments struct {
	Submitter string  `json:"submitter" jsonschema:"required,description=The name of the thing calling this tool (openai, google, claude, etc)"`
	Content   Content `json:"content" jsonschema:"required,description=The content of the message"`
}

type HttpHandler struct{}

var (
	postEndpoint    = "/sse"
	sessionList     = make(map[string]*sse.SSEServerTransport)
	sessionListLock = sync.RWMutex{} // Uncomment if you need to protect sessionList from concurrent access
)

func (h *HttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request:", r.Method, r.URL.Path)
	// Check if the request is a POST request to the SSE endpoint
	if r.Method == http.MethodPost && r.URL.Path == postEndpoint {
		// get the session ID from the request header or URL parameters
		sessionID := r.Header.Get("session")
		if sessionID == "" {
			// if not found in header, try to get it from URL parameters
			sessionID = r.URL.Query().Get("sessionId")
		}
		if sessionID == "" {
			http.Error(w, "Session ID is required", http.StatusBadRequest)
			return
		}
		// Check if the session ID already exists in the session list
		sessionListLock.RLock()
		transport, exists := sessionList[sessionID]
		sessionListLock.RUnlock()
		if exists {
			// If it exists, reuse the existing transport
			log.Println("Reusing existing session:", sessionID)
			transport.HandlePostMessage(r)
			// response with 202 Accepted
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Accepted"))
			return
		}
	}

	transport, err := sse.NewSSEServerTransport(postEndpoint, w)
	if err != nil {
		panic(err)
	}
	// process the incoming POST request
	server := mcp_golang.NewServer(transport)
	// 自定义一个http handler来处理http请求
	err = server.RegisterTool("hello", "Say hello to a person", func(arguments MyFunctionsArguments) (*mcp_golang.ToolResponse, error) {
		log.Println("Received request:", arguments)
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Hello, %server!", arguments.Submitter))), nil
	})
	if err != nil {
		panic(err)
	}

	err = server.Serve()
	if err != nil {
		panic(err)
	}
	sessionListLock.Lock()
	sessionList[transport.SessionID()] = transport
	sessionListLock.Unlock()
	<-r.Context().Done()
}

func main() {
	// Create a new HTTP server
	handler := &http.Server{
		Addr:    ":8080",
		Handler: &HttpHandler{},
	}

	// Start the server
	fmt.Println("Starting server on :8080")
	if err := handler.ListenAndServe(); err != nil {
		panic(err)
	}
}
