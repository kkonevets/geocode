// encode/decode one entity
package server

import (
	"context"
	"fmt"
	"time"

	"github.com/X-Company/geocoding/cache/db"
	pb "github.com/X-Company/geocoding/proto/geocoding"
	"github.com/X-Company/geocoding/stat"
	log "github.com/sirupsen/logrus"
)

func (s *GeocodingServer) decodeOne(ctx context.Context, point *pb.Point) (*pb.Addresses, error) {
	ipoint, err := (&db.FloatPoint{Lat: point.Lat, Lon: point.Lon}).ToInt()
	if err != nil {
		s.statC <- &stat.Point{stat.DecodeFail: 1}
		log.Errorf("%s: %+v %+v", ctxAddr(ctx), err, point)
		return &pb.Addresses{Id: point.Id, Status: pb.Status_BAD_REQUEST}, err
	}

	// do not change these names, you can only add new (otherwise change database records)
	langs := []string{"ru-RU", "en-US", ""}

	switch point.Source {
	case pb.Source_CACHE:

		rows := []db.CacheRow{}
		const query string = `
                SELECT address, lang, updated, encode FROM geocode_cache where lat=$1 AND lon=$2`
		if err := s.dbx.Select(&rows, query, ipoint.Lat, ipoint.Lon); err != nil {
			s.statC <- &stat.Point{stat.DecodeFail: 1}
			log.Errorf("%s: CACHE: %#v, SELECT %d, %d", ctxAddr(ctx), err, ipoint.Lat, ipoint.Lon)
			return &pb.Addresses{Id: point.Id, Status: pb.Status_INTERNAL_SERVER_ERROR}, err
		} else if len(rows) > 0 /* found in CACHE */ {
			addrs := pb.Addresses{Id: point.Id}
			halfAYearBefore := time.Now().AddDate(0, -6, 0)
			for _, row := range rows {
				// go update if too old and not a user defined address
				if !row.Updated.Valid ||
					row.Updated.Time.Before(halfAYearBefore) && !row.Encode {
					goto NOMINATIM
				}
				addrs.Items = append(addrs.Items, &pb.Address{Data: row.Address,
					Source: pb.Source_CACHE, Lang: row.Lang.String})
			}
			s.statC <- &stat.Point{stat.DecodeSuccess: 1, stat.CacheDecodeSuccess: 1}
			return &addrs, nil
		}
	NOMINATIM:
		fallthrough

	case pb.Source_NOMINATIM:

		if addrs := s.decodeHttpLangs(ctx, point, langs, pb.Source_NOMINATIM); len(addrs.Items) > 0 {
			s.statC <- &stat.Point{stat.DecodeSuccess: 1, stat.NominatimSuccess: 1}
			return addrs, nil
		}
		s.statC <- &stat.Point{stat.NominatimFail: 1}
		if point.Source == pb.Source_NOMINATIM {
			break
		}
		fallthrough // break

	case pb.Source_YANDEX:

		// could't find locally, try yandex
		if addrs := s.decodeHttpLangs(ctx, point, langs[:2], pb.Source_YANDEX); len(addrs.Items) > 0 {
			// save to CACHE
			if err = db.UpdateCacheDecode(s.dbx, ipoint, addrs); err != nil {
				s.statC <- &stat.Point{stat.DecodeFail: 1, stat.YandexFail: 1}
				log.Errorf("%s: YANDEX: %+v %+v", ctxAddr(ctx), err, point)
				return &pb.Addresses{Id: point.Id, Status: pb.Status_INTERNAL_SERVER_ERROR}, err
			}
			s.statC <- &stat.Point{stat.DecodeSuccess: 1, stat.YandexSuccess: 1}
			return addrs, nil
		}
		s.statC <- &stat.Point{stat.YandexFail: 1}

	default:

		err := fmt.Errorf("source %s not implemented", point.Source)
		s.statC <- &stat.Point{stat.DecodeFail: 1}
		log.Errorf("%s: %+v, %+v", ctxAddr(ctx), err, point)
		return &pb.Addresses{Id: point.Id, Status: pb.Status_BAD_REQUEST}, err
	}

	// FINALLY:
	s.statC <- &stat.Point{stat.DecodeFail: 1}
	log.Warnf("%s: Decode(): Not found, %+v", ctxAddr(ctx), point)
	return &pb.Addresses{Id: point.Id, Status: pb.Status_NOT_FOUND}, nil
}

