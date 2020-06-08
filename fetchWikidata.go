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
	"net/http"
	"strconv"

	"github.com/antonholmquist/jason"
)

// WikidataEntity is a struct representing... an entity on Wikidata.
// Its fields are private, and it should only have its methods used.
type WikidataEntity struct {
	id     string
	object *jason.Object
}

var cachedWikidata map[string]WikidataEntity

// GetFloatClaim takes a claimID and fetches the claim from the entity.
func (e WikidataEntity) GetFloatClaim(claimID string) (result float64, err error) {
	matchingClaims, err := e.object.GetObjectArray("entities", e.id, "claims", claimID)
	if err != nil {
		return
	}
	// get the last claim and parse its mainsnak value as an int
	amount, err := matchingClaims[len(matchingClaims)-1].GetString("mainsnak", "datavalue", "value", "amount")
	if err != nil {
		return
	}
	return strconv.ParseFloat(amount, 64)
}

// fetchWikidata takes a wikidata entityID and returns a WikidataEntity object,
// caching the data locally
func fetchWikidata(entityID string) (entity WikidataEntity, err error) {
	if cachedWikidata[entityID] != (WikidataEntity{}) {
		return cachedWikidata[entityID], nil
	}
	resp, err := http.Get("https://www.wikidata.org/wiki/Special:EntityData/" + entityID + ".json")
	if err != nil {
		return
	}
	v, err := jason.NewObjectFromReader(resp.Body)
	if err != nil {
		return
	}
	entity.id = entityID
	entity.object = v
	return
}
