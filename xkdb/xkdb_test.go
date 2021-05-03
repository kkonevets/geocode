package xxdb

import (
	"fmt"
	"log"
	"net"
	"testing"

	"github.com/X-Company/geocoding/config"
	pb "github.com/X-Company/geocoding/proto/xxdb"
)

func TestSimple(t *testing.T) {
	conf := config.MustLoad().Xxdb
	conn, err := net.Dial("tcp", conf.Host)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client, err := NewClient(conn, conf.User, conf.Pass)
	if err != nil {
		log.Fatal(err)
	}

	events := []*pb.Query_Event{{Id: 1, DeviceHash: 23452335, DeviceDt: 20210110124425,
		Hide: false, Extra: "{'some_key': 'some_value'}"}}
	query := &pb.Query{WithMerge: false, Type: pb.Query_INSERT,
		Events: events}
	if resp, err := client.Exec(query); err != nil {
		log.Fatal(err)
	} else {
		fmt.Printf("%v\n", resp)
	}
}
