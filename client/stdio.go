package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// StdioServerParameters represents configuration for the stdio server
type StdioServerParameters struct {
	Command string            `json:"command"`     // The executable to run to start the server
	Args    []string         `json:"args"`        // Command line arguments to pass to the executable
	Env     map[string]string `json:"env"`         // The environment to use when spawning the process
}

// JSONRPCMessage represents a JSON-RPC message
type JSONRPCMessage struct {
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	ID     interface{}     `json:"id,omitempty"`
	Result interface{}     `json:"result,omitempty"`
	Error  interface{}     `json:"error,omitempty"`
}

// MessageChan is a channel type for JSONRPCMessage
type MessageChan chan JSONRPCMessage

// StdioClient represents a client that communicates over stdio
type StdioClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewStdioClient creates a new stdio client with the given parameters
func NewStdioClient(ctx context.Context, params StdioServerParameters) (*StdioClient, MessageChan, MessageChan, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, params.Command, params.Args...)
	
	if params.Env != nil {
		cmd.Env = os.Environ() // Start with current environment
		for k, v := range params.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		stdin.Close()
		return nil, nil, nil, err
	}

	cmd.Stderr = os.Stderr

	readChan := make(MessageChan)
	writeChan := make(MessageChan)

	client := &StdioClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		ctx:     ctx,
		cancel:  cancel,
	}

	if err := cmd.Start(); err != nil {
		cancel()
		stdin.Close()
		stdout.Close()
		return nil, nil, nil, err
	}

	// Start goroutine for reading from stdout
	client.wg.Add(1)
	go func() {
		defer client.wg.Done()
		defer close(readChan)
		
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				var message JSONRPCMessage
				if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
					continue
				}
				select {
				case readChan <- message:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Start goroutine for writing to stdin
	client.wg.Add(1)
	go func() {
		defer client.wg.Done()
		defer close(writeChan)
		defer stdin.Close()
		
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-writeChan:
				if !ok {
					return
				}
				data, err := json.Marshal(message)
				if err != nil {
					continue
				}
				data = append(data, '\n')
				if _, err := stdin.Write(data); err != nil {
					return
				}
			}
		}
	}()

	return client, readChan, writeChan, nil
}

// Close closes the stdio client and waits for all goroutines to finish
func (c *StdioClient) Close() error {
	// Cancel context first to stop goroutines
	c.cancel()
	
	// Close stdin to signal EOF to the process
	if err := c.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	
	// Wait for goroutines to finish
	c.wg.Wait()
	
	// Kill the process and wait for it to finish
	c.cmd.Process.Kill()
	
	// Wait for the process to finish
	if err := c.cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return fmt.Errorf("failed to wait for process: %w", err)
		}
		// Ignore exit errors as we killed the process
	}
	
	return nil
}