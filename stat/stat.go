// send satatistics to InfluxDb
package stat

import (
	"fmt"
	"os"
	"time"

	"github.com/X-Company/geocoding/config"
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	influxdb "github.com/influxdata/influxdb1-client/v2"
	log "github.com/sirupsen/logrus"
)

type (
	Field int           // field enum
	Point map[Field]int // point of measurement, accumulates values from channel
)

const (
	Decode             Field = iota // number of function calls
	Encode                          // number of function calls
	DecodeMany                      // number of function calls
	EncodeMany                      // number of function calls
	DecodeBi                        // number of function calls
	EncodeBi                        // number of function calls
	DecodeSuccess                   // number of successfully decoded points
	DecodeFail                      // number of point decoding failures
	EncodeSuccess                   // number of successfully encoded addresses
	EncodeFail                      // number of address encoding failures
	NominatimSuccess                // number of successful Nominatim queries
	NominatimFail                   // number of failed Nominatim queries
	YandexSuccess                   // number of successful Yandex queries
	YandexFail                      // number of failed Yandex queries
	CacheDecodeSuccess              // number of points found in Cache
	CacheEncodeSuccess              // number of addresses found in Cache
)

var fieldName = map[Field]string{
	Decode:             "Decode",
	Encode:             "Encode",
	DecodeMany:         "DecodeMany",
	EncodeMany:         "EncodeMany",
	DecodeBi:           "DecodeBi",
	EncodeBi:           "EncodeBi",
	DecodeSuccess:      "DecodeSuccess",
	DecodeFail:         "DecodeFail",
	EncodeSuccess:      "EncodeSuccess",
	EncodeFail:         "EncodeFail",
	NominatimSuccess:   "NominatimSuccess",
	NominatimFail:      "NominatimFail",
	YandexSuccess:      "YandexSuccess",
	YandexFail:         "YandexFail",
	CacheDecodeSuccess: "CacheDecodeSuccess",
	CacheEncodeSuccess: "CacheEncodeSuccess",
}

var infxClient influxdb.Client // set global client to reuse http connections
var dataBase string
var hostName string

func sendPoint(p *Point, measurement string) {
	tags := map[string]string{"Host": hostName}
	fields := map[string]interface{}{}

	for k, v := range *p {
		if v != 0 {
			if name, ok := fieldName[k]; ok {
				fields[name] = v
			} else {
				log.Panicf("Field name undefined for: %d", k)
			}
		}
	}

	pt, err := influxdb.NewPoint(measurement, tags, fields, time.Now())
	if err != nil {
		log.Panicf("Influx: NewPoint:", err)
	}

	bp, err := influxdb.NewBatchPoints(influxdb.BatchPointsConfig{
		Database:  dataBase,
		Precision: "s",
	})
	if err != nil {
		log.Panicf("Influx: NewBatchPoints: ", err)
	}

	bp.AddPoint(pt)
	if err := infxClient.Write(bp); err != nil {
		log.Errorf("Influx Write: %s", err)
	}
}

// save statistics to InfluxDb periodically, receive points through a channel
func Serve(in <-chan *Point, measurement string) {
	ticker := time.NewTicker(10 * time.Second) // save statistics every 10 secs
	p := &Point{}
	for inp := range in {
		for k, v := range *inp {
			if v != 0 {
				(*p)[k] += v // increment point value by incoming value
			}
		}

		select {
		case <-ticker.C:
			go sendPoint(p, measurement)
			p = &Point{} // clear stat Point after send
		default:
		}
	}
}

func init() {
	var err error
	if hostName, err = os.Hostname(); err != nil {
		log.Panic(err)
	}

	conf := config.MustLoad()
	infxClient, err = influxdb.NewHTTPClient(influxdb.HTTPConfig{
		Addr: fmt.Sprintf("http://%s", conf.InfluxDb.Host),
	})
	if err != nil {
		log.Panicf("Error creating InfluxDB Client: ", err.Error())
	}
	dataBase = conf.InfluxDb.Database
}
