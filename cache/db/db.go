package db

import (
	"database/sql"
	"fmt"
	"hash/fnv"
	"strings"

	pb "github.com/X-Company/geocoding/proto/geocoding"
	"github.com/jmoiron/sqlx"
)

// scale factor to convert between int32 and float64 point representation,
// represents 4 digit mantissa
const scale float64 = 10000

// row of PSQL table geocode_cache
type (
	CacheRow struct {
		Id          int32          `db:"id"`
		Source      sql.NullString `db:"source"`
		Address     string         `db:"address"`
		AddressHash int64          `db:"address_hash"`
		Lat         sql.NullInt32  `db:"lat"`
		Lon         sql.NullInt32  `db:"lon"`
		Updated     sql.NullTime   `db:"updated"`
		Lang        sql.NullString `db:"lang"`
		Encode      bool           `db:"encode"`
	}

	DataProcessedRow struct {
		Id        int64        `db:"id"`
		Extra     string       `db:"extra"`
		Address   string       `db:"address"`
		AddressDt sql.NullTime `db:"address_dt"`
	}
)

// computes hash number out of string using fnv-1a algorithm
func StringHash(str string) int64 {
	alg := fnv.New64a()
	alg.Write([]byte(str))
	// uint64 to int64: 18446744073709551615 becomes -1
	return int64(alg.Sum64())
}

type IntPoint struct {
	Lat int32
	Lon int32
}

type FloatPoint struct {
	Lat float64
	Lon float64
}

func checkPoint(point *FloatPoint) error {
	if point.Lat < -90 || point.Lat > 90 {
		return fmt.Errorf("%+v latitude is out of bound", point)
	} else if point.Lon < -180 || point.Lon > 180 {
		return fmt.Errorf("%+v longitude is out of bound", point)
	}
	return nil
}

// convert point coordinates to integers
func (point *FloatPoint) ToInt() (*IntPoint, error) {
	if err := checkPoint(point); err != nil {
		return &IntPoint{}, err
	}
	return &IntPoint{int32(point.Lat * scale), int32(point.Lon * scale)}, nil
}

// convert pair of integers to a point as pair of floats
func (point *IntPoint) ToFloat() (*FloatPoint, error) {
	fpoint := &FloatPoint{float64(point.Lat) / scale, float64(point.Lon) / scale}
	if err := checkPoint(fpoint); err != nil {
		return &FloatPoint{}, err
	}
	return fpoint, nil
}

func sqlLang(lang string) string {
	if lang == "" {
		return "NULL"
	}
	return fmt.Sprintf("'%s'", lang)
}

// Update all addresses of a point
func UpdateCacheDecode(dbx *sqlx.DB, ipoint *IntPoint, addrs *pb.Addresses) error {
	const pat string = `
        DELETE FROM geocode_cache where (lat, lon, lang) IN (%s) AND encode=FALSE;
        INSERT INTO
        geocode_cache(lat, lon, source, lang, address, address_hash, encode, updated)
        VALUES %s;`

	var delVals, insVals []string
	const delValPat string = "(%d, %d, %s)"
	const insValPat string = "(%d, %d, '%s', %s, '%s', %d, FALSE, CURRENT_TIMESTAMP)"
	for _, item := range addrs.Items {
		lang := sqlLang(item.Lang)
		val := fmt.Sprintf(delValPat, ipoint.Lat, ipoint.Lon, lang)
		delVals = append(delVals, val)

		hash := StringHash(item.Data) // hash should be taken of a raw string
		escaped := strings.ReplaceAll(item.Data, "'", "''")
		val = fmt.Sprintf(insValPat, ipoint.Lat, ipoint.Lon,
			item.Source, lang, escaped, hash)
		insVals = append(insVals, val)
	}

	query := fmt.Sprintf(pat, strings.Join(delVals, ","), strings.Join(insVals, ","))
	_, err := dbx.Exec(query)

	return err
}

// Update one address in CACHE. ipoint==nil indicates address was not found in all sources
func UpdateCacheEncode(dbx *sqlx.DB, ipoint *IntPoint, adr *pb.Address) error {
	const pat string = `
        DELETE FROM geocode_cache where address_hash=%d AND encode=TRUE;
        INSERT INTO
        geocode_cache(lat, lon, source, address, address_hash, encode, updated)
        VALUES %s;`

	hash := StringHash(adr.Data)
	escaped := strings.ReplaceAll(adr.Data, "'", "''")

	var valStr string
	if ipoint != nil {
		valStr = fmt.Sprintf("(%d, %d, '%s', '%s', %d, TRUE, CURRENT_TIMESTAMP)",
			ipoint.Lat, ipoint.Lon, adr.Source, escaped, hash)
	} else {
		valStr = fmt.Sprintf("(NULL, NULL, '%s', '%s', %d, TRUE, CURRENT_TIMESTAMP)",
			adr.Source, escaped, hash)
	}

	query := fmt.Sprintf(pat, hash, valStr)
	_, err := dbx.Exec(query)

	return err
}
