// Берем из Кафки набор задач для декодирования.
// Делаем запрос к API geocoding.
// Результат запроса нужен для нескольких языков
// Результат запроса сохраняем в поля address и address_dt  в таблице data_processed,
// НО только в том случае, если address_dt из БД старее, чем device_dt из задачи
// Сохраняем в таблицу events_pool
// Помечаем сообщение в кафке как прочитанное

package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/X-Company/geocoding/config"
	pb "github.com/X-Company/geocoding/proto/geocoding"
	"github.com/X-Company/geocoding/stat"
	_ "github.com/lib/pq"
	kafka "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type (
	// Events from Kafka queue (parsed Messages)
	Event struct {
		DeviceHash int64  `json:"device_hash"`
		DeviceDt   string `json:"device_dt"`
		NeedSave   bool   `json:"need_save"`
		NeedSend   bool   `json:"need_send"`
		Location   struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		}
		LocationType int `json:"location_type"`
		Sid          int `json:"sid"`
		Gmcc         int `json:"gmcc,omitempty"`
	}

	Consumer interface {
		CommitMessages(ctx context.Context, msgs ...kafka.Message) error
		FetchMessage(ctx context.Context) (kafka.Message, error)
	}
)

// Consume Kafka messages and commit them periodically
func consume(queueCtx context.Context, conf *config.Config, reader Consumer, commitThreshold int) {
	opts := []grpc.DialOption{grpc.WithBlock(), grpc.WithInsecure()}
	conn, err := grpc.Dial(conf.Decoder, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %+v", err)
	}
	defer conn.Close()

	work := initWork(conf, conn)
	defer work.Close()

	go stat.Serve(work.statC, "geodecoder") // receive work statistics

	go work.Do() // start processing coordinates

	for { // commit every hour or on number of messages threshold
		if len(*work.messages) == commitThreshold {
			work.commit(queueCtx, reader, conn)
		}

		select {
		case <-work.ticker.C:
			work.commit(queueCtx, reader, conn)
		case <-queueCtx.Done():
			work.commit(queueCtx, reader, conn)
			return // return from test case
		default:
			var ev Event
			m, err := reader.FetchMessage(context.Background())
			if err != nil {
				log.Fatalf("%+v", err)
			} else if err = json.Unmarshal(m.Value, &ev); err != nil {
				log.Fatalf("%+v", err)
			}
			if ev.Location.Latitude == 0 && ev.Location.Longitude == 0 {
				continue
			}

			*work.messages = append(*work.messages, m) // for future commit
			*work.events = append(*work.events, ev)    // for doing work

			// set Id equal to index in messages/events
			point := &pb.Point{Id: (int64)(len(*work.events) - 1), // convert int to int64
				Lat: ev.Location.Latitude, Lon: ev.Location.Longitude}
			if err := work.stream.Send(point); err != nil {
				log.Fatalf("Failed to send a point: %v", err)
			}
		}
	}
}

func main() {
	config.InitLogging("decoder.log") // LOG_LEVEL=warning ./build/decoder -log

	conf := config.MustLoad()
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:           []string{conf.Kafka.Broker},
		GroupID:           conf.Kafka.GroupId,
		StartOffset:       kafka.FirstOffset,
		Topic:             conf.Kafka.Topic,
		HeartbeatInterval: 5 * time.Second,
		SessionTimeout:    30 * time.Second,
	})
	defer reader.Close()

	consume(context.TODO(), conf, reader, 100_000)
}
