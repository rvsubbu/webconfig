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

/*
	W3C Doc: https://www.w3.org/TR/trace-context-1

	Usage:
		1. AppID should be set in the config
			Config Template (from HC):
			-------------------------------------------
				"Tracing": {
					// This identifies the microservice
					"AppID": "0000000000000001"
				},
			-------------------------------------------
		2. Call GetTracingHeader with a http Request, and it will return a modified
			traceparent/tracestate header
		3. Set it in req.Header
		4. If the generate flag is set in GetTracingHeader and if the input req doesn't
			have a traceparent header, these two headers will be created
	TODO:
		Also pass a Kafka message as an input to do the above

	Spec:
	traceparent header has 4 sections
		"<version>-<traceID>-<callerID>-<Flags>"
		version is 2 digits, traceID is 32 digits, callerIF is 16, and flags is 2
	if traceparent is present in headers, replace the callerID with app's ID

	Template - hc's ID defaults to "0000000000000001" and can be changed in config

	If tracestate header exists, add "<appName>=<appID>," to the beginning of the header
	else set it to "<appName>=<appID>"

	As of now, we don't generate traceparent if the header is not passed in
	Any trace flag setting that tells us to regenerate the traceID is ignored.
	Version, TraceID, and trace flags are passed as-is. Only CallerID is modified
	TraceState will be generated if it doesn't exist

	Epic: XPC-16688
	HC Ticket: ODP-25026
	Xapproxy Ticket: XPC-17946

	Changes:
		otel implements their own traceparent/tracestate propagation and this clashes with
		our home-grown propagation. If otel is enabled, suppress the home-grown propagation.
	TODO: Change homegrown propagation too to a per API basis instead of per app basis
*/

package tracing

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	// "sync"
	"time"
	log "github.com/sirupsen/logrus"
	"github.com/rdkcentral/webconfig/common"
)

const (
	// These are two completely internal
	traceVersion = "01"
	traceFlags   = "00"

	defaultAppID = "0000000000000001"

	traceAppIDNotSetErr = "Tracing: AppID not set"
)

type TraceHeader struct {
	Key, Value string
}

func GetTraceHeaders(r *http.Request, generateTrace bool) ([]TraceHeader, error) {
	// This is not concurrency safe (hat tip to Jay)
	// Hence it is instantiated in the goroutine that handles the API req
	// Google search says this is inexpensive and acceptable
	// Prefer not to use a mutex as multiple Kafka reqs can wait on this lock

	randGen := rand.NewSource(time.Now().UnixNano())

	headers := make([]TraceHeader, 0)
	traceParent := r.Header.Get("traceparent")
	traceComponents := strings.Split(traceParent, "-")
	if len(traceComponents) != 4 {
		// traceparent header is either incorrectly formatted or doesn't exist
		if !generateTrace {
			// Don't create traceparent header if flag is not set
			// Don't bother about tracestate header
			return headers, nil
		}

		if xpcTracer.appID == "" {
			return headers, fmt.Errorf(traceAppIDNotSetErr)
		}
		// Create a new traceparent header
		traceParent = fmt.Sprintf("%s-%016x%016x-%s-%s",
			traceVersion,
			// genInt63(randGen), genInt63(randGen),
			randGen.Int63(), randGen.Int63(),
			xpcTracer.appID,
			traceFlags)
	} else {
		// Replace the appID part
		if xpcTracer.appID == "" {
			return headers, fmt.Errorf(traceAppIDNotSetErr)
		}
		// Create a new traceparent header
		traceParent = fmt.Sprintf("%s-%s-%s-%s",
			traceComponents[0],
			traceComponents[1],
		 	xpcTracer.appID,
			traceComponents[3])
	}
	headers = append(headers, TraceHeader{
		Key:   "traceparent",
		Value: traceParent,
	})

	traceState := r.Header.Get("tracestate")
	if traceState == "" {
		// Create tracestate header if it doesn't exist
		traceState = fmt.Sprintf("%s=%s", xpcTracer.appName, xpcTracer.appID)
	} else {
		// Add current app to traceState
		traceState = fmt.Sprintf("%s=%s,%s", xpcTracer.appName, xpcTracer.appID, traceState)
	}
	headers = append(headers, TraceHeader{
		Key:   "tracestate",
		Value: traceState,
	})
	return headers, nil
}

func tryHomegrownTpTs(r *http.Request, xpcTrace *XpcTrace) {
	// if homegrownTracePropagation is true, we modify existing traceparent/tracestate and forward them
	// but we will not generate them if they don't exist
	// if homegrownTraceGeneration is true, and there is no tp/ts, we generate it

	if !xpcTracer.homegrownTracePropagation {
		return
	}
	// ctx := r.Context()
	log.Debug("Tracing: using homegrown traceparent propagation/generation")
	if traceHeaders, err := GetTraceHeaders(r, xpcTracer.homegrownTraceGeneration); err == nil {
		// TODO: Change homegrown propagation too to a per API basis instead of per app basis
		// Reverse engineer otel basically
		for _, h := range traceHeaders {
			log.Debugf("Tracing: %s set to %s", h.Key, h.Value)
			if h.Key == common.HeaderTraceparent {
				xpcTrace.OutTraceparent = h.Value
			}
			if h.Key == common.HeaderTracestate {
				xpcTrace.OutTracestate = h.Value
			}
		}
	} else {
		log.Errorf("Error in homegrown traceparent handling %+v", err)
	}
}

/*
	// Benchmarking expt to compare mutex based random vs one source per goroutine
	var (
		// This is for a benchmarking expt
		global bool
		globalLock sync.Mutex
		globalRandGen = rand.NewSource(time.Now().UnixNano())

		// Sample Benchmarking Results
		//	BenchmarkSharedRandGen-16         	  352419	      3052 ns/op
		//	BenchmarkIndependentRandGen-16    	  670383	      2755 ns/op
	)

	func setGlobal(g bool) {
		global = g
	}

	func genInt63(randGen rand.Source) int64 {
		if global {
			globalLock.Lock()
			i := globalRandGen.Int63()
			globalLock.Unlock()
			return i
		}
		return randGen.Int63()
	}
*/

/*
	traceparentParentID             string
	tracestateVendorID              string
	otelEnabled                     bool
		traceparentParentID:             traceparentParentID,
		tracestateVendorID:              tracestateVendorID,
func (s *WebconfigServer) TraceparentParentID() string {
	return s.traceparentParentID
}

func (s *WebconfigServer) SetTraceparentParentID(x string) {
	s.traceparentParentID = x
}

func (s *WebconfigServer) TracestateVendorID() string {
	return s.tracestateVendorID
}

func (s *WebconfigServer) SetTracestateVendorID(x string) {
	s.tracestateVendorID = x
}
*/
