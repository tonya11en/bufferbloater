//Generated by gRPC Go plugin
//If you make any local changes, they will be lost
//source: bouncer

//nolint
package bouncer_flatbuf

import (
	context "context"

	flatbuffers "github.com/google/flatbuffers/go"
	grpc "google.golang.org/grpc"
)

// Client API for MetricsBuffer service
type MetricsBufferClient interface {
	PushRecord(ctx context.Context, in *flatbuffers.Builder,
		opts ...grpc.CallOption) (*PushRecordReply, error)
	PopRecord(ctx context.Context,
		opts ...grpc.CallOption) (MetricsBuffer_PopRecordClient, error)
}

type metricsBufferClient struct {
	cc *grpc.ClientConn
}

func NewMetricsBufferClient(cc *grpc.ClientConn) MetricsBufferClient {
	return &metricsBufferClient{cc}
}

func (c *metricsBufferClient) PushRecord(ctx context.Context, in *flatbuffers.Builder,
	opts ...grpc.CallOption) (*PushRecordReply, error) {
	out := new(PushRecordReply)
	err := grpc.Invoke(ctx, "/bouncer_flatbuf.MetricsBuffer/PushRecord", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *metricsBufferClient) PopRecord(ctx context.Context,
	opts ...grpc.CallOption) (MetricsBuffer_PopRecordClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_MetricsBuffer_serviceDesc.Streams[0], c.cc, "/bouncer_flatbuf.MetricsBuffer/PopRecord", opts...)
	if err != nil {
		return nil, err
	}
	x := &metricsBufferPopRecordClient{stream}
	return x, nil
}

type MetricsBuffer_PopRecordClient interface {
	Send(*flatbuffers.Builder) error
	Recv() (*PopRecordReply, error)
	grpc.ClientStream
}

type metricsBufferPopRecordClient struct {
	grpc.ClientStream
}

func (x *metricsBufferPopRecordClient) Send(m *flatbuffers.Builder) error {
	return x.ClientStream.SendMsg(m)
}

func (x *metricsBufferPopRecordClient) Recv() (*PopRecordReply, error) {
	m := new(PopRecordReply)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for MetricsBuffer service
type MetricsBufferServer interface {
	PushRecord(context.Context, *PushRecordRequest) (*flatbuffers.Builder, error)
	PopRecord(MetricsBuffer_PopRecordServer) error
}

func RegisterMetricsBufferServer(s *grpc.Server, srv MetricsBufferServer) {
	s.RegisterService(&_MetricsBuffer_serviceDesc, srv)
}

func _MetricsBuffer_PushRecord_Handler(srv interface{}, ctx context.Context,
	dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PushRecordRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetricsBufferServer).PushRecord(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/bouncer_flatbuf.MetricsBuffer/PushRecord",
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetricsBufferServer).PushRecord(ctx, req.(*PushRecordRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MetricsBuffer_PopRecord_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(MetricsBufferServer).PopRecord(&metricsBufferPopRecordServer{stream})
}

type MetricsBuffer_PopRecordServer interface {
	Send(*flatbuffers.Builder) error
	Recv() (*PopRecordRequest, error)
	grpc.ServerStream
}

type metricsBufferPopRecordServer struct {
	grpc.ServerStream
}

func (x *metricsBufferPopRecordServer) Send(m *flatbuffers.Builder) error {
	return x.ServerStream.SendMsg(m)
}

func (x *metricsBufferPopRecordServer) Recv() (*PopRecordRequest, error) {
	m := new(PopRecordRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _MetricsBuffer_serviceDesc = grpc.ServiceDesc{
	ServiceName: "bouncer_flatbuf.MetricsBuffer",
	HandlerType: (*MetricsBufferServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "PushRecord",
			Handler:    _MetricsBuffer_PushRecord_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "PopRecord",
			Handler:       _MetricsBuffer_PopRecord_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
}