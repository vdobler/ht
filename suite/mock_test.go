package suite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/vdobler/ht/ht"
)

func pipeHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/users/") {
		http.Error(w, "No good", http.StatusBadRequest)
		return
	}
	userid := path[len("/users/"):]

	// In our nanoservice architecture we'll look up surename
	// and lastname from different services!

	// Surename: retrieved in a JSON from :9901/surenameservice/{userid}
	snReq, err := http.NewRequest("GET", "http://localhost:9901/surenameservice/"+userid, nil)
	if err != nil {
		http.Error(w, "Cannot create request: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	snReq.Header.Set("X-Authorized", "okay")
	snResp, err := http.DefaultClient.Do(snReq)
	if err != nil {
		http.Error(w, "Cannot reach surenameservice: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	snJson, err := ioutil.ReadAll(snResp.Body)
	// fmt.Println("Surname-Service Body: ", string(snJson))
	snResp.Body.Close()
	var x struct {
		Status   string `json:"status"`
		Surename string `json:"surename"`
		UserID   string `json:"userid"`
	}
	err = json.Unmarshal(snJson, &x)
	if err != nil {
		http.Error(w, "Cannot decode surename JSON: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	if x.Status != "okay" || x.UserID != userid {
		http.Error(w, "Hoppala", http.StatusInternalServerError)
		return
	}
	surename := x.Surename
	// fmt.Println("Got surename: ", surename)

	// Lastname
	// TODO: POST to "http://localhost:9902/rest/v7/lookup"
	lnReq, err := http.NewRequest("POST", "http://localhost:9902/rest/v7/lookup",
		strings.NewReader("userid="+userid))
	if err != nil {
		http.Error(w, "Cannot create request: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	lnReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	lnResp, err := http.DefaultClient.Do(lnReq)
	if err != nil {
		http.Error(w, "Cannot reach lastname service: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	lnBody, err := ioutil.ReadAll(lnResp.Body)
	if err != nil {
		http.Error(w, "Cannot read lastname body: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	n := bytes.Index(lnBody, []byte(","))
	if n < 1 {
		http.Error(w, "Cannot malformed body: "+string(lnBody),
			http.StatusInternalServerError)
		return
	}
	lastname := string(lnBody[n+1:])
	// fmt.Println("Got lastname: ", lastname)

	var answer struct {
		Name   string `json:"name"`
		UserID string `json:"userid"`
	}
	answer.Name = fmt.Sprintf("%s %s", surename, lastname)
	answer.UserID = userid
	body, err := json.Marshal(answer)
	if err != nil {
		http.Error(w, "Cannot marshal JSON "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	// fmt.Println("Final Answer: ", string(body))
	w.Write(body)
}

var mockSuiteResults = []struct {
	status ht.Status
	err    string
}{
	{ht.Fail, "got 500, want 200\n"},
	{ht.Fail, "Main test passed, but mock invocations failed: Unequal, was \"okay\"\n"},
	{ht.Pass, "<nil>\n"},
	{ht.Fail, "Main test passed, but mock invocations failed: mock \"Mock 2: Some other Mock\" was not called\n"},
	{ht.Pass, "<nil>\n"},
}

func TestMockSuite(t *testing.T) {
	raw, err := LoadRawSuite("testdata/mock.suite", nil)
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(pipeHandler))

	s := raw.Execute(map[string]string{"BASEURL": ts.URL}, nil, logger())

	if testing.Verbose() {
		os.MkdirAll("./testdata/mockreport", 0766)
		err = HTMLReport("./testdata/mockreport", s)
		if err != nil {
			t.Fatalf("Unexpected error %s", err)
		}
	}

	if got := s.Status; got != ht.Fail {
		t.Errorf("Got suite status of %s, want Fail; err=%v", got, s.Error)
	}

	for i, test := range s.Tests {
		if test.Status != mockSuiteResults[i].status {
			t.Errorf("Test %d: Got status %s, want %s",
				i, test.Status, mockSuiteResults[i].status)
		}
		if e := fmt.Sprintln(test.Error); e != mockSuiteResults[i].err {
			t.Errorf("Test %d: Got error %q, want %q",
				i, e, mockSuiteResults[i].err)
		}
	}
}
