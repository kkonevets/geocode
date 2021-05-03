// Print collisions of a hash function in database
package main

import (
	"fmt"
	"log"

	"github.com/X-Company/geocoding/cache/db"
	"github.com/X-Company/geocoding/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	conf := config.MustLoad()
	postgres := conf.Cache
	conn := sqlx.MustConnect("postgres", postgres.ConnectionParams)

	row := db.CacheRow{}
	rows, err := conn.Queryx("SELECT id, address, address_hash FROM geocode_cache ORDER BY address_hash")
	if err != nil {
		log.Fatal(err)
	}
	var hash_prev int64 = 0
	var address_prev string = ""
	for rows.Next() {
		err := rows.StructScan(&row)
		if err != nil {
			log.Fatalln(err)
		}

		if hash_prev == row.AddressHash && address_prev != row.Address {
			fmt.Printf("%#v\n", row)

		}
		hash_prev = row.AddressHash
		address_prev = row.Address
	}
}
