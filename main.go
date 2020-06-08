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
	"log"
	"regexp"
	"strconv"
	"strings"

	"cgt.name/pkg/go-mwclient/params"
	"github.com/antonholmquist/jason"
	"github.com/mashedkeyboard/ybtools"
)

var dataslotRegex *regexp.Regexp

func main() {
	ybtools.SetupBot("Wikidatable", "Yapperbot")

	dataslotRegex = regexp.MustCompile(`(?i)<!-- *DATASLOT:([^:]+?):([^:]+?) *-->`)

	w := ybtools.CreateAndAuthenticateClient()

	configs, err := ybtools.LoadJSONFromPageTitle(w, "User:Yapperbot/Wikidatable.json").GetStringArray("configurations")
	if err != nil {
		log.Fatal("Failed to get configurations with error ", err)
	}

	for _, config := range configs {
		configJSON := ybtools.LoadJSONFromPageTitle(w, config)

		dataJSON, err := configJSON.GetString("data")
		if err != nil {
			logFailureMessage("data", config, err)
			continue
		}
		template, err := configJSON.GetString("template")
		if err != nil {
			logFailureMessage("template", config, err)
			continue
		}
		headings, err := configJSON.GetObject("headings")
		if err != nil {
			logFailureMessage("headings", config, err)
			continue
		}

		data := ybtools.LoadJSONFromPageTitle(w, dataJSON)
		templateText, err := ybtools.FetchWikitextFromTitle(w, template)
		if err != nil {
			logFailureMessage("template text", config, err)
			continue
		}

		templatesDone := map[string]bool{}

		for _, match := range dataslotRegex.FindAllStringSubmatch(templateText, -1) {
			// don't process a template (heading + lookup) more than once,
			// even if it appears on the page more than once - we replace them all
			if templatesDone[match[0]] {
				continue
			} else {
				templatesDone[match[0]] = true
			}

			heading, err := headings.GetObject(match[1])
			if err != nil {
				logFailureMessage("heading "+match[1], config, err)
				continue
			}

			dataKeys, err := data.GetObject(match[2])
			if err != nil {
				logFailureMessage("data key "+match[2], config, err)
				continue
			}

			dataProp, err := heading.GetString("data")
			if err != nil {
				logFailureMessage("config heading data for "+match[1], config, err)
				continue
			}

			usePer := true
			perProp, err := heading.GetString("per")
			if err != nil {
				usePer = false
			}

			claim, err := loadEntityAndClaimFromJSON(dataKeys, dataProp)
			if err != nil {
				logFailureMessage("loadEntityAndClaimFromJSON for "+dataProp+" in "+match[1], config, err)
			}

			if usePer {
				perClaim, err := loadEntityAndClaimFromJSON(dataKeys, perProp)
				if err != nil {
					logFailureMessage("loadEntityAndClaimFromJSON for perProp "+perProp+" in "+match[1], config, err)
				}
				claim = (claim / perClaim) * 100.0
			}

			// format to 1dp
			templateText = strings.ReplaceAll(templateText, match[0], strconv.FormatFloat(claim, 'f', 10, 64))
		}

		err = w.Edit(params.Values{
			"title":    config[:len(config)-5], // take off the .json on the end of the config page to get our new name
			"text":     templateText,
			"summary":  "Updating Wikidatatable from template",
			"notminor": "true",
			"bot":      "true",
		})
		if err != nil {
			log.Fatal("Error raised when editing, can't handle, so failing. Error was ", err)
		}
	}
}

func loadEntityAndClaimFromJSON(dataKeys *jason.Object, dataProp string) (claim float64, err error) {
	entityForProp, err := dataKeys.GetString(dataProp)
	if err != nil {
		return
	}

	return fetchEntityAndClaim(entityForProp, dataProp)
}

func fetchEntityAndClaim(entityID string, claimID string) (float64, error) {
	entity, err := fetchWikidata(entityID)
	if err != nil {
		return 0, err
	}

	return entity.GetFloatClaim(claimID)
}

func logFailureMessage(thing string, config string, err error) {
	log.Println("Failed to get", thing, "for config", config, "with error", err)
}
