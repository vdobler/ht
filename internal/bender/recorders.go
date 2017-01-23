/*
Copyright 2014 Pinterest.com
Copyright 2016 Volker Dobler

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

import "log"

type Recorder func(Event)

func Record(c chan Event, done chan bool, recorders ...Recorder) {
	for e := range c {
		for _, recorder := range recorders {
			recorder(e)
		}
	}
	close(done)
}

func logMessage(l *log.Logger, e Event) {
	switch e.Typ {
	case StartEvent:
		log.Print("Begin of loadtest at ", e.Start)
	case EndEvent:
		log.Print("Finish of loadtest at ", e.End, " took ", (e.End-e.Start)/1e9, " seconds")
	case WaitEvent:
		log.Print("Waiting ", e.Start/1e6, " ms, Overtime ", e.End/1e6, " ms")
	case StartRequestEvent:
		log.Print("Running ", e.Test.Name)
	case EndRequestEvent:
		log.Printf("Done %s %s %s", e.Test.Response.Duration, e.Test.Status, e.Test.Name)
	}
}

func NewLoggingRecorder(l *log.Logger) Recorder {
	return func(e Event) {
		logMessage(l, e)
	}
}
