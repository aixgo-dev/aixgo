package proto

import (
	"context"

	"google.golang.org/grpc"
)

// Stub types for DistributedRuntime gRPC service
// TODO: Replace with generated protobuf code

// ExecuteRequest represents a request to execute an agent
type ExecuteRequest struct {
	AgentName string
	Input     *Message

	// Session context (optional)
	SessionID      string          // Session ID to use
	IncludeHistory bool            // Whether to include conversation history
	SessionContext *SessionContext // Full session context if needed
}

// ExecuteResponse represents the response from agent execution
type ExecuteResponse struct {
	Output *Message

	// Session updates (if session was used)
	SessionUpdate *SessionUpdate
}

// SessionContext contains session information passed in requests
type SessionContext struct {
	ID        string
	UserID    string
	AgentName string
	History   []*Message // Conversation history
}

// SessionUpdate contains session changes from agent execution
type SessionUpdate struct {
	NewEntries   []*SessionEntry
	MessageCount int32
}

// SessionEntry represents a session log entry
type SessionEntry struct {
	ID        string
	ParentID  string
	Timestamp int64
	Type      string
	Data      []byte
}

// ListenRequest represents a request to listen for messages from an agent
type ListenRequest struct {
	AgentName string
}

// ListenResponse wraps a message for streaming
type ListenResponse struct {
	Message *Message
}

// SendRequest represents a request to send a message to an agent
type SendRequest struct {
	Target  string
	Message *Message
}

// SendResponse represents the response from sending a message
type SendResponse struct {
	Success bool
}

// AgentServiceClient is the client interface for the agent service
type AgentServiceClient interface {
	Execute(ctx context.Context, in *ExecuteRequest, opts ...grpc.CallOption) (*ExecuteResponse, error)
	Send(ctx context.Context, in *SendRequest, opts ...grpc.CallOption) (*SendResponse, error)
	Listen(ctx context.Context, in *ListenRequest, opts ...grpc.CallOption) (AgentService_ListenClient, error)
}

// AgentService_ListenClient is the client interface for the Listen streaming RPC
type AgentService_ListenClient interface {
	Recv() (*ListenResponse, error)
	grpc.ClientStream
}

// agentServiceClient implements AgentServiceClient
type agentServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewAgentServiceClient creates a new AgentServiceClient
func NewAgentServiceClient(cc grpc.ClientConnInterface) AgentServiceClient {
	return &agentServiceClient{cc}
}

func (c *agentServiceClient) Execute(ctx context.Context, in *ExecuteRequest, opts ...grpc.CallOption) (*ExecuteResponse, error) {
	out := new(ExecuteResponse)
	err := c.cc.Invoke(ctx, "/aixgo.AgentService/Execute", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentServiceClient) Send(ctx context.Context, in *SendRequest, opts ...grpc.CallOption) (*SendResponse, error) {
	out := new(SendResponse)
	err := c.cc.Invoke(ctx, "/aixgo.AgentService/Send", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentServiceClient) Listen(ctx context.Context, in *ListenRequest, opts ...grpc.CallOption) (AgentService_ListenClient, error) {
	stream, err := c.cc.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "Listen",
		ServerStreams: true,
	}, "/aixgo.AgentService/Listen", opts...)
	if err != nil {
		return nil, err
	}
	x := &agentServiceListenClient{stream}
	if err := x.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type agentServiceListenClient struct {
	grpc.ClientStream
}

func (x *agentServiceListenClient) Recv() (*ListenResponse, error) {
	m := new(ListenResponse)
	if err := x.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// AgentServiceServer is the server interface for the agent service
type AgentServiceServer interface {
	Execute(context.Context, *ExecuteRequest) (*ExecuteResponse, error)
	Send(context.Context, *SendRequest) (*SendResponse, error)
	Listen(*ListenRequest, AgentService_ListenServer) error
}

// AgentService_ListenServer is the server interface for the Listen streaming RPC
type AgentService_ListenServer interface {
	Send(*ListenResponse) error
	grpc.ServerStream
}

// UnimplementedAgentServiceServer provides default implementations
type UnimplementedAgentServiceServer struct{}

func (UnimplementedAgentServiceServer) Execute(context.Context, *ExecuteRequest) (*ExecuteResponse, error) {
	return nil, nil
}

func (UnimplementedAgentServiceServer) Send(context.Context, *SendRequest) (*SendResponse, error) {
	return nil, nil
}

func (UnimplementedAgentServiceServer) Listen(*ListenRequest, AgentService_ListenServer) error {
	return nil
}

// _AgentService_Execute_Handler is the handler for the Execute RPC method
func _AgentService_Execute_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ExecuteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServiceServer).Execute(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/aixgo.AgentService/Execute",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServiceServer).Execute(ctx, req.(*ExecuteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// _AgentService_Send_Handler is the handler for the Send RPC method
func _AgentService_Send_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServiceServer).Send(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/aixgo.AgentService/Send",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServiceServer).Send(ctx, req.(*SendRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// _AgentService_Listen_Handler is the handler for the Listen streaming RPC method
func _AgentService_Listen_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ListenRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AgentServiceServer).Listen(m, &agentServiceListenServer{stream})
}

type agentServiceListenServer struct {
	grpc.ServerStream
}

func (x *agentServiceListenServer) Send(m *ListenResponse) error {
	return x.SendMsg(m)
}

// RegisterAgentServiceServer registers the agent service with gRPC
func RegisterAgentServiceServer(s grpc.ServiceRegistrar, srv AgentServiceServer) {
	// Stub implementation - would be generated by protoc
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "aixgo.AgentService",
		HandlerType: (*AgentServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "Execute",
				Handler:    _AgentService_Execute_Handler,
			},
			{
				MethodName: "Send",
				Handler:    _AgentService_Send_Handler,
			},
		},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "Listen",
				Handler:       _AgentService_Listen_Handler,
				ServerStreams: true,
			},
		},
		Metadata: "agent_service.proto",
	}, srv)
}
