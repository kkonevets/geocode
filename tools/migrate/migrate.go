// Migrate x-company-devices.geocode_cache table to a new structure
// | id | source | address                        | address_hash        | lat    | lon    | lang  |
// |:--:|:------:|:------------------------------:|:-------------------:|:------:|:------:|-------|
// | 3  | yandex | Россия, Москва, Совeтская ул 4 | 3145177133033400300 | 547304 | 559946 | ru-RU |

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/X-Company/geocoding/cache/db"
	"github.com/X-Company/geocoding/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type RowOld struct {
	Id       int32  `db:"id"`
	Common   string `db:"common"`
	PointStr string `db:"hash"`
}

type Common struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

func updateFrom(rows []*db.CacheRow) string {
	query_pat := `
	UPDATE geocode_cache as gc SET
	    source = c.source,
	    address = c.address,
            address_hash = c.address_hash,
            lat = c.lat,
            lon = c.lon,
            encode = FALSE
	FROM (VALUES %s)
        AS c(id, source, address, address_hash, lat, lon)
	WHERE c.id = gc.id;`

	val_pat := "(%d ,'%s', '%s', %d, %d, %d)"
	var values []string
	for _, row := range rows {
		val := fmt.Sprintf(val_pat, row.Id, row.Source.String,
			row.Address, row.AddressHash, row.Lat.Int32, row.Lon.Int32)
		values = append(values, val)
	}

	query := fmt.Sprintf(query_pat, strings.Join(values, ","))
	return query
}

func main() {
	conf := config.MustLoad()
	postgres := conf.Cache
	conn := sqlx.MustConnect("postgres", postgres.ConnectionParams)

	conn.MustExec(`
        /* a point can have multiple addresses for each language */
        ALTER TABLE geocode_cache
            DROP CONSTRAINT IF EXISTS geocode_cache_unique_hash,
            DROP CONSTRAINT IF EXISTS geocode_cache_unique_hashint;
        DROP INDEX IF EXISTS geocode_cache_hashint;

	ALTER TABLE geocode_cache
	    RENAME COLUMN "commonAt" TO updated;
        ALTER TABLE geocode_cache
            ALTER COLUMN updated SET DEFAULT now(),
            DROP COLUMN "createdAt",
            DROP COLUMN "updatedAt",
            DROP COLUMN hashint,
	    ADD COLUMN lat INTEGER,
	    ADD COLUMN lon INTEGER,
	    ADD COLUMN source VARCHAR,
            ADD COLUMN lang VARCHAR,
	    ADD COLUMN address VARCHAR,
	    ADD COLUMN address_hash BIGINT,
            ADD COLUMN encode BOOLEAN;

        COMMENT ON COLUMN geocode_cache.encode IS 'Distinguish encoding/decoding address';

        CREATE INDEX geocode_cache_lat_lon
            ON geocode_cache(lat, lon);
        CREATE INDEX geocode_cache_address_hash
            ON geocode_cache(address_hash);`)

	i := 0
	row := RowOld{}
	rows, err := conn.Queryx("SELECT id, common, hash FROM geocode_cache")
	if err != nil {
		log.Fatalln(err)
	}
	var max_size int = 1000
	var chunk []*db.CacheRow
	for rows.Next() {
		if err := rows.StructScan(&row); err != nil {
			log.Fatalln(err)
		}

		com := Common{}
		if err = json.Unmarshal([]byte(row.Common), &com); err != nil {
			log.Panicf("could not parse message: %s", row.Common)
		}

		splited := strings.Split(row.PointStr, ",")
		fpoint := db.FloatPoint{}
		if fpoint.Lat, err = strconv.ParseFloat(splited[0], 64); err != nil {
			log.Panicf("%+v, %+v", err, row.PointStr)
		}
		if fpoint.Lon, err = strconv.ParseFloat(splited[1], 64); err != nil {
			log.Panicf("%+v, %+v", err, row.PointStr)
		}
		ipoint, err := fpoint.ToInt()
		if err != nil {
			log.Panicf("%s", err)
		}
		address_hash := db.StringHash(com.Address)

		escaped := strings.ReplaceAll(com.Address, "'", "''")
		chunk = append(chunk, &db.CacheRow{Id: row.Id, Source:  //
		sql.NullString{String: com.Type, Valid: true}, Address: escaped,
			AddressHash: address_hash, Lat: sql.NullInt32{Int32: ipoint.Lat, Valid: true},
			Lon: sql.NullInt32{Int32: ipoint.Lon, Valid: true}})

		// fmt.Printf("id: %d, %#v\n", row.Id, row_new)
		if len(chunk) == max_size {
			query := updateFrom(chunk)
			// fmt.Println(query)
			conn.MustExec(query)
			chunk = chunk[:0]
			i++
			fmt.Printf("\r%d", i*max_size)
		}
	}

	// finally
	if len(chunk) > 0 {
		query := updateFrom(chunk)
		conn.MustExec(query)
		fmt.Printf("\r%d", i*max_size+len(chunk))
	}
	fmt.Println()

	conn.MustExec(`
        ALTER TABLE geocode_cache
            DROP COLUMN common,
            DROP COLUMN hash,
            ALTER COLUMN address SET NOT NULL,
            ALTER COLUMN address_hash SET NOT NULL,
            ALTER COLUMN encode SET NOT NULL;`)
}
