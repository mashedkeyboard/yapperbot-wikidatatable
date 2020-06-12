package main

//
// Wikidatable, the Wikidata table updater bot for Wikipedia
// Copyright (C) 2020 Naypta

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//

import (
	"bytes"
	"log"
	"time"

	"github.com/knakk/sparql"
)

const queries = `
# This query generates all available bits, sorts them by highest rank,
# and then if they're all the same rank, sorts them by date.
# It then picks the top one: i.e. preferred rank if available,
# otherwise the latest standard rank. The ORDER BY also makes sure that
# where multiple references are available, if any have data,
# those should be returned.
# tag: lookup-query
SELECT ?val ?pointintime ?refLabel ?url ?retrieved ?published ?rank WHERE {
  wd:{{.Entity}} p:{{.Property}} ?statement.
  ?statement ps:{{.Property}} ?val;
    wikibase:rank ?rank.
  OPTIONAL { ?statement pq:P585 ?pointintime. }
  OPTIONAL { 
    ?statement prov:wasDerivedFrom ?refnode.
    OPTIONAL { ?refnode pr:P248 ?ref. }
    OPTIONAL { ?refnode pr:P854|pr:P856|pr:P1065 ?url. }
    OPTIONAL { ?refnode pr:P577 ?published. }
    OPTIONAL { ?refnode pr:P813 ?retrieved. }
  }
  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
}
ORDER BY DESC(?rank) DESC(?pointintime) DESC(?published) DESC(?retrieved) DESC(?url) DESC(?refLabel) LIMIT 1
`

var queryBank sparql.Bank
var sparqlRepo *sparql.Repo

func init() {
	f := bytes.NewBufferString(queries)
	queryBank = sparql.LoadBank(f)

	var err error
	sparqlRepo, err = sparql.NewRepo("https://query.wikidata.org/sparql",
		sparql.Timeout(time.Millisecond*1500),
	)

	if err != nil {
		log.Fatal("Failed to set up SPARQL repository for Wikidata with error ", err)
	}
}

func generateQueryFor(entity, property string) string {
	q, err := queryBank.Prepare("lookup-query", struct{ Entity, Property string }{entity, property})
	if err != nil {
		log.Fatal("Failed to generate lookup query with error ", err)
	}
	return q
}
