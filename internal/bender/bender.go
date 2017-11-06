/*
Copyright 2014 Pinterest.com
Copyright 2016 Volker Dobler.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bender

import (
	"sync"
	"time"

	"github.com/vdobler/ht/ht"
)

type IntervalGenerator func(int64) int64

type EventType int

const (
	StartEvent EventType = iota
	EndEvent
	WaitEvent
	StartRequestEvent
	EndRequestEvent
)

type Event struct {
	Typ           EventType
	Start, End    int64
	Wait, Overage int64
	Test          *ht.Test
	Err           error
}

type Test struct {
	Test *ht.Test
	Done chan bool
}

/******

// StartEvent is sent once at the start of the load test.
// The Unix epoch time in nanoseconds at which the load test started.
Start int64

// EndEvent is sent once at the end of the load test, after which no more
// events are sent.
// The Unix epoch times in nanoseconds at which the load test
// started and ended.
Start, End int64

// WaitEvent is sent once for each request before sleeping for the given
// interval.
// The next wait time (in nanoseconds) and the accumulated overage
// time (the difference between
// the actual wait time and the intended wait time).
Wait, Overage int64

// StartRequestEvent is sent before a request is executed. The sending of this
// event happens before the timing of the request starts, to avoid potential
// issues, so it contains the timestamp of the event send, and not the
// timestamp of the request start.
// The Unix epoch time (in nanoseconds) at which this event was
// created, which will be earlier
// than the sending of the associated request (for performance reasons)
Time int64
// The request that will be sent, nothing good can come from modifying it
Request *ht.Test

// EndRequestEvent is sent after a request has completed.
// The Unix epoch times (in nanoseconds) at which the request was
// started and finished
Start, End int64
// The response data returned by the request executor
Response *ht.Test

************/

// LoadTestThroughput starts a load test in which the caller controls the
// interval between requests being sent. See the package documentation for
// details on the arguments to this function.
func LoadTestThroughput(intervals IntervalGenerator, requests chan Test, recorder chan Event) {
	go func() {
		start := time.Now().UnixNano()
		recorder <- Event{Typ: StartEvent, Start: start}

		var wg sync.WaitGroup
		var overage int64
		t0 := time.Now().UnixNano()
		for request := range requests {
			now := time.Now().UnixNano()
			overage += now - t0
			wait := intervals(now) - overage
			// fmt.Println("WaitIntervall", (wait+overage)/1000, "  Overage", overage/1000, "[mus]")
			if wait >= 0 {
				time.Sleep(time.Duration(wait))
				overage = 0
			} else {
				overage = -wait
			}
			t0 = time.Now().UnixNano()

			wg.Add(1)
			go func(test Test, overage int64) {
				defer wg.Done()
				reqStart := time.Now().UnixNano()
				test.Test.Run()
				test.Done <- true
				recorder <- Event{
					Typ:     EndRequestEvent,
					Start:   reqStart,
					End:     time.Now().UnixNano(),
					Wait:    wait,
					Overage: overage,
					Test:    test.Test,
				}
			}(request, overage)

		}
		wg.Wait()
		recorder <- Event{
			Typ:   EndEvent,
			Start: start,
			End:   time.Now().UnixNano(),
		}
		close(recorder)
	}()
}
