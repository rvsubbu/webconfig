/**
* Copyright 2021 Comcast Cable Communications Management, LLC
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
* http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
* SPDX-License-Identifier: Apache-2.0
*/
package util

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rdkcentral/webconfig/common"
	log "github.com/sirupsen/logrus"
)

func ValidateMac(mac string) bool {
	if len(mac) != 12 {
		return false
	}
	for _, r := range mac {
		if r < 48 || r > 70 || (r > 57 && r < 65) {
			return false
		}
	}
	return true
}

func ValidatePokeQuery(values url.Values) (string, error) {
	// handle ?doc=xxx
	if docQueryParamStrs, ok := values["doc"]; ok {
		if len(docQueryParamStrs) > 1 {
			err := fmt.Errorf("multiple doc parameter is not allowed")
			return "", common.NewError(err)
		}

		qparams := strings.Split(docQueryParamStrs[0], ",")
		if len(qparams) > 1 {
			err := fmt.Errorf("multiple doc parameter is not allowed")
			return "", common.NewError(err)
		}

		queryStr := qparams[0]
		if !Contains(common.SupportedPokeDocs, queryStr) {
			err := fmt.Errorf("invalid query parameter: %v", queryStr)
			return "", common.NewError(err)

		}
		return queryStr, nil
	}

	// handle ?route=xxx
	if qparams, ok := values["route"]; ok {
		if len(qparams) > 1 {
			err := fmt.Errorf("multiple route parameter is not allowed")
			return "", common.NewError(err)
		}

		qparams := strings.Split(qparams[0], ",")
		if len(qparams) > 1 {
			err := fmt.Errorf("multiple route parameter is not allowed")
			return "", common.NewError(err)
		}

		queryStr := qparams[0]
		if !Contains(common.SupportedPokeRoutes, queryStr) {
			err := fmt.Errorf("invalid query parameter: %v", queryStr)
			return "", common.NewError(err)

		}
		return queryStr, nil
	}

	// return default
	return "primary", nil
}

func ValidateQueryParams(r *http.Request, fields log.Fields) error {
	groupIdValues, ok := r.URL.Query()["group_id"]
	if !ok || len(groupIdValues) == 0 {
		return common.NewError(common.ErrInvalidQueryParams)
	}
	fields["group_id"] = groupIdValues[0]
	r.Header.Set(common.HeaderDocName, groupIdValues[0])

	subdocIds := strings.Split(groupIdValues[0], ",")
	if len(subdocIds) == 0 {
		return common.NewError(common.ErrInvalidQueryParams)
	}

	if len(subdocIds) > 0 && subdocIds[0] != "root" {
		return common.NewError(common.ErrInvalidQueryParams)
	}

	for _, subdocId := range subdocIds[1:] {
		if _, ok := common.SubdocBitIndexMap[subdocId]; !ok {
			return common.NewError(common.ErrInvalidQueryParams)
		}
	}

	ifNoneMatch := r.Header.Get(common.HeaderIfNoneMatch)
	if len(ifNoneMatch) == 0 {
		return common.NewError(common.ErrInvalidQueryParams)
	}

	versions := strings.Split(ifNoneMatch, ",")
	if len(versions) != len(subdocIds) {
		return common.NewError(common.ErrInvalidQueryParams)
	}
	return nil
}
