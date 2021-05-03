package common

import (
	"encoding/json"
	"fmt"

	pb "github.com/X-Company/geocoding/proto/geocoding"
)

type AddressesJson struct {
	Ru    string `json:"ru,omitempty"`
	En    string `json:"en,omitempty"`
	Local string `json:"local,omitempty"`
}

func Addrs2Json(addrs *pb.Addresses) ([]byte, error) {
	adrsJson := AddressesJson{}
	for _, adr := range addrs.Items {
		switch adr.Lang {
		case "ru-RU":
			adrsJson.Ru = adr.Data
		case "en-US":
			adrsJson.En = adr.Data
		case "":
			adrsJson.Local = adr.Data
		default:
			return nil, fmt.Errorf("language %s not supported", adr.Lang)
		}
	}
	return json.Marshal(adrsJson)
}
