package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	pb "github.com/X-Company/geocoding/proto/geocoding"
)

const (
	nominatimURL = "http://nominatim.api.x-company.net"
	yandexURL    = "https://geocode-maps.yandex.ru"
	googleURL    = "https://maps.googleapis.com"
)

var googleAPIKey string = os.Getenv("GOOGLE_API_KEY")
var yandexAPIKey string = os.Getenv("YANDEX_API_KEY")

type (
	nominatimResponse struct {
		Lat     string            `json:"lat"`
		Lon     string            `json:"lon"`
		Address map[string]string `json:"address"`
		Error   string            `json:"error"`
	}

	yandexResponse struct {
		Response struct {
			GeoObjectCollection struct {
				FeatureMember []*yandexFeatureMember `json:"featureMember"`
			} `json:"GeoObjectCollection"`
		} `json:"response"`
		StatusCode int    `json:"statusCode"`
		Error      string `json:"error"`
		Message    string `json:"message"`
	}

	yandexFeatureMember struct {
		GeoObject struct {
			MetaDataProperty struct {
				GeocoderMetaData struct {
					Text string `json:"text"`
				} `json:"GeocoderMetaData"`
			} `json:"metaDataProperty"`
			Point struct {
				Pos string `json:"pos"`
			} `json:"Point"`
		} `json:"GeoObject"`
	}

	googleResponse struct {
		Results []struct {
			FormattedAddress string `json:"formatted_address"`
			Geometry         struct {
				Location struct {
					Lat, Lng float64
				}
			}
		}
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	}
)

func getJson(ctx context.Context, client *http.Client, query string, response interface{}, lang string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", query, nil)
	if err != nil {
		return fmt.Errorf("NewRequestWithContext %s", err)
	} else if lang != "" {
		req.Header.Set("Accept-Language", lang)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Status: %s", resp.Status)
	}

	if data, err := ioutil.ReadAll(resp.Body); err != nil {
		return fmt.Errorf("http ReadAll %s", err)
	} else if json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("Unmarshal http json %s", err)
	}

	return nil
}

func parse_latlon(slat, slon string, point *pb.Point) error {
	lat, err_lat := strconv.ParseFloat(slat, 64)
	lon, err_lon := strconv.ParseFloat(slon, 64)
	if err_lat != nil || err_lon != nil {
		return fmt.Errorf("could not parse lat/lon: %s/%s", slat, slon)
	} else {
		point.Lat = lat
		point.Lon = lon
	}
	return nil
}

// convert Nominatim address details to a string:
// country, region, city, road, the rest ...
func nominatimAddress(details map[string]string) string {
	if len(details) == 0 {
		return ""
	}

	var addressArray []string

	delete(details, "country_code")
	delete(details, "postcode")

	if v, ok := details["country"]; ok {
		if v == "РФ" {
			v = "Россия"
		}
		addressArray = append(addressArray, v)
		delete(details, "country")
	}
	if v, ok := details["region"]; ok {
		addressArray = append(addressArray, v)
		delete(details, "state")
		delete(details, "region")
	}
	if v, ok := details["city"]; ok {
		addressArray = append(addressArray, v)
		delete(details, "county")
		delete(details, "state_district")
		delete(details, "city")
	}
	if v, ok := details["road"]; ok {
		addressArray = append(addressArray, v)
		delete(details, "suburb")
		delete(details, "neighbourhood")
		delete(details, "town")
		delete(details, "residential")
		delete(details, "state_district")
		delete(details, "road")
	}

	// the rest
	for _, v := range details {
		addressArray = append(addressArray, v)
	}

	return strings.Join(addressArray, ", ")
}

