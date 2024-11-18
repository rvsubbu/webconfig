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
	"net/http"
	"testing"

	"github.com/rdkcentral/webconfig/common"
	"gotest.tools/assert"
)

func TestString(t *testing.T) {
	s := "112233445566"
	c := ToColonMac(s)
	expected := "11:22:33:44:55:66"
	assert.Equal(t, c, expected)
}

func TestGetAuditId(t *testing.T) {
	auditId := GetAuditId()
	assert.Equal(t, len(auditId), 32)
}

func TestTelemetryQuery(t *testing.T) {
	header := http.Header{}
	header.Set(common.HeaderProfileVersion, "2.0")
	header.Set(common.HeaderModelName, "TG1682G")
	header.Set(common.HeaderPartnerID, "comcast")
	header.Set(common.HeaderAccountID, "1234567890")
	header.Set(common.HeaderFirmwareVersion, "TG1682_3.14p9s6_PROD_sey")
	mac := "567890ABCDEF"
	qstr := GetTelemetryQueryString(header, mac, "", "comcast")

	expected := "env=PROD&partnerId=comcast&version=2.0&model=TG1682G&accountId=1234567890&firmwareVersion=TG1682_3.14p9s6_PROD_sey&estbMacAddress=567890ABCDF1&ecmMacAddress=567890ABCDEF"
	assert.Equal(t, qstr, expected)

	// with queryParams
	queryParams := "stormReadyWifi=true"
	qstr = GetTelemetryQueryString(header, mac, queryParams, "comcast")
	expected = "env=PROD&partnerId=comcast&version=2.0&model=TG1682G&accountId=1234567890&firmwareVersion=TG1682_3.14p9s6_PROD_sey&estbMacAddress=567890ABCDF1&ecmMacAddress=567890ABCDEF&stormReadyWifi=true"
	assert.Equal(t, qstr, expected)
}

func TestIsValidUTF8(t *testing.T) {
	b1 := []byte(`{"foo":"bar","hello":123,"world":true}`)
	assert.Assert(t, IsValidUTF8(b1))

	b2 := RandomBytes(100, 150)
	assert.Assert(t, !IsValidUTF8(b2))
}

func TestTelemetryQueryWithWanMac(t *testing.T) {
	header := http.Header{}
	header.Set(common.HeaderProfileVersion, "2.0")
	header.Set(common.HeaderModelName, "TG1682G")
	header.Set(common.HeaderPartnerID, "comcast")
	header.Set(common.HeaderAccountID, "1234567890")
	header.Set(common.HeaderFirmwareVersion, "TG1682_3.14p9s6_PROD_sey")
	mac := "567890ABCDEF"
	header.Set(common.HeaderWanMac, "567890ABCDEF")
	qstr := GetTelemetryQueryString(header, mac, "", "comcast")

	expected := "env=PROD&partnerId=comcast&version=2.0&model=TG1682G&accountId=1234567890&firmwareVersion=TG1682_3.14p9s6_PROD_sey&estbMacAddress=567890ABCDEF"
	assert.Equal(t, qstr, expected)
}
