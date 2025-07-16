package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"
	pb "github.com/metoro-io/mcp-golang/transport/grpc/protogen/jsonrpc"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var CLI struct {
	Address string `default:"localhost:50051" help:"Address of the gRPC server to connect to"`
}

func main() {
	kong.Parse(&CLI)
	conn, err := grpc.NewClient(CLI.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewJSONRPCServiceClient(conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.Transport(ctx)
	if err != nil {
		log.Fatalf("could not open stream: %v", err)
	}

	// Handle Ctrl+C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
		os.Exit(0)
	}()

	// Goroutine to read from stdin and send to server
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Bytes()
			var msg pb.GenericJSONRPCMessage
			if err := json.Unmarshal(line, &msg); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid input: %v\n", err)
				continue
			}

			typedId, err := getId(line)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get ID: %v\n", err)
				continue
			}

			msg.TypedId = typedId
			// fmt.Printf("SENDING: %v\n", msg)
			if err := stream.Send(&msg); err != nil {
				fmt.Fprintf(os.Stderr, "Send error: %v\n", err)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Scanner error: %v\n", err)
		}
		// time.Sleep(5 * time.Second)
		// stream.CloseSend()
	}()

	// Read responses from server and print to stdout
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Receive error: %v\n", err)
			break
		}
		b, err := parseResp(resp)
		// b, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Marshal error: %v\n", err)
			continue
		}
		fmt.Println(string(b))
	}

	// Give time for goroutines to finish
	// time.Sleep(5 * time.Second)
}

func getId(line []byte) (*pb.ID, error) {
	var id *pb.ID
	msg := &struct {
		ID any `json:"id,omitempty"`
	}{}

	if err := json.Unmarshal(line, &msg); err != nil {
		fmt.Fprintf(os.Stderr, "Unmarshal error: %v\n", err)
		return id, err
	}

	switch v := msg.ID.(type) {
	case string:
		id = &pb.ID{Kind: &pb.ID_Str{Str: v}}
	case int:
		id = &pb.ID{Kind: &pb.ID_Num{Num: int64(v)}}
	case float64:
		id = &pb.ID{Kind: &pb.ID_Num{Num: int64(v)}}
	case nil:
		return nil, nil
	default:
		return id, fmt.Errorf("failed to infer 'id' type: '%T:%v'", v, v)
	}

	return id, nil
}

func parseResp(m *pb.GenericJSONRPCMessage) ([]byte, error) {
	switch v := m.TypedId.GetKind().(type) {
	case *pb.ID_Num:
		type respType struct {
			ID int64 `json:"id"`
			*pb.GenericJSONRPCMessage
		}

		resp := respType{GenericJSONRPCMessage: m}
		resp.ID = int64(v.Num)
		resp.TypedId = nil

		return json.Marshal(resp)
	case *pb.ID_Str:
		type respType struct {
			ID string `json:"id"`
			*pb.GenericJSONRPCMessage
		}

		resp := respType{GenericJSONRPCMessage: m}
		resp.ID = v.Str
		resp.TypedId = nil

		return json.Marshal(resp)
	default:
		return nil, fmt.Errorf("unsupported 'id' type in response: '%T:%v'", v, v)
	}
}
