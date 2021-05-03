package server

import (
	"context"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/X-Company/geocoding/cache/db"
	"github.com/X-Company/geocoding/config"
	pb "github.com/X-Company/geocoding/proto/geocoding"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var conf *config.Config
var client pb.GeocodingClient
var ctx context.Context

func TestDecode(t *testing.T) {
	point := &pb.Point{Lat: 47.433299, Lon: -13.788183}
	addrs, err := client.Decode(ctx, point) // Atlantic Ocean
	if err != nil {
		log.Panic(err)
	}

	var adr *pb.Address = nil
	for _, item := range addrs.Items {
		if item.Lang == "ru-RU" {
			adr = item
		}
	}

	want := "Атлантический океан"
	if adr.Data != want {
		t.Errorf("Got %q, want %q", adr.Data, want)
	}

	addrs, err = client.Decode(ctx, point) // Atlantic Ocean
	if err != nil {
		log.Panic(err)
	}

	adr = addrs.Items[0]
	if adr.Source != pb.Source_CACHE {
		t.Errorf("Got %q, want %q", adr.Source, pb.Source_CACHE)
	}

	// cleanup
	dbx := sqlx.MustConnect("postgres", conf.Cache.ConnectionParams)
	defer dbx.Close()
	ipoint, _ := (&db.FloatPoint{Lat: point.Lat, Lon: point.Lon}).ToInt()
	res := dbx.MustExec("DELETE FROM geocode_cache WHERE (lat,lon) IN (($1, $2)) RETURNING *;",
		ipoint.Lat, ipoint.Lon)

	nrows, err := res.RowsAffected()
	if nrows != 2 {
		t.Errorf("Number of deleted rows should be 2, not %d", nrows)
	}
}

func testDist(t *testing.T, point *pb.Point, point2 *pb.Point) {
	dist := math.Sqrt(math.Pow(point.Lat-point2.Lat, 2) +
		math.Pow(point.Lon-point2.Lon, 2))
	const lengthMeridian float64 = 20004274
	const METER float64 = 180. / lengthMeridian
	if dist > 1000*METER {
		t.Errorf("point is too far from target")
	}
}

func TestEncode(t *testing.T) {
	tryEncode := func(adr *pb.Address, success bool) {
		point, err := client.Encode(ctx, adr)
		if err != nil {
			log.Panic(err)
		}
		if success {
			if point.Status == pb.Status_NOT_FOUND {
				log.Panicf("%v %v", point.Status, adr)
			}

			point2 := &pb.Point{Lat: 55.67687203009099, Lon: 37.56833762739498}
			testDist(t, point, point2)
		} else {
			if point.Status != pb.Status_NOT_FOUND {
				t.Errorf("Did not expect to encode this: %v", adr)
			}
		}
	}

	adr_data := "Москва Нахимовский проспект 36"
	adr := &pb.Address{Data: adr_data}
	tryEncode(adr, true) // NOMINATIM

	adr = &pb.Address{Data: adr_data, Source: pb.Source_YANDEX}
	tryEncode(adr, true)

	adr = &pb.Address{Data: adr_data, Source: pb.Source_CACHE}
	point, err := client.Encode(ctx, adr)
	if err != nil {
		log.Panic(err)
	}
	if point.Source != pb.Source_CACHE {
		t.Errorf("Second read should succeed in CACHE, got %s", point.Source)
	}

	// cleanup
	postgres := conf.Cache
	dbx := sqlx.MustConnect("postgres", postgres.ConnectionParams)
	defer dbx.Close()

	delRow := func() {
		res := dbx.MustExec("DELETE FROM geocode_cache WHERE address_hash=$1 RETURNING *;",
			db.StringHash(adr.Data))
		nrows, _ := res.RowsAffected()
		if nrows != 1 {
			t.Errorf("Number of deleted rows should be 1, not %d", nrows)
		}
	}

	delRow()

	adr = &pb.Address{Data: "abracadabrafffldldldls wdfjwwjefjkk", Source: pb.Source_CACHE}
	tryEncode(adr, false)
	delRow()
}

func TestServerStreaming(t *testing.T) {
	addrs := &pb.Addresses{}
	points := &pb.Points{}
	addrs.Items = append(addrs.Items, &pb.Address{Data: "Москва Нахимовский проспект 36"})
	stream, err := client.EncodeMany(ctx, addrs)
	if err != nil {
		log.Fatalf("%v.EncodeMany(_) = _, %v", client, err)
	}
	for {
		point, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("%v.EncodeMany(_) = _, %v", client, err)
		}
		points.Items = append(points.Items, point)
	}

	dstream, err := client.DecodeMany(ctx, points)
	if err != nil {
		log.Fatalf("%v.DecodeMany(_) = _, %v", client, err)
	}
	for {
		addrs, err := dstream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("%v.DecodeMany(_) = _, %v", client, err)
		}
		var adr *pb.Address
		for _, adr = range addrs.Items {
			if adr.Lang == "ru-RU" {
				break
			}
		}
		want := "Россия, Академический район, Нахимовский проспект"
		if !strings.Contains(adr.Data, want) {
			t.Fatalf("\"%v\" should contain \"%v\"", adr.Data, want)
		}
	}
}

func TestBiStreaming(t *testing.T) {
	var point *pb.Point
	{
		adr := &pb.Address{Data: "Москва Нахимовский проспект 36"}

		stream, err := client.EncodeBi(ctx)
		if err != nil {
			log.Fatalf("%v.RouteChat(_) = _, %v", client, err)
		}
		waitc := make(chan struct{})

		go func() {
			for {
				res, err := stream.Recv()
				if err == io.EOF {
					close(waitc) // read done
					return
				} else if err != nil {
					log.Fatalf("Failed to receive a message : %v", err)
				}
				point = res
			}
		}()

		if err := stream.Send(adr); err != nil {
			log.Fatalf("Failed to send a note: %v", err)
		}

		stream.CloseSend()
		<-waitc
	}

	stream, err := client.DecodeBi(ctx)
	if err != nil {
		log.Fatalf("%v.RouteChat(_) = _, %v", client, err)
	}
	waitc := make(chan struct{})

	go func() {
		for {
			addrs, err := stream.Recv()
			if err == io.EOF {
				close(waitc) // read done
				return
			} else if err != nil {
				log.Fatalf("Failed to receive a message : %v", err)
			}
			var adr *pb.Address
			for _, adr = range addrs.Items {
				if adr.Lang == "ru-RU" {
					break
				}
			}
			want := "Россия, Академический район, Нахимовский проспект"
			if !strings.Contains(adr.Data, want) {
				t.Errorf("\"%v\" should contain \"%v\"", adr.Data, want)
			}
		}
	}()

	if err := stream.Send(point); err != nil {
		log.Fatalf("Failed to send a note: %v", err)
	}

	stream.CloseSend()
	<-waitc
}

func init() {
	go StartServer()

	opts := []grpc.DialOption{grpc.WithBlock(), grpc.WithInsecure()}
	conf = config.MustLoad()
	conn, err := grpc.Dial(conf.GRPC.Host, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	// defer conn.Close()

	client = pb.NewGeocodingClient(conn)
	ctx = context.Background()
}
