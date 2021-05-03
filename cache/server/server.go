// server for caching external http requests
package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/X-Company/geocoding/config"
	pb "github.com/X-Company/geocoding/proto/geocoding"
	"github.com/X-Company/geocoding/stat"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// grpc Server
type GeocodingServer struct {
	pb.UnimplementedGeocodingServer
	dbx                *sqlx.DB         // sql client
	client             *http.Client     // http request client
	statC              chan *stat.Point // channel for statistics
	MaxOpenConnections int              // max number of concurrent connections
}

func HttpClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100

	client := &http.Client{
		Transport: t,
		Timeout:   10 * time.Second,
	}
	return client
}

func newServer(conf *config.Config) *GeocodingServer {
	dbx := sqlx.MustConnect("postgres", conf.Cache.ConnectionParams)
	dbx.SetMaxOpenConns(conf.Cache.MaxOpenConnections) // max number of parallel sql queries
	dbx.SetMaxIdleConns(conf.Cache.MaxOpenConnections) // max number of parallel sql queries

	s := &GeocodingServer{dbx: dbx, client: HttpClient(),
		// create buffered channel for statistics,
		// assume maximum 50 nonblocking statistical measurements (sends) per connection
		statC:              make(chan *stat.Point, 50*conf.Cache.MaxOpenConnections),
		MaxOpenConnections: conf.Cache.MaxOpenConnections}
	go stat.Serve(s.statC, "geocache") // start goroutine for writing statistics

	return s
}

func StartServer() {
	conf := config.MustLoad()
	lis, err := net.Listen("tcp", conf.GRPC.Host)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterGeocodingServer(grpcServer, newServer(conf))
	grpcServer.Serve(lis)
}

// Get point address
func (s *GeocodingServer) Decode(ctx context.Context, point *pb.Point) (*pb.Addresses, error) {
	s.statC <- &stat.Point{stat.Decode: 1}
	return s.decodeOne(ctx, point)
}

// Get lat/lon coordinates of an address
func (s *GeocodingServer) Encode(ctx context.Context, adr *pb.Address) (*pb.Point, error) {
	s.statC <- &stat.Point{stat.Encode: 1}
	return s.encodeOne(ctx, adr)
}

// Server-side streaming
// Decodes points to a group of addresses in different languages.
// Each address group is associated with a point by pb.Point.Id = pb.Addresses.Id
func (s *GeocodingServer) DecodeMany(points *pb.Points, stream pb.Geocoding_DecodeManyServer) error {
	s.statC <- &stat.Point{stat.DecodeMany: 1}
	queries := make([]interface{}, len(points.Items))
	for i, v := range points.Items {
		queries[i] = v
	}

	return s.HandleMany(queries, &DecodeManyStream{Stream: stream}, handleOneDecode)
}

// Server-side streaming
// Encodes address to a lat/lon point. Each address is assosiated with a point by pb.Point.Id = pb.Address.Id
func (s *GeocodingServer) EncodeMany(addrs *pb.Addresses, stream pb.Geocoding_EncodeManyServer) error {
	s.statC <- &stat.Point{stat.EncodeMany: 1}
	queries := make([]interface{}, len(addrs.Items))
	for i, v := range addrs.Items {
		queries[i] = v
	}

	return s.HandleMany(queries, &EncodeManyStream{Stream: stream}, handleOneEncode)
}

// Bidirectional streaming
// Decodes points to a group of addresses in different languages.
// Each address group is associated with a point by pb.Point.Id = pb.Addresses.Id
func (s *GeocodingServer) DecodeBi(stream pb.Geocoding_DecodeBiServer) error {
	s.statC <- &stat.Point{stat.DecodeBi: 1}
	return s.HandleBi(&DecodeBiStream{Stream: stream}, handleOneDecode)
}

// Bidirectional streaming
// Encodes address to a lat/lon point. Each address is assosiated with a point by pb.Point.Id = pb.Address.Id
func (s *GeocodingServer) EncodeBi(stream pb.Geocoding_EncodeBiServer) error {
	s.statC <- &stat.Point{stat.EncodeBi: 1}
	return s.HandleBi(&EncodeBiStream{Stream: stream}, handleOneEncode)
}

///////////////////////////////////////////////////////////////////////////////
//                                 Task Arena                                //
///////////////////////////////////////////////////////////////////////////////

// Limits number of parallel tasks, executes them and delivers result via channel
type TaskArena struct {
	// counting semaphore used to enforce a limit of MaxOpenConnections concurrent requests.
	semaphore chan struct{}
	handle    handleOne        // function representing a work
	dbx       *sqlx.DB         // sql connection
	done      chan interface{} // holds results of handleOne
	wg        sync.WaitGroup   // ensures all tasks are complete
}

func (s *GeocodingServer) NewTaskArena(handle handleOne) *TaskArena {
	a := &TaskArena{
		semaphore: make(chan struct{}, s.MaxOpenConnections),
		handle:    handle,
		dbx:       s.dbx,
		done:      make(chan interface{}),
		wg:        sync.WaitGroup{},
	}
	return a
}

// abstract worker - write and read from a buffered channel
func (a *TaskArena) Work(ctx context.Context, s *GeocodingServer, query interface{}) {
	defer a.wg.Done()
	var res interface{}

	a.semaphore <- struct{}{} // acquire
	select {
	case <-ctx.Done():
		res = nil
	default:
		res, _ = a.handle(ctx, s, query)
	}
	<-a.semaphore // release

	a.done <- res
}

///////////////////////////////////////////////////////////////////////////////
//                               Stream Drivers                              //
///////////////////////////////////////////////////////////////////////////////

// Handles batches of requests of different types in parallel, server streaming
// Number of parallel workers is defined by MaxOpenConnections
func (s *GeocodingServer) HandleMany(queries []interface{}, stream ServerStreamer, handle handleOne) error {
	arena := s.NewTaskArena(handle)
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel() // cancel when we are finished with queries

	for _, query := range queries {
		arena.wg.Add(1)              // need to use Add here to satisfy general algorithm
		go arena.Work(ctx, s, query) // launch worker for each query
	}

	var err error
	for range queries {
		res := <-arena.done           // drain the channel
		if res != nil && err == nil { // Send if no error
			if err = stream.Send(res); err != nil {
				cancel() // cancel pending workers
				log.Errorf("%s: HandleMany: %s", ctxAddr(ctx), err)
			}
		}
	}

	return err
}

// Handles requests of different types in parallel, bidirectional streaming
// Number of parallel workers is defined by MaxOpenConnections
func (s *GeocodingServer) HandleBi(stream BiStreamer, handle handleOne) error {
	arena := s.NewTaskArena(handle)
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel() // cancel when we are finished with queries

	fail := make(chan error)

	go func() { // sender
		var err error
		for {
			select {
			case res := <-arena.done: // drain the channel
				if res != nil && err == nil {
					if err = stream.Send(res); err != nil {
						cancel()
					}
				}
			case <-fail: // exit on behalf of main loop only when arena channel is drained
				fail <- err
				return
			}
		}
	}()

	for {
		query, err := stream.Recv()
		if err == io.EOF { // client finished sending and is waiting for us
			arena.wg.Wait()
			fail <- nil
			return <-fail
		} else if err != nil {
			cancel()
			arena.wg.Wait()
			fail <- nil
			<-fail
			return err
		}

		arena.wg.Add(1)
		go arena.Work(ctx, s, query) // launch worker for each query
	}
}

func ctxAddr(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok {
		return p.Addr.String()
	} else {
		return ""
	}
}
