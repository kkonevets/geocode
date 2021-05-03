// work done on a single bidirectional grpc connection
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/X-Company/geocoding/config"
	decom "github.com/X-Company/geocoding/decoder/common"
	pb "github.com/X-Company/geocoding/proto/geocoding"
	"github.com/X-Company/geocoding/stat"
	"github.com/jmoiron/sqlx"
	kafka "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type (
	Payload struct {
		Event   *Event // parsed Kafka message
		Address string // point address decoded by Cache server
	}

	// Marshal struct for events_pool.event
	PoolEvent struct {
		Address   string `json:"address"`
		AddressDt string `json:"address_dt"`
	}

	// work accosiated with a single gRPC stream at a time
	Work struct {
		dbxDP    *sqlx.DB                    // data_processing connection
		dbxEP    *sqlx.DB                    // events_pool connection
		stream   pb.Geocoding_DecodeBiClient // grpc bidirectional stream
		cancel   context.CancelFunc          // stream cancel function
		messages *[]kafka.Message            // Kafka messages for the stream
		events   *[]Event                    // parsed json kafka.Messages
		waitc    chan struct{}               // wait channel, indicates the end of work
		statC    chan *stat.Point            // channel to send statistics to
		duration time.Duration               // commit interval of Kafka messages
		ticker   *time.Ticker                // ticker for commiting Kafka messages
	}
)

func initWork(conf *config.Config, conn *grpc.ClientConn) *Work {
	stream, cancel := initStream(conn)
	duration := time.Hour
	work := &Work{stream: stream, cancel: cancel,
		events: &[]Event{}, messages: &[]kafka.Message{},
		dbxDP: sqlConnect(conf.DataProcessed.ConnectionParams, conf.DataProcessed.MaxOpenConnections),
		dbxEP: sqlConnect(conf.EventsPool.ConnectionParams, conf.EventsPool.MaxOpenConnections),
		waitc: make(chan struct{}),
		// assume max 10 nonblocking statistical measurements (sends)
		statC:    make(chan *stat.Point, 10),
		duration: duration,
		ticker:   time.NewTicker(duration),
	}
	return work
}

func sqlConnect(params string, MaxOpenConnections int) *sqlx.DB {
	dbx := sqlx.MustConnect("postgres", params)
	dbx.SetMaxOpenConns(MaxOpenConnections) // max number of parallel sql queries
	dbx.SetMaxIdleConns(MaxOpenConnections)
	return dbx
}

func initStream(conn *grpc.ClientConn) (pb.Geocoding_DecodeBiClient, context.CancelFunc) {
	commitCtx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	client := pb.NewGeocodingClient(conn)
	stream, err := client.DecodeBi(commitCtx)
	if err != nil {
		log.Fatalf("%v.DecodeBi(_) = _, %v", client, err)
	}
	return stream, cancel
}

// Update address_dt and address but only if address_dt < DeviceDt
func updateDataProcessed(dbx *sqlx.DB, wg *sync.WaitGroup, payload *Payload) {
	defer wg.Done()

	const pat string = `
        set time zone UTC;
	UPDATE data_processed
	SET update_dt = CURRENT_TIMESTAMP,
	    address =
	    CASE
	        WHEN address_dt IS NULL OR address_dt < '%[1]s'::timestamp at time zone 'UTC'
                THEN '%[2]s'::JSONB
	        ELSE address
	    END,
	    address_dt =
	    CASE
	        WHEN address_dt IS NULL OR address_dt < '%[1]s'::timestamp at time zone 'UTC'
                THEN '%[1]s'::timestamp at time zone 'UTC'
	        ELSE address_dt
	    END
	WHERE device_hash = %[3]d;`

	address := strings.ReplaceAll(payload.Address, "'", "''")
	query := fmt.Sprintf(pat, payload.Event.DeviceDt, address, payload.Event.DeviceHash)
	if _, err := dbx.Exec(query); err != nil {
		log.Errorf("data processed: %s: %+v %s", err, payload.Event, payload.Address)
	}
}

func updateEventsPool(dbx *sqlx.DB, wg *sync.WaitGroup, payload *Payload) {
	defer wg.Done()

	const pat string = `
	INSERT INTO events_pool(device_hash, event, created_at, updated_at)
	VALUES ($1, $2::JSONB, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);`

	address := strings.ReplaceAll(payload.Address, "'", "''")
	pev := PoolEvent{Address: address, AddressDt: payload.Event.DeviceDt}
	if jsonb, err := json.Marshal(pev); err != nil {
		log.Errorf("event pool %s: %+v", err, pev)
	} else if _, err := dbx.Exec(pat, payload.Event.DeviceHash, jsonb); err != nil {
		log.Errorf("event pool: %s: %+v %s", err, payload.Event, payload.Address)
	}
}

