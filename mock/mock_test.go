package mock

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/vdobler/ht/scope"
)

type mapTest struct {
	vars    scope.Variables
	mapping Mapping
	want    string
	value   string
}

var mapTests = []mapTest{
	{
		scope.Variables{"X": "foo"},
		Mapping{Variables: []string{"X", "A"},
			Table: []string{"foo", "bar"}},
		"A", "bar",
	},
	{
		scope.Variables{"X": "foo"},
		Mapping{Variables: []string{"X", "A"},
			Table: []string{
				"foo", "bar",
				"foo", "waz",
			}},
		"A", "bar",
	},
	{
		scope.Variables{"X": "foo", "Y": "bar"},
		Mapping{Variables: []string{"X", "Y", "A"},
			Table: []string{
				"foo", "bar", "123",
				"foo", "wuz", "999",
				"foo", "*", "234",
				"*", "bar", "345",
				"*", "*", "456",
			}},
		"A", "123",
	},
	{
		scope.Variables{"X": "rr", "Y": "tt"},
		Mapping{Variables: []string{"X", "Y", "A"},
			Table: []string{
				"foo", "bar", "123",
				"foo", "wuz", "999",
				"foo", "*", "234",
				"*", "bar", "345",
				"*", "*", "456",
			}},
		"A", "456",
	},
	{
		scope.Variables{"X": "foo", "Y": "tt"},
		Mapping{Variables: []string{"X", "Y", "A"},
			Table: []string{
				"foo", "bar", "123",
				"foo", "wuz", "999",
				"foo", "*", "234",
				"*", "bar", "345",
				"*", "*", "456",
			}},
		"A", "234",
	},
	{
		scope.Variables{"X": "rr", "Y": "bar"},
		Mapping{Variables: []string{"X", "Y", "A"},
			Table: []string{
				"foo", "bar", "123",
				"foo", "wuz", "999",
				"foo", "*", "234",
				"*", "*", "456",
				"*", "bar", "345",
			}},
		"A", "345",
	},

	// The un-happy paths.
	{
		scope.Variables{"X": "foo"},
		Mapping{Variables: []string{"X", "A"},
			Table: []string{"wuz", "bar"}},
		"A", "-undefined-",
	},
	{
		scope.Variables{"X": "foo"},
		Mapping{Variables: []string{"Z", "A"},
			Table: []string{"foo", "bar"}},
		"A", "-undefined-Z-",
	},
	{
		scope.Variables{"X": "foo"},
		Mapping{Variables: []string{"X", "A"}},
		"", "-malformed-table-",
	},
	{
		scope.Variables{"X": "foo"},
		Mapping{Variables: []string{"X", "A"},
			Table: []string{"wuz", "bar", "kiz"}},
		"", "-malformed-table-",
	},
	{
		scope.Variables{"X": "foo"},
		Mapping{Variables: []string{"X"},
			Table: []string{"foo", "bar"}},
		"", "-malformed-variables-",
	},
}

func TestMapping(t *testing.T) {
	for i, tc := range mapTests {
		name, value := tc.mapping.lookup(tc.vars)
		if name != tc.want {
			t.Errorf("%d. Bad name, got %q, want %q",
				i, name, tc.want)
		}
		if value != tc.value {
			t.Errorf("%d. Bad value, got %q, want %q",
				i, value, tc.value)
		}
	}
}

// client is a fast-failing client which does not verify TLS certificates.
var client = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   1 * time.Second,
			KeepAlive: 2 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:    10,
		IdleConnTimeout: 4 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		TLSHandshakeTimeout:   1 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	Timeout: 500 * time.Millisecond,
}

// get the URL u and return status code and body. Uses client to skip
// TLS certificate verification.
func get(u string) (int, string, error) {
	resp, err := client.Get(u)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	code := resp.StatusCode
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return code, "", err
	}
	return code, string(body), nil
}

func TestServe(t *testing.T) {
	var logger Log
	if testing.Verbose() {
		logger = log.New(os.Stdout, "", 0)
	}
	mocks := []*Mock{
		{
			Name:   "Mock A",
			Method: "GET",
			URL:    "http://localhost:8080/ma/{NAME}",
			Response: Response{
				// StatusCode defaults to 200
				Body: "Hello {{NAME}}",
			},
		},
		{
			Name:   "Mock B",
			Method: "GET",
			URL:    "https://localhost:8443/mb/{NAME}",
			Response: Response{
				StatusCode: 202,
				Body:       "Hola {{NAME}}",
			},
		},
	}

	stop, err := Serve(mocks, nil, logger, "./testdata/dummy.cert", "./testdata/dummy.key")
	if err != nil {
		t.Fatal(err)
	}

	status, body, err := get("http://localhost:8080/ma/Foo")
	if status != 200 || body != "Hello Foo" || err != nil {
		t.Errorf("Mock A: got %d %q %v", status, body, err)
	}

	status, body, err = get("https://localhost:8443/mb/Bar")
	if status != 202 || body != "Hola Bar" || err != nil {
		t.Errorf("Mock B: got %d %q %v", status, body, err)
	}

	status, body, err = get("http://localhost:8080/xyz")
	if status != 404 || body != "404 page not found\n" || err != nil {
		t.Errorf("404: got %d %q %v", status, body, err)
	}

	stop <- true
	<-stop
}
