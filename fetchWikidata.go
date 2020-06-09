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
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/antonholmquist/jason"
)

// WikidataEntity is a struct representing... an entity on Wikidata.
// Its fields are private, and it should only have its methods used.
type WikidataEntity struct {
	id     string
	object *jason.Object
}

// WikidataReference is a struct representing a reference for a Wikidata claim.
type WikidataReference struct {
	Found     bool
	URL       string
	Retrieved string
	Published string
	Title     string
	Lang      string
	Website   string
}

var cachedWikidata = map[string]WikidataEntity{}
var cachedCitationResults = map[string][]string{}

// GetFloatClaim takes a claimID and fetches the claim from the entity.
func (e WikidataEntity) GetFloatClaim(claimID string) (result float64, reference WikidataReference, err error) {
	reference = WikidataReference{}
	matchingClaims, err := e.object.GetObjectArray("entities", e.id, "claims", claimID)
	if err != nil {
		return
	}
	// get the last claim and parse its mainsnak value as an int
	amount, err := matchingClaims[len(matchingClaims)-1].GetString("mainsnak", "datavalue", "value", "amount")
	if err != nil {
		return
	}

	refDetails, err := matchingClaims[len(matchingClaims)-1].GetObjectArray("references")
	if err == nil {
		lastRef, err := refDetails[len(refDetails)-1].GetObject("snaks")
		if err == nil {
			url, err := fetchSingularProp(lastRef, "P854").GetString("datavalue", "value")
			if err == nil {
				reference.Found = true
				reference.URL = url
				retrieved := parseDateIfAppropriate(fetchSingularProp(lastRef, "P813").GetString("datavalue", "value", "time"))
				if retrieved != "" {
					reference.Retrieved = retrieved
				}
				published := parseDateIfAppropriate(fetchSingularProp(lastRef, "P577").GetString("datavalue", "value", "time"))
				if published == "" {
					quals, err := matchingClaims[len(matchingClaims)-1].GetObject("qualifiers")
					if err == nil {
						reference.Retrieved = parseDateIfAppropriate(fetchSingularProp(quals, "P585").GetString("datavalue", "value", "time"))
					}
				} else {
					reference.Published = published
				}
			}
		}
	}

	result, err = strconv.ParseFloat(amount, 64)
	return
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
	cachedWikidata[entityID] = entity
	return
}

func parseDateIfAppropriate(date string, err error) string {
	if err == nil {
		parsedDate, err := time.Parse("+2006-01-02T15:04:05Z", date)
		if err == nil {
			return parsedDate.Format("2006-01-02")
		}
	}
	return ""
}

func (r *WikidataReference) loadURLCitation() {
	if result, ok := cachedCitationResults[r.URL]; ok {
		r.Title = result[0]
		r.Lang = result[1]
		r.Website = result[2]
	} else {
		resp, err := http.Get("https://en.wikipedia.org/api/rest_v1/data/citation/mediawiki/" + url.PathEscape(r.URL) + "?action=query&format=json")
		if err == nil {
			v, err := jason.NewValueFromReader(resp.Body)
			if err == nil {
				results, err := v.ObjectArray()
				if err == nil {
					title, err := results[0].GetString("title")
					if err == nil {
						r.Title = title
					}
					lang, err := results[0].GetString("language")
					if err == nil {
						r.Lang = lang
					}
					site, err := results[0].GetString("websiteTitle")
					if err == nil {
						r.Website = site
					}
				}
			}
		}
		cachedCitationResults[r.URL] = []string{r.Title, r.Lang, r.Website}
	}
}

func (r WikidataReference) refToCiteWeb() string {
	if r.Title == "" || titleIsURL(r.Title) {
		var builder strings.Builder
		builder.WriteString("[" + r.URL + "]")
		if r.Published != "" {
			writeDateToBuilder(&builder, r.Published, "published")
		}
		if r.Retrieved != "" {
			writeDateToBuilder(&builder, r.Retrieved, "retrieved")
		}
		return builder.String()
	}
	return "{{cite web|url=" + r.URL + "|title=" + citeClean(r.Title) + "|date=" + r.Published +
		"|access-date=" + r.Retrieved + "|language=" + r.Lang + "|website=" + citeClean(r.Website) + "}}"
}

func fetchSingularProp(obj *jason.Object, propID string) *jason.Object {
	arr, err := obj.GetObjectArray(propID)
	if err != nil {
		return &jason.Object{}
	}
	return arr[0]
}

func titleIsURL(s string) bool {
	_, err := url.ParseRequestURI(s)
	return (err == nil)
}

// see https://en.wikipedia.org/wiki/Help:CS1_errors#invisible_char
var charsToClean = []string{
	"\u00A0", "\u00AD", "\uFFFD", "\u200A",
	"\u200B", "\u200D", "\u0009", "\u0010",
	"\u0013", "\u007F"}

func citeClean(s string) string {
	working := strings.ReplaceAll(s, "|", "{{!}}")
	for _, check := range charsToClean {
		working = strings.ReplaceAll(working, check, "")
	}
	return working
}

func writeDateToBuilder(builder *strings.Builder, date string, dateType string) {
	builder.WriteString(" (")
	builder.WriteString(dateType)
	builder.WriteString(" {{date|")
	builder.WriteString(date)
	builder.WriteString("}})")
}
