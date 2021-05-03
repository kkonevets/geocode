package server

import (
	"context"

	pb "github.com/X-Company/geocoding/proto/geocoding"
	"google.golang.org/grpc"
)

type (
	// abstract handler for a single request
	handleOne func(ctx context.Context, s *GeocodingServer,
		query interface{}) (interface{}, error)

	// A wrapper of grpc.ServerStream to make server-side streaming of different types
	ServerStreamer interface {
		grpc.ServerStream
		Send(interface{}) error
	}

	// A wrapper of grpc.ServerStream to make bidirectional streaming of different types
	BiStreamer interface {
		grpc.ServerStream
		Send(interface{}) error
		Recv() (interface{}, error)
	}

	DecodeManyStream struct {
		grpc.ServerStream
		Stream pb.Geocoding_DecodeManyServer
	}

	EncodeManyStream struct {
		grpc.ServerStream
		Stream pb.Geocoding_EncodeManyServer
	}

	DecodeBiStream struct {
		grpc.ServerStream
		Stream pb.Geocoding_DecodeBiServer
	}

	EncodeBiStream struct {
		grpc.ServerStream
		Stream pb.Geocoding_EncodeBiServer
	}
)

func (s *DecodeManyStream) Send(addrs interface{}) error {
	return s.Stream.Send(addrs.(*pb.Addresses))
}

func (s *DecodeManyStream) Context() context.Context {
	return s.Stream.Context()
}

func (s *EncodeManyStream) Send(point interface{}) error {
	return s.Stream.Send(point.(*pb.Point))
}

func (s *EncodeManyStream) Context() context.Context {
	return s.Stream.Context()
}

func (s *DecodeBiStream) Send(addrs interface{}) error {
	return s.Stream.Send(addrs.(*pb.Addresses))
}

func (s *DecodeBiStream) Context() context.Context {
	return s.Stream.Context()
}

func (s *DecodeBiStream) Recv() (interface{}, error) {
	return s.Stream.Recv()
}

func (s *EncodeBiStream) Send(point interface{}) error {
	return s.Stream.Send(point.(*pb.Point))
}

func (s *EncodeBiStream) Context() context.Context {
	return s.Stream.Context()
}

func (s *EncodeBiStream) Recv() (interface{}, error) {
	return s.Stream.Recv()
}

func handleOneDecode(ctx context.Context, s *GeocodingServer, query interface{}) (interface{}, error) {
	res, err := s.decodeOne(ctx, query.(*pb.Point))
	if err != nil {
		res.Error = err.Error()
	}
	return res, err
}

func handleOneEncode(ctx context.Context, s *GeocodingServer, query interface{}) (interface{}, error) {
	res, err := s.encodeOne(ctx, query.(*pb.Address))
	if err != nil {
		res.Error = err.Error()
	}
	return res, err
}
