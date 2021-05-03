// Migrate data_processed to a new schema. Add two columns "address" and "address_dt",
// fill those columns by decoding point coordinates

package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/X-Company/geocoding/config"
	decom "github.com/X-Company/geocoding/decoder/common"
	pb "github.com/X-Company/geocoding/proto/geocoding"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type (
	Row struct {
		Id     int64
		GsmLat sql.NullString
		GsmLon sql.NullString
		GeoLat sql.NullFloat64
		GeoLon sql.NullFloat64
		GeoDt  sql.NullTime
		GsmDt  sql.NullTime
	}

	Event struct {
		Id    int
		epoch int64
	}
)

func decode(ctx context.Context, client pb.GeocodingClient, dbx *sqlx.DB,
	points *pb.Points, events *[]Event) {
	stream, err := client.DecodeMany(ctx, points)
	if err != nil {
		log.Fatalf("%v.DecodeMany(_) = _, %v", client, err)
	}

	var values []string
	for {
		addrs, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("%v.DecodeMany(_) = _, %v", client, err)
		}

		if addrs.Status != pb.Status_OK {
			continue
		}

		if jsonb, err := decom.Addrs2Json(addrs); err != nil {
			log.Fatal(err)
		} else if len(addrs.Items) > 0 {
			ev := &(*events)[int(addrs.Id)]
			address := bytes.ReplaceAll(jsonb, []byte("'"), []byte("''"))
			vals := fmt.Sprintf("(%d, '%s'::JSONB, to_timestamp(%d))", ev.Id, address, ev.epoch)
			values = append(values, vals)
		}
	}

	// bulk update
	query := `
		UPDATE data_processed
		SET
		  address = tmp.address,
                  address_dt = tmp.address_dt
		FROM (VALUES %s) AS tmp (id, address, address_dt)
		WHERE data_processed.id = tmp.id`

	query = fmt.Sprintf(query, strings.Join(values, ","))
	if _, err = dbx.Exec(query); err != nil {
		log.Fatalf("decode: %s", err)
	}

	points.Items = points.Items[:0]
	*events = (*events)[:0]
}

func main() {
	conf := config.MustLoad()
	dbx := sqlx.MustConnect("postgres", conf.DataProcessed.ConnectionParams)

	dbx.MustExec(`
	ALTER TABLE data_processed
	    ADD COLUMN IF NOT EXISTS address JSONB,
	    ADD COLUMN IF NOT EXISTS address_dt TIMESTAMP WITH TIME ZONE;
	`)

	opts := []grpc.DialOption{grpc.WithBlock(), grpc.WithInsecure()}
	conn, err := grpc.Dial(conf.Decoder, opts...)
	if err != nil {
		log.Panicf("fail to dial: %+v", err)
	}
	defer conn.Close()

	client := pb.NewGeocodingClient(conn)
	lifetime := 24 * time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), lifetime)
	defer cancel()

	rows, err := dbx.Queryx(
		`SELECT id AS Id, extra->'gsm_dec'->'lat' AS GsmLat,
                        extra->'gsm_dec'->'lon' AS GsmLon,
                        geo[0] AS GeoLat, geo[1] AS GeoLon, geo_dt AS GeoDt,
                        (extra_dt->'gsm_dec')::text::timestamp AT time zone 'UTC' AS GsmDt
                 FROM data_processed;`) // where device_hash=-3869566462393835170
	if err != nil {
		log.Fatal(err)
	}

	points := &pb.Points{}
	events := []Event{}
	row := Row{}
	batchSize := 20_000
	i := 0
	for rows.Next() {
		if err := rows.StructScan(&row); err != nil {
			log.Panic(err)
		}

		i++
		point := pb.Point{}

		if !row.GeoDt.Valid && !row.GsmDt.Valid {
			continue
		} else if !row.GeoDt.Valid {
			goto GSM
		} else if !row.GsmDt.Valid {
			goto GEO
		} else if row.GeoDt.Time.After(row.GsmDt.Time) || row.GeoDt.Time.Equal(row.GsmDt.Time) {
			goto GEO
		} else {
			goto GSM
		}

	GSM:
		if row.GsmLat.Valid && row.GsmLon.Valid {
			lat, err_lat := strconv.ParseFloat(row.GsmLat.String, 64)
			lon, err_lon := strconv.ParseFloat(row.GsmLon.String, 64)
			if err_lat == nil && err_lon == nil {
				point.Lat = lat
				point.Lon = lon
				events = append(events, Event{Id: int(row.Id), epoch: row.GsmDt.Time.Unix()})
			} else {
				continue
			}
		} else {
			continue
		}
		goto FINISH

	GEO:
		if row.GeoLat.Valid && row.GeoLon.Valid {
			point.Lat = row.GeoLat.Float64
			point.Lon = row.GeoLon.Float64
			events = append(events, Event{Id: int(row.Id), epoch: row.GeoDt.Time.Unix()})
		} else {
			continue
		}

	FINISH:

		point.Id = (int64)(len(events) - 1)
		points.Items = append(points.Items, &point)

		if len(points.Items) == batchSize {
			decode(ctx, client, dbx, points, &events)
			fmt.Printf("\r%d", i)
		}
	}

	if len(points.Items) > 0 {
		decode(ctx, client, dbx, points, &events)
		fmt.Printf("\r%d\n", i+len(points.Items))
	}
}
