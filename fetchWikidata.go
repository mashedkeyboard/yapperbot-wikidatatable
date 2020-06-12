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
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/avast/retry-go"
	"github.com/knakk/rdf"
	"github.com/knakk/sparql"
)

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

var cachedCitationResults = map[string][]string{}

// GetFloatClaimAndReference takes a claimID and fetches the claim from the entity,
// along with a reference if one is available.
func GetFloatClaimAndReference(entityID, claimID string) (result float64, reference WikidataReference, err error) {
	var res *sparql.Results
	retry.Do(
		func() error {
			res, err = sparqlRepo.Query(generateQueryFor(entityID, claimID))
			return err
		},
		retry.Attempts(3),
	)
	if err != nil {
		return
	}

	reference = WikidataReference{}

	var solution map[string]rdf.Term

	if len(res.Solutions()) < 1 {
		err = fmt.Errorf("no solutions found for given parameters")
		return
	}

	solution = res.Solutions()[0]
	amount := solution["val"].String()

	if refLabel, ok := solution["refLabel"]; ok {
		reference.Found = true
		reference.Title = refLabel.String()
	}

	if refURL, ok := solution["url"]; ok {
		reference.Found = true
		reference.URL = refURL.String()
	}

	if reference.Found {
		retrieved := parseDateIfAppropriate(solution, "retrieved")
		if retrieved != "" {
			reference.Retrieved = retrieved
		}
		published := parseDateIfAppropriate(solution, "published")
		if published == "" {
			reference.Published = parseDateIfAppropriate(solution, "pointintime")
		} else {
			reference.Published = published
		}
	}

	result, err = strconv.ParseFloat(amount, 64)
	return
}

func parseDateIfAppropriate(term map[string]rdf.Term, field string) string {
	if dateField, ok := term[field]; ok {
		var parsedDate time.Time
		var err error
		var date string = dateField.String()

		if date != "" {
			parsedDate, err = time.Parse("+2006-01-02T15:04:05Z", date)
			if err == nil {
				return parsedDate.Format("2006-01-02")
			}

			// Try both month and year-only accuracy, in case it's just that it's not accurate enough
			// this doesn't work by just leaving it, because 00 is not a valid month or day
			parsedDate, err = time.Parse("+2006-01-00T00:00:00Z", date)
			if err == nil {
				return parsedDate.Format("January 2006")
			}
			parsedDate, err = time.Parse("+2006-00-00T00:00:00Z", date)
			if err == nil {
				return parsedDate.Format("2006")
			}
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
					if err == nil && r.Title == "" {
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
	var builder strings.Builder
	if r.URL == "" {
		builder.WriteString(r.Title)
		writeDatesToBuilderWhereNeeded(&builder, r)
		return builder.String()
	}
	r.loadURLCitation()
	if r.Title == "" || titleIsURL(r.Title) {
		builder.WriteString("[" + r.URL + "]")
		writeDatesToBuilderWhereNeeded(&builder, r)
		return builder.String()
	}
	return "{{cite web|postscript=none|url=" + r.URL + "|title=" + citeClean(r.Title) + "|date=" + r.Published +
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

func writeDatesToBuilderWhereNeeded(builder *strings.Builder, r WikidataReference) {
	if r.Published != "" {
		writeDateToBuilder(builder, r.Published, "published")
	}
	if r.Retrieved != "" {
		writeDateToBuilder(builder, r.Retrieved, "retrieved")
	}
}

func writeDateToBuilder(builder *strings.Builder, date string, dateType string) {
	builder.WriteString(" (")
	builder.WriteString(dateType)
	builder.WriteString(" {{date|")
	builder.WriteString(date)
	builder.WriteString("}})")
}
