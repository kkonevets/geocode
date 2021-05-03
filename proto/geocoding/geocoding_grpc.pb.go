// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package geocoding

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// GeocodingClient is the client API for Geocoding service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type GeocodingClient interface {
	// returns point addresses (in multiple languages)
	Decode(ctx context.Context, in *Point, opts ...grpc.CallOption) (*Addresses, error)
	// returns lat/lon of an address
	Encode(ctx context.Context, in *Address, opts ...grpc.CallOption) (*Point, error)
	// returns addresses (in multiple languages) for each point in a stream
	DecodeMany(ctx context.Context, in *Points, opts ...grpc.CallOption) (Geocoding_DecodeManyClient, error)
	// returns point for each address in a stream
	EncodeMany(ctx context.Context, in *Addresses, opts ...grpc.CallOption) (Geocoding_EncodeManyClient, error)
	// same as DecodeMany, but using bidirectional stream
	DecodeBi(ctx context.Context, opts ...grpc.CallOption) (Geocoding_DecodeBiClient, error)
	// same as EncodeMany, but using bidirectional stream
	EncodeBi(ctx context.Context, opts ...grpc.CallOption) (Geocoding_EncodeBiClient, error)
}

type geocodingClient struct {
	cc grpc.ClientConnInterface
}

func NewGeocodingClient(cc grpc.ClientConnInterface) GeocodingClient {
	return &geocodingClient{cc}
}

func (c *geocodingClient) Decode(ctx context.Context, in *Point, opts ...grpc.CallOption) (*Addresses, error) {
	out := new(Addresses)
	err := c.cc.Invoke(ctx, "/geocoding.Geocoding/Decode", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *geocodingClient) Encode(ctx context.Context, in *Address, opts ...grpc.CallOption) (*Point, error) {
	out := new(Point)
	err := c.cc.Invoke(ctx, "/geocoding.Geocoding/Encode", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *geocodingClient) DecodeMany(ctx context.Context, in *Points, opts ...grpc.CallOption) (Geocoding_DecodeManyClient, error) {
	stream, err := c.cc.NewStream(ctx, &Geocoding_ServiceDesc.Streams[0], "/geocoding.Geocoding/DecodeMany", opts...)
	if err != nil {
		return nil, err
	}
	x := &geocodingDecodeManyClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Geocoding_DecodeManyClient interface {
	Recv() (*Addresses, error)
	grpc.ClientStream
}

type geocodingDecodeManyClient struct {
	grpc.ClientStream
}

func (x *geocodingDecodeManyClient) Recv() (*Addresses, error) {
	m := new(Addresses)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *geocodingClient) EncodeMany(ctx context.Context, in *Addresses, opts ...grpc.CallOption) (Geocoding_EncodeManyClient, error) {
	stream, err := c.cc.NewStream(ctx, &Geocoding_ServiceDesc.Streams[1], "/geocoding.Geocoding/EncodeMany", opts...)
	if err != nil {
		return nil, err
	}
	x := &geocodingEncodeManyClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Geocoding_EncodeManyClient interface {
	Recv() (*Point, error)
	grpc.ClientStream
}

type geocodingEncodeManyClient struct {
	grpc.ClientStream
}

func (x *geocodingEncodeManyClient) Recv() (*Point, error) {
	m := new(Point)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *geocodingClient) DecodeBi(ctx context.Context, opts ...grpc.CallOption) (Geocoding_DecodeBiClient, error) {
	stream, err := c.cc.NewStream(ctx, &Geocoding_ServiceDesc.Streams[2], "/geocoding.Geocoding/DecodeBi", opts...)
	if err != nil {
		return nil, err
	}
	x := &geocodingDecodeBiClient{stream}
	return x, nil
}

type Geocoding_DecodeBiClient interface {
	Send(*Point) error
	Recv() (*Addresses, error)
	grpc.ClientStream
}

type geocodingDecodeBiClient struct {
	grpc.ClientStream
}

func (x *geocodingDecodeBiClient) Send(m *Point) error {
	return x.ClientStream.SendMsg(m)
}

func (x *geocodingDecodeBiClient) Recv() (*Addresses, error) {
	m := new(Addresses)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *geocodingClient) EncodeBi(ctx context.Context, opts ...grpc.CallOption) (Geocoding_EncodeBiClient, error) {
	stream, err := c.cc.NewStream(ctx, &Geocoding_ServiceDesc.Streams[3], "/geocoding.Geocoding/EncodeBi", opts...)
	if err != nil {
		return nil, err
	}
	x := &geocodingEncodeBiClient{stream}
	return x, nil
}

type Geocoding_EncodeBiClient interface {
	Send(*Address) error
	Recv() (*Point, error)
	grpc.ClientStream
}

type geocodingEncodeBiClient struct {
	grpc.ClientStream
}

func (x *geocodingEncodeBiClient) Send(m *Address) error {
	return x.ClientStream.SendMsg(m)
}

func (x *geocodingEncodeBiClient) Recv() (*Point, error) {
	m := new(Point)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// GeocodingServer is the server API for Geocoding service.
// All implementations must embed UnimplementedGeocodingServer
// for forward compatibility
type GeocodingServer interface {
	// returns point addresses (in multiple languages)
	Decode(context.Context, *Point) (*Addresses, error)
	// returns lat/lon of an address
	Encode(context.Context, *Address) (*Point, error)
	// returns addresses (in multiple languages) for each point in a stream
	DecodeMany(*Points, Geocoding_DecodeManyServer) error
	// returns point for each address in a stream
	EncodeMany(*Addresses, Geocoding_EncodeManyServer) error
	// same as DecodeMany, but using bidirectional stream
	DecodeBi(Geocoding_DecodeBiServer) error
	// same as EncodeMany, but using bidirectional stream
	EncodeBi(Geocoding_EncodeBiServer) error
	mustEmbedUnimplementedGeocodingServer()
}

// UnimplementedGeocodingServer must be embedded to have forward compatible implementations.
type UnimplementedGeocodingServer struct {
}

func (UnimplementedGeocodingServer) Decode(context.Context, *Point) (*Addresses, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Decode not implemented")
}
func (UnimplementedGeocodingServer) Encode(context.Context, *Address) (*Point, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Encode not implemented")
}
func (UnimplementedGeocodingServer) DecodeMany(*Points, Geocoding_DecodeManyServer) error {
	return status.Errorf(codes.Unimplemented, "method DecodeMany not implemented")
}
func (UnimplementedGeocodingServer) EncodeMany(*Addresses, Geocoding_EncodeManyServer) error {
	return status.Errorf(codes.Unimplemented, "method EncodeMany not implemented")
}
func (UnimplementedGeocodingServer) DecodeBi(Geocoding_DecodeBiServer) error {
	return status.Errorf(codes.Unimplemented, "method DecodeBi not implemented")
}
func (UnimplementedGeocodingServer) EncodeBi(Geocoding_EncodeBiServer) error {
	return status.Errorf(codes.Unimplemented, "method EncodeBi not implemented")
}
func (UnimplementedGeocodingServer) mustEmbedUnimplementedGeocodingServer() {}

// UnsafeGeocodingServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to GeocodingServer will
// result in compilation errors.
type UnsafeGeocodingServer interface {
	mustEmbedUnimplementedGeocodingServer()
}

func RegisterGeocodingServer(s grpc.ServiceRegistrar, srv GeocodingServer) {
	s.RegisterService(&Geocoding_ServiceDesc, srv)
}

func _Geocoding_Decode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Point)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GeocodingServer).Decode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/geocoding.Geocoding/Decode",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GeocodingServer).Decode(ctx, req.(*Point))
	}
	return interceptor(ctx, in, info, handler)
}

func _Geocoding_Encode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Address)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GeocodingServer).Encode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/geocoding.Geocoding/Encode",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GeocodingServer).Encode(ctx, req.(*Address))
	}
	return interceptor(ctx, in, info, handler)
}