func (s *GeocodingServer) encodeOne(ctx context.Context, adr *pb.Address) (*pb.Point, error) {
	switch adr.Source {
	case pb.Source_CACHE:

		hash := db.StringHash(adr.Data)
		rows := []db.CacheRow{}
		// take first non NULL coordinate or NULL if nothing is left
		const query string = `
                SELECT lat, lon, updated, encode FROM geocode_cache
                WHERE address_hash=$1 order by (lat IS NULL OR lon is NULL) limit 1;`
		if err := s.dbx.Select(&rows, query, hash); err != nil {
			s.statC <- &stat.Point{stat.EncodeFail: 1}
			log.Errorf("%s: CACHE: %#v, SELECT %+v", ctxAddr(ctx), err, adr)
			return &pb.Point{Id: adr.Id, Status: pb.Status_INTERNAL_SERVER_ERROR}, err
		} else if len(rows) > 0 { // found in CACHE
			row := rows[0]
			if (!row.Lat.Valid || !row.Lon.Valid) && row.Encode {
				dayBefore := time.Now().AddDate(0, 0, -1)
				if !row.Updated.Valid || row.Updated.Time.After(dayBefore) {
					// NULL lat/lon during last day consider NOT_FOUND
					s.statC <- &stat.Point{stat.EncodeFail: 1}
					log.Warnf("%s: Encode(): Not found, %+v", ctxAddr(ctx), adr)
					return &pb.Point{Id: adr.Id, Status: pb.Status_NOT_FOUND}, nil
				}
				goto NOMINATIM
			}
			fpoint, err := (&db.IntPoint{Lat: row.Lat.Int32, Lon: row.Lon.Int32}).ToFloat()
			if err != nil {
				s.statC <- &stat.Point{stat.EncodeFail: 1}
				log.Errorf("%s: CACHE: %+v %+v", ctxAddr(ctx), err, adr)
				return &pb.Point{Id: adr.Id, Status: pb.Status_INTERNAL_SERVER_ERROR}, err
			}
			point := &pb.Point{Id: adr.Id, Lat: fpoint.Lat, Lon: fpoint.Lon,
				Source: pb.Source_CACHE}
			s.statC <- &stat.Point{stat.EncodeSuccess: 1, stat.CacheEncodeSuccess: 1}
			return point, nil
		}

	NOMINATIM:
		fallthrough

	case pb.Source_NOMINATIM:

		if point, err := encodeHttp(ctx, s.client, adr, pb.Source_NOMINATIM); err != nil {
			log.Errorf("%s: %+v %s", ctxAddr(ctx), err, adr.Data)
		} else if point.Status == pb.Status_NOT_FOUND {
			log.Warnf("%s: NOMINATIM: Not found %+v", ctxAddr(ctx), adr)
		} else {
			s.statC <- &stat.Point{stat.EncodeSuccess: 1, stat.NominatimSuccess: 1}
			return point, nil
		}
		s.statC <- &stat.Point{stat.NominatimFail: 1}
		if adr.Source == pb.Source_NOMINATIM {
			break
		}
		fallthrough

	case pb.Source_YANDEX:

		if point, err := encodeHttp(ctx, s.client, adr, pb.Source_YANDEX); err != nil {
			log.Errorf("%s: %+v %s", ctxAddr(ctx), err, adr.Data)
		} else if point.Status == pb.Status_NOT_FOUND {
			// save to CACHE that we found nothing
			if err = db.UpdateCacheEncode(s.dbx, nil, adr); err != nil {
				s.statC <- &stat.Point{stat.EncodeFail: 1, stat.YandexFail: 1}
				log.Errorf("%s: YANDEX: %+v %+v", ctxAddr(ctx), err, point)
				return &pb.Point{Id: adr.Id, Status: pb.Status_INTERNAL_SERVER_ERROR}, err
			}
			log.Warnf("%s: YANDEX: Not found %+v", ctxAddr(ctx), adr)
		} else {
			// save to CACHE
			ipoint, err := (&db.FloatPoint{Lat: point.Lat, Lon: point.Lon}).ToInt()
			if err != nil {
				s.statC <- &stat.Point{stat.EncodeFail: 1, stat.YandexFail: 1}
				log.Errorf("%s: YANDEX: %+v %+v", ctxAddr(ctx), err, point)
				return &pb.Point{Id: adr.Id, Status: pb.Status_INTERNAL_SERVER_ERROR}, err
			} else if err = db.UpdateCacheEncode(s.dbx, ipoint, adr); err != nil {
				s.statC <- &stat.Point{stat.EncodeFail: 1, stat.YandexFail: 1}
				log.Errorf("%s: YANDEX: %+v %+v", ctxAddr(ctx), err, point)
				return &pb.Point{Id: adr.Id, Status: pb.Status_INTERNAL_SERVER_ERROR}, err
			}
			s.statC <- &stat.Point{stat.EncodeSuccess: 1, stat.YandexSuccess: 1}
			return point, nil
		}
		s.statC <- &stat.Point{stat.YandexFail: 1}

	default:

		err := fmt.Errorf("source %s not implemented", adr.Source)
		s.statC <- &stat.Point{stat.EncodeFail: 1}
		log.Errorf("%s: %+v %+v", ctxAddr(ctx), err, adr)
		return &pb.Point{Id: adr.Id, Status: pb.Status_BAD_REQUEST}, err

	}

	// FINALLY:
	s.statC <- &stat.Point{stat.EncodeFail: 1}
	log.Warnf("%s: Encode(): Not found, %+v", ctxAddr(ctx), adr)
	return &pb.Point{Id: adr.Id, Status: pb.Status_NOT_FOUND}, nil
}

func (s *GeocodingServer) decodeHttpLangs(ctx context.Context, point *pb.Point,
	langs []string, source pb.Source) *pb.Addresses {
	ch := make(chan *pb.Address)
	for _, lang := range langs {
		go func(lang string) {
			var adr *pb.Address
			if addrs, err := decodeHttp(ctx, s.client, point, lang, source); err != nil {
				log.Errorf("%s: %+v, %+v, lang '%s'", ctxAddr(ctx), err, point, lang)
			} else if len(addrs) == 0 {
				log.Warnf("%s: %v: Not found, %+v lang '%s'", ctxAddr(ctx), source, point, lang)
			} else {
				adr = addrs[0]
			}
			ch <- adr
		}(lang)
	}

	addrs := &pb.Addresses{Id: point.Id}
	for range langs {
		if adr := <-ch; adr != nil {
			addrs.Items = append(addrs.Items, adr)
		}
	}
	return addrs
}
