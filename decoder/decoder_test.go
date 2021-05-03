package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/X-Company/geocoding/cache/server"
	"github.com/X-Company/geocoding/config"
	kafka "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
)

var cancelTest context.CancelFunc

// Mock Kafka queue by reading local file
type Reader struct {
	scanner     *bufio.Scanner
	nLines      int
	currentLine int
}

func (r *Reader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error { return nil }
func (r *Reader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if r.scanner.Scan() {
		r.currentLine++
		if r.currentLine == r.nLines {
			cancelTest()
		}
		return kafka.Message{Value: r.scanner.Bytes()}, nil
	} else {
		if err := r.scanner.Err(); err != nil {
			log.Panic(err)
		}
		return kafka.Message{}, nil
	}
}

func eventsStatistics(file *os.File) (int, int, int) {
	scanner := bufio.NewScanner(file)
	i, j := 0, 0
	devicesWithLocation := make(map[int]bool)
	for scanner.Scan() {
		var ev Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			log.Panicf("%+v", err)
		}
		i++
		if ev.Location.Latitude != 0 || ev.Location.Longitude != 0 {
			if ev.NeedSend {
				j++
			}
			devicesWithLocation[int(ev.DeviceHash)] = true
		}

	}

	if err := scanner.Err(); err != nil {
		log.Panic(err)
	}
	return i, j, len(devicesWithLocation)
}

func TestConsume(t *testing.T) {
	conf := config.MustLoad()

	dbxDP := sqlConnect(conf.DataProcessed.ConnectionParams, conf.DataProcessed.MaxOpenConnections)
	defer dbxDP.Close()
	dbxEP := sqlConnect(conf.EventsPool.ConnectionParams, conf.EventsPool.MaxOpenConnections)
	defer dbxEP.Close()

	var nRowsDataBefore int
	if err := dbxDP.Get(&nRowsDataBefore, "SELECT COUNT(*) FROM data_processed;"); err != nil {
		log.Panic(err)
	} else if nRowsDataBefore > 0 {
		t.Fatal("data_processed table is not empty. " +
			"Are you shure this is not a production database? " +
			"Because data_processed table would be cleared for test purposes.")
	}

	var (
		_, b, _, _ = runtime.Caller(0)
		basepath   = filepath.Dir(b)
	)

	dbxDP.MustExec(`
	ALTER TABLE data_processed
	    ADD COLUMN IF NOT EXISTS address JSONB,
	    ADD COLUMN IF NOT EXISTS address_dt TIMESTAMP WITH TIME ZONE;
        `)

	content, err := ioutil.ReadFile(path.Join(basepath + "/data_processed.sql"))
	if err != nil {
		log.Panic(err)
	}
	sqlInsertQuery := string(content)
	dbxDP.MustExec(sqlInsertQuery)

	file, err := os.Open(path.Join(basepath + "/geo_decoding_data.txt"))
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	nLinesPool, nNeedSend, withLocation := eventsStatistics(file)
	file.Seek(0, 0)
	reader := &Reader{scanner: bufio.NewScanner(file), nLines: nLinesPool}

	var ctx context.Context
	ctx, cancelTest = context.WithCancel(context.Background())
	defer cancelTest()
	consume(ctx, conf, reader, 40)

	var nRowsPool int
	if err := dbxEP.Get(&nRowsPool,
		"SELECT COUNT(*) FROM events_pool where event IS NOT NULL;"); err != nil {
		log.Panic(err)
	}

	var nRowsData int
	if err := dbxDP.Get(&nRowsData,
		"SELECT COUNT(*) FROM data_processed WHERE address IS NOT NULL;"); err != nil {
		log.Panic(err)
	}

	// cleanup
	dbxDP.MustExec(`DELETE FROM data_processed WHERE true`)
	dbxEP.MustExec(`DELETE FROM events_pool WHERE true`)

	if int(nRowsPool) != nNeedSend {
		t.Errorf("got %d events_pool number of lines, wanted %d", nRowsPool, nNeedSend)
	}

	if int(nRowsData) != withLocation {
		t.Errorf("got %d data_processed number of lines, wanted %d", nRowsData, withLocation)
	}
}

func init() {
	go server.StartServer()
}