func _Geocoding_DecodeMany_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Points)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(GeocodingServer).DecodeMany(m, &geocodingDecodeManyServer{stream})
}

type Geocoding_DecodeManyServer interface {
	Send(*Addresses) error
	grpc.ServerStream
}

type geocodingDecodeManyServer struct {
	grpc.ServerStream
}

func (x *geocodingDecodeManyServer) Send(m *Addresses) error {
	return x.ServerStream.SendMsg(m)
}

func _Geocoding_EncodeMany_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Addresses)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(GeocodingServer).EncodeMany(m, &geocodingEncodeManyServer{stream})
}

type Geocoding_EncodeManyServer interface {
	Send(*Point) error
	grpc.ServerStream
}

type geocodingEncodeManyServer struct {
	grpc.ServerStream
}

func (x *geocodingEncodeManyServer) Send(m *Point) error {
	return x.ServerStream.SendMsg(m)
}

func _Geocoding_DecodeBi_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(GeocodingServer).DecodeBi(&geocodingDecodeBiServer{stream})
}

type Geocoding_DecodeBiServer interface {
	Send(*Addresses) error
	Recv() (*Point, error)
	grpc.ServerStream
}

type geocodingDecodeBiServer struct {
	grpc.ServerStream
}

func (x *geocodingDecodeBiServer) Send(m *Addresses) error {
	return x.ServerStream.SendMsg(m)
}

func (x *geocodingDecodeBiServer) Recv() (*Point, error) {
	m := new(Point)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Geocoding_EncodeBi_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(GeocodingServer).EncodeBi(&geocodingEncodeBiServer{stream})
}

type Geocoding_EncodeBiServer interface {
	Send(*Point) error
	Recv() (*Address, error)
	grpc.ServerStream
}

type geocodingEncodeBiServer struct {
	grpc.ServerStream
}

func (x *geocodingEncodeBiServer) Send(m *Point) error {
	return x.ServerStream.SendMsg(m)
}

func (x *geocodingEncodeBiServer) Recv() (*Address, error) {
	m := new(Address)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Geocoding_ServiceDesc is the grpc.ServiceDesc for Geocoding service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Geocoding_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "geocoding.Geocoding",
	HandlerType: (*GeocodingServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Decode",
			Handler:    _Geocoding_Decode_Handler,
		},
		{
			MethodName: "Encode",
			Handler:    _Geocoding_Encode_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "DecodeMany",
			Handler:       _Geocoding_DecodeMany_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "EncodeMany",
			Handler:       _Geocoding_EncodeMany_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "DecodeBi",
			Handler:       _Geocoding_DecodeBi_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "EncodeBi",
			Handler:       _Geocoding_EncodeBi_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "geocoding.proto",
}