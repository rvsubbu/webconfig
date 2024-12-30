/**
 * @license
 * Copyright Comcast.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//  @author RV

package tracing

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	// "sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testTraceID     = "0123456789abcdef0123456789abcdef"
	testCallerID    = "fedcba9876543210"
	testTraceParent = traceVersion + "-" + testTraceID + "-" + testCallerID + "-" + traceFlags

	testCallerName  = "test"
	testTraceState  = testCallerName + "=" + testCallerID

	testMyName      = "whatever"
	testMyID        = "0123456789abcdef"
)

func TestAppIDNotSet(t *testing.T) {
	// Mimic appName/appID not set by explicitly setting them to empty strs
	appID := xpcTracer.appID
	appName := xpcTracer.appName
	xpcTracer.appID = ""
	xpcTracer.appName = ""

	req := makeReq(t)
	req.Header.Add("traceparent", testTraceParent)
	_, err := GetTraceHeaders(req, false)
	require.Equal(t, traceAppIDNotSetErr, err.Error())

	xpcTracer.appID = appID
	xpcTracer.appName = appName
}

func TestNoGenerate(t *testing.T) {
	appID := xpcTracer.appID
	appName := xpcTracer.appName
	xpcTracer.appID = testMyID
	xpcTracer.appName = testMyName

	req := makeReq(t)
	headers, _ := GetTraceHeaders(req, false)
	require.Equal(t, 0, len(headers))

	xpcTracer.appID = appID
	xpcTracer.appName = appName
}

func TestOnlyParentHeader(t *testing.T) {
	appID := xpcTracer.appID
	appName := xpcTracer.appName
	xpcTracer.appID = testMyID
	xpcTracer.appName = testMyName

	req := makeReq(t)
	req.Header.Add("traceparent", testTraceParent)

	headers, _ := GetTraceHeaders(req, false)
	require.Equal(t, 2, len(headers))
	validateTraceParentHeader(t, headers)
	for _, h := range headers {
		if h.Key == "tracestate" {
			require.Equal(t, fmt.Sprintf("%s=%s", xpcTracer.appName, xpcTracer.appID), h.Value)
		}
	}

	xpcTracer.appID = appID
	xpcTracer.appName = appName
}

func TestOnlyStateHeader(t *testing.T) {
	appID := xpcTracer.appID
	appName := xpcTracer.appName
	xpcTracer.appID = testMyID
	xpcTracer.appName = testMyName

	req := makeReq(t)
	req.Header.Add("tracestate", testTraceState)
	headers, _ := GetTraceHeaders(req, false)
	require.Equal(t, 0, len(headers))

	xpcTracer.appID = appID
	xpcTracer.appName = appName
}

func TestParentPlusStateHeaders(t *testing.T) {
	appID := xpcTracer.appID
	appName := xpcTracer.appName
	xpcTracer.appID = testMyID
	xpcTracer.appName = testMyName

	req := makeReq(t)
	req.Header.Add("traceparent", testTraceParent)
	req.Header.Add("tracestate", testTraceState)
	headers, _ := GetTraceHeaders(req, false)
	require.Equal(t, 2, len(headers))
	validateTraceParentHeader(t, headers)
	for _, h := range headers {
		if h.Key == "tracestate" {
			require.Equal(t, fmt.Sprintf("%s=%s,%s", xpcTracer.appName, xpcTracer.appID,testTraceState), h.Value)
		}
	}

	xpcTracer.appID = appID
	xpcTracer.appName = appName
}

func TestTraceHeaderGeneration(t *testing.T) {
	appID := xpcTracer.appID
	appName := xpcTracer.appName
	xpcTracer.appID = testMyID
	xpcTracer.appName = testMyName

	testTraceHeaderGeneration(t)

	xpcTracer.appID = appID
	xpcTracer.appName = appName
}

func validateTraceParentHeader(t *testing.T, headers []TraceHeader) {
	for _, h := range headers {
		if h.Key == "traceparent" {
			traceComponents := strings.Split(h.Value, "-")
			require.Equal(t, 4, len(traceComponents), 4)
			require.Equal(t, traceVersion, traceComponents[0])
			require.Equal(t, testTraceID, traceComponents[1])
			require.Equal(t, xpcTracer.appID, traceComponents[2])
			require.Equal(t, traceFlags, traceComponents[3])
		}
	}
}

func makeReq(t *testing.T) *http.Request {
	url := "/testurl" 
	var body []byte
	req, err := http.NewRequest("GET", url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func testTraceHeaderGeneration(t *testing.T) {
	req := makeReq(t)
	headers, _ := GetTraceHeaders(req, true)
	require.Equal(t, 2, len(headers))
	for _, h := range headers {
		if h.Key == "traceparent" {
			traceComponents := strings.Split(h.Value, "-")
			require.Equal(t, 4, len(traceComponents))
			require.Equal(t, traceVersion, traceComponents[0])
			// We don't know the ID here, we can only check it is 32 digits long
			require.Equal(t, 32, len(traceComponents[1]))
			require.Equal(t, xpcTracer.appID, traceComponents[2])
			require.Equal(t, traceFlags, traceComponents[3])
		}
		if h.Key == "tracestate" {
			require.Equal(t, fmt.Sprintf("%s=%s", xpcTracer.appName, xpcTracer.appID), h.Value)
		}
	}
}

/*
	// These tests/benchmarks are to test mutex based random generation
	// We are not going to use mutex as of now, as using one source
	// per goroutine approach is faster
	func TestTraceHeaderGenerationGlobal(t *testing.T) {
		setGlobal(true)
		testTraceHeaderGeneration(t)
		setGlobal(false)
	}

	func BenchmarkSharedRandGen(b *testing.B) {
		setGlobal(true)
		url := "/testurl" 
		var body []byte
		req, _ := http.NewRequest("GET", url, bytes.NewReader(body))
		var wg sync.WaitGroup
		for n := 0; n < b.N; n++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				GetTraceHeaders(req, true)
			}()
		}
		wg.Wait()
		setGlobal(false)
	}

	func BenchmarkIndependentRandGen(b *testing.B) {
		url := "/testurl" 
		var body []byte
		req, _ := http.NewRequest("GET", url, bytes.NewReader(body))
		var wg sync.WaitGroup
	
		for n := 0; n < b.N; n++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				GetTraceHeaders(req, true)
			}()
		}
		wg.Wait()
	}
*/
