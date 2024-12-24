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
package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rdkcentral/webconfig/util"
	log "github.com/sirupsen/logrus"
	"gotest.tools/assert"
)

func TestUpstreamConnector(t *testing.T) {
	server := NewWebconfigServer(sc, true)

	// setup upstream mock server
	mockedUpstreamResponse := util.RandomBytes(100, 150)
	upstreamMockServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(mockedUpstreamResponse)
		}))
	server.SetUpstreamHost(upstreamMockServer.URL)
	targetUpstreamHost := server.UpstreamHost()
	assert.Equal(t, upstreamMockServer.URL, targetUpstreamHost)
	defer upstreamMockServer.Close()

	// ==== post new data ====
	mac := util.GenerateRandomCpeMac()
	header := make(http.Header)
	header.Set("Red", "maroon")
	header.Set("Orange", "auburn")
	header.Set("Yellow", "amber")
	bbytes := []byte("hello world")
	var err error
	fields := log.Fields{}
	ctx := context.Background()
	_, _, err = server.PostUpstream(ctx, mac, header, bbytes, fields)
	assert.NilError(t, err)
}