// Query source by address
func encodeHttp(ctx context.Context, client *http.Client, adr *pb.Address, source pb.Source) (*pb.Point, error) {
	point := &pb.Point{Id: adr.Id, Source: source}

	switch source {
	case pb.Source_NOMINATIM:
		query := fmt.Sprintf("%s/search?q=%s&format=json", nominatimURL, url.QueryEscape(adr.Data))
		result := []nominatimResponse{}
		if err := getJson(ctx, client, query, &result, ""); err != nil {
			return nil, fmt.Errorf("%s: %v", source, err)
		} else if len(result) == 0 {
			point.Status = pb.Status_NOT_FOUND
			return point, nil
		} else if err := parse_latlon(result[0].Lat, result[0].Lon, point); err != nil {
			return nil, fmt.Errorf("%s: %v", source, err)
		}
	case pb.Source_YANDEX:
		query := fmt.Sprintf("%s/1.x/?apikey=%s&format=json&results=1&geocode=%s",
			yandexURL, yandexAPIKey, url.QueryEscape(adr.Data))
		result := yandexResponse{}
		if err := getJson(ctx, client, query, &result, ""); err != nil {
			return nil, fmt.Errorf("%s: %v", source, err)
		} else if result.Error != "" {
			return nil, fmt.Errorf("%s: %v", source, result.Error)
		} else if arr := result.Response.GeoObjectCollection.FeatureMember; len(arr) == 0 {
			point.Status = pb.Status_NOT_FOUND
			return point, nil
		} else if pos := strings.Split(arr[0].GeoObject.Point.Pos, " "); len(pos) != 2 {
			return nil, fmt.Errorf("%s: Could't parse point", source)
		} else if err := parse_latlon(pos[1], pos[0], point); err != nil {
			return nil, fmt.Errorf("%s: %v", source, err)
		}

	case pb.Source_GOOGLE:
		query := fmt.Sprintf("%s/maps/api/geocode/json?address=%s&key=%s",
			googleURL, url.QueryEscape(adr.Data), googleAPIKey)
		result := googleResponse{}
		if err := getJson(ctx, client, query, &result, ""); err != nil {
			return nil, fmt.Errorf("%s: %v", source, err)
		} else if result.Status != "OK" {
			return nil, fmt.Errorf("%s: %+v", source, result)
		} else if len(result.Results) == 0 {
			point.Status = pb.Status_NOT_FOUND
			return point, nil
		} else {
			latlng := result.Results[0].Geometry.Location
			point.Lat = latlng.Lat
			point.Lon = latlng.Lng
		}
	default:
		return nil, fmt.Errorf("Source %s is not implemented", source)
	}

	return point, nil
}

// Makes a reverse query to Source by coordinates.
func decodeHttp(ctx context.Context, client *http.Client, point *pb.Point,
	lang string, source pb.Source) ([]*pb.Address, error) {
	adr := &pb.Address{Source: source, Lang: lang}

	switch source {
	case pb.Source_NOMINATIM:
		query := fmt.Sprintf("%s/reverse?lat=%g&lon=%g&format=json", nominatimURL,
			point.Lat, point.Lon)
		result := &nominatimResponse{}
		if err := getJson(ctx, client, query, result, lang); err != nil {
			return nil, fmt.Errorf("%s: %v", source, err)
		} else if strings.Contains(result.Error, "Unable to geocode") {
			return []*pb.Address{}, nil
		} else if result.Error != "" {
			return nil, fmt.Errorf("%s: %v", source, result.Error)
		}
		adr.Data = nominatimAddress(result.Address)
	case pb.Source_YANDEX:
		query := fmt.Sprintf("%s/1.x/?apikey=%s&format=json&results=1&lang=%s&geocode=%g,%g",
			yandexURL, yandexAPIKey, lang, point.Lon, point.Lat)
		result := &yandexResponse{}
		if err := getJson(ctx, client, query, result, lang); err != nil {
			return nil, fmt.Errorf("%s: %v", source, err)
		} else if result.Error != "" {
			return nil, fmt.Errorf("%s: %v", source, result.Error)
		} else if arr := result.Response.GeoObjectCollection.FeatureMember; len(arr) == 0 {
			return []*pb.Address{}, nil
		} else {
			adr.Data = arr[0].GeoObject.MetaDataProperty.GeocoderMetaData.Text
		}
	case pb.Source_GOOGLE:
		query := fmt.Sprintf("%s/maps/api/geocode/json?latlng=%g,%g&language=%s&key=%s",
			googleURL, point.Lat, point.Lon, lang, googleAPIKey)
		result := &googleResponse{}
		if err := getJson(ctx, client, query, result, lang); err != nil {
			return nil, fmt.Errorf("%s: %v", source, err)
		} else if result.Status != "OK" {
			return nil, fmt.Errorf("%s: %+v", source, result)
		} else if len(result.Results) == 0 {
			return []*pb.Address{}, nil
		}
		adr.Data = result.Results[0].FormattedAddress
	default:
		return nil, fmt.Errorf("Source %s is not implemented", source)
	}

	return []*pb.Address{adr}, nil
}
