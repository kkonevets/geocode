package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"

	pb "github.com/X-Company/geocoding/proto/geocoding"
)

var httpClient *http.Client

func testSource(t *testing.T, source pb.Source, whats []string, lang string) {
	ctx := context.Background()

	adr := &pb.Address{Data: "Москва, улица Ивана Бабушкина 15 к1-2"}
	point, err := encodeHttp(ctx, httpClient, adr, source)
	if err != nil {
		log.Panicf("%v", err)
	}

	addrs, _ := decodeHttp(ctx, httpClient, point, "ru-RU", source)
	adr = addrs[0]
	if !strings.Contains(adr.Data, whats[0]) {
		t.Errorf("address \"%v\" should contain \"%v\"", adr.Data, whats[0])
	}

	addrs, err = decodeHttp(ctx, httpClient, point, lang, source)
	if !strings.Contains(addrs[0].Data, whats[1]) {
		fmt.Printf("%s\n", addrs[0].Data)
		fmt.Printf("%s\n", whats[1])

		t.Errorf("address \"%v\" should contain \"%v\"", addrs[0].Data, whats[1])
	}

	adr = &pb.Address{Data: "wljefnowje dg jgbwiojbgwri g"}
	point, err = encodeHttp(ctx, httpClient, adr, source)
	if point.Status != pb.Status_NOT_FOUND {
		t.Errorf("Nothing should be found for %s", adr)
	}

	point = &pb.Point{Lat: 123, Lon: 2334}
	addrs, err = decodeHttp(ctx, httpClient, point, "en-EN", source)
	if len(addrs) != 0 {
		t.Errorf("Nothing should be found for %v", point)
	}
}

func TestHttp(t *testing.T) {
	what := []string{"улица Ивана Бабушкина", "Russia, Akademichesky District, улица Ивана Бабушкина"}
	testSource(t, pb.Source_NOMINATIM, what, "en-US")

	what = []string{"улица Ивана Бабушкина", "Moskova, ulitsa Ivana Babushkina"}
	testSource(t, pb.Source_YANDEX, what, "tr_TR")

	// what := []string{"улица Ивана Бабушкина", "South-Western Administrative Okrug"}
	// testSource(t, pb.Source_GOOGLE, what, "en-US")
}

func TestNominatimAddress(t *testing.T) {
	point := &pb.Point{Lat: 64.54009865422924, Lon: 34.777316217578}
	addrs, _ := decodeHttp(ctx, httpClient, point, "ru-RU", pb.Source_NOMINATIM)
	if len(addrs) > 0 {
		adr := addrs[0]
		want := "Россия, улица Воронина"
		if !strings.HasPrefix(adr.Data, want) {
			t.Errorf("Address should start with '%s', got '%s'", want, adr.Data)
		}
	} else {
		t.Errorf("Not found: %v", point)
	}
	// fmt.Printf("%s\n", addrs[0].Data)

}

func init() {
	httpClient = HttpClient()
}