// Process all messages from a stream and notify on a channel at the end
func (work *Work) Do() {
	var wg sync.WaitGroup

	for {
		addrs, err := work.stream.Recv()
		if err == io.EOF {
			wg.Wait()                // wait for all tasks to finish
			work.waitc <- struct{}{} // notify work is done
			return
		} else if err != nil {
			log.Fatalf("Failed to receive a message : %v", err)
		}

		ev := &(*work.events)[int(addrs.Id)] // Id is an index in messages/events
		if addrs.Status != pb.Status_OK {
			log.Warnf("work: %v, %v", addrs, *ev)
			work.statC <- &stat.Point{stat.DecodeFail: 1}
			continue
		}

		if data, err := decom.Addrs2Json(addrs); err != nil {
			log.Fatal(err)
		} else if len(addrs.Items) > 0 {
			if source := addrs.Items[0].Source; source == pb.Source_CACHE {
				log.Infof("%s %s %v", source, string(data), ev)
			}
			payload := &Payload{Event: ev, Address: string(data)}

			wg.Add(1)
			go updateDataProcessed(work.dbxDP, &wg, payload)
			if ev.NeedSend {
				wg.Add(1)
				go updateEventsPool(work.dbxEP, &wg, payload)
			}
		}
		work.statC <- &stat.Point{stat.DecodeSuccess: 1}
	}
}

// close channel and wait for work to finish then commit Kafka messages, reconnect to server
func (w *Work) commit(queueCtx context.Context, reader Consumer, conn *grpc.ClientConn) {
	w.stream.CloseSend() // tell server we want to stop
	<-w.waitc            // wait for work to finish

	if l := len(*w.events); l > 0 {
		log.Warnf("device_dt: %v commiting", (*w.events)[l-1].DeviceDt)
	}

	// commit on success
	if err := reader.CommitMessages(context.Background(), (*w.messages)...); err != nil {
		log.Fatal("failed to commit messages:", err)
	}
	(*w.messages) = (*w.messages)[:0] // clear messages
	(*w.events) = (*w.events)[:0]     // clear events

	select {
	case <-queueCtx.Done(): // do not spawn worker
	default:
		w.stream, w.cancel = initStream(conn) // reconnect to avoid stream timeout limit
		go w.Do()                             // start consuming a stream
		w.ticker.Reset(w.duration)            // reset commit ticker
	}
}

func (w *Work) Close() {
	w.dbxDP.Close()
	w.dbxEP.Close()
	w.cancel() // cancel grpc stream
	w.ticker.Stop()
}

// xconn, err := net.Dial("tcp", conf.Xxdb.Host)
// if err != nil {
// 	log.Fatalf("Failed to dial: %v", err)
// }
// defer xconn.Close()

// xxdbc, err := xxdb.NewClient(xconn, conf.Xxdb.User, conf.Xxdb.Pass)
// if err != nil {
// 	log.Fatalf("Xxdb: %s", err)
// }

// var err error
// if work.digitsReg, err = regexp.Compile("[^0-9]+"); err != nil {
// 	log.Fatal(err)
// }

// func updateXxdb(xxdbc *xxdb.Client, wg *sync.WaitGroup, payload *Payload, digits_reg *regexp.Regexp) {
// 	defer wg.Done()

// 	ddt := digits_reg.ReplaceAllString(payload.Event.DeviceDt, "")
// 	ddti, err := strconv.ParseUint(ddt, 10, 64)
// 	if err != nil {
// 		log.Fatalf("%s, %s", err, ddt)
// 	}
// 	events := []*pbx.Query_Event{{DeviceHash: payload.Event.DeviceHash,
// 		DeviceDt: ddti, Hide: false, Extra: payload.Address}}
// 	query := &pbx.Query{WithMerge: true, Type: pbx.Query_INSERT, Events: events}
// 	if resp, err := xxdbc.Exec(query); err != nil {
// 		log.Fatalf("%s: %v", err, query)
// 	} else if resp.Status != pbx.Response_OK {
// 		log.Fatalf("%v: %v", resp, query)
// 	}
// }
