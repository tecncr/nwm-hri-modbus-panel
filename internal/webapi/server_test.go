package webapi

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"nwm-hri-modbus-panel/internal/modbus"
)

type testLink struct {
	requests [][]byte
}

func (l *testLink) Exchange(_ context.Context, request []byte, expected int) ([]byte, error) {
	l.requests = append(l.requests, append([]byte(nil), request...))
	if expected == 0 {
		return nil, nil
	}
	switch request[1] {
	case modbus.FunctionReadHoldingRegisters:
		count := binary.BigEndian.Uint16(request[4:6])
		response := []byte{request[0], request[1], byte(count * 2)}
		for i := uint16(0); i < count; i++ {
			response = append(response, 0x12, byte(0x34+i))
		}
		return modbus.AppendCRC(response), nil
	case modbus.FunctionWriteSingleRegister:
		return modbus.AppendCRC(request[:6]), nil
	case modbus.FunctionWriteMultiple:
		return modbus.AppendCRC(request[:6]), nil
	default:
		return nil, fs.ErrInvalid
	}
}

func (*testLink) Transactions() []modbus.Transaction { return nil }

func newTestServer(t *testing.T) *Server {
	t.Helper()
	assets := fstest.MapFS{
		"web/dist/index.html":     {Data: []byte("<main>panel shell</main>")},
		"web/dist/assets/app.js":  {Data: []byte("console.log('panel')")},
		"web/dist/assets/app.css": {Data: []byte("body{color:#123}")},
	}
	server, err := New(Settings{
		HTTPAddr:      "127.0.0.1:8080",
		SerialPort:    "/dev/null",
		SerialBaud:    9600,
		SerialTimeout: time.Second,
		DefaultSlave:  23,
		BuildVersion:  "v1.2.3",
	}, &testLink{}, assets)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func TestStatusAndRegisterMapEndpoints(t *testing.T) {
	handler := newTestServer(t).Handler()
	status := httptest.NewRecorder()
	handler.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if status.Code != http.StatusOK {
		t.Fatalf("status code = %d", status.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(status.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["baudRate"] != float64(9600) || payload["defaultSlave"] != float64(23) || payload["version"] != "v1.2.3" {
		t.Fatalf("status = %#v", payload)
	}
	registers := httptest.NewRecorder()
	handler.ServeHTTP(registers, httptest.NewRequest(http.MethodGet, "/api/registers", nil))
	var mapped []modbus.Register
	if err := json.Unmarshal(registers.Body.Bytes(), &mapped); err != nil {
		t.Fatal(err)
	}
	if registers.Code != http.StatusOK || len(mapped) != len(modbus.RegisterMap) || mapped[len(mapped)-1].Address != 0x1b {
		t.Fatalf("register response code=%d map size=%d", registers.Code, len(mapped))
	}
}

func TestReadAndWriteEndpointsUseAllSupportedFunctions(t *testing.T) {
	link := &testLink{}
	assets := fstest.MapFS{"web/dist/index.html": {Data: []byte("ok")}}
	server, err := New(Settings{SerialTimeout: time.Second}, link, assets)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Handler()

	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodPost, "/api/read", strings.NewReader(`{"slave":23,"address":1,"count":2}`)))
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"values":[4660,4661]`) {
		t.Fatalf("read response: code=%d body=%s", read.Code, read.Body.String())
	}

	single := httptest.NewRecorder()
	handler.ServeHTTP(single, httptest.NewRequest(http.MethodPost, "/api/write-single", strings.NewReader(`{"slave":23,"address":13,"value":5}`)))
	if single.Code != http.StatusOK {
		t.Fatalf("single write response: %d %s", single.Code, single.Body.String())
	}

	multiple := httptest.NewRecorder()
	handler.ServeHTTP(multiple, httptest.NewRequest(http.MethodPost, "/api/write-multiple", strings.NewReader(`{"slave":23,"address":18,"values":[1,2,3,4]}`)))
	if multiple.Code != http.StatusOK || len(link.requests) != 3 {
		t.Fatalf("multiple write response: %d %s; frames=%d", multiple.Code, multiple.Body.String(), len(link.requests))
	}
	if link.requests[0][1] != 0x03 || link.requests[1][1] != 0x06 || link.requests[2][1] != 0x10 {
		t.Fatalf("function codes = %x, %x, %x", link.requests[0][1], link.requests[1][1], link.requests[2][1])
	}
}

func TestInvalidReadAndMalformedJSONAreClientErrors(t *testing.T) {
	handler := newTestServer(t).Handler()
	for _, body := range []string{`{"slave":0,"address":0,"count":1}`, `{"slave":4,"address":0,"count":126}`, `{"slave":4,"count":1,"extra":true}`} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/read", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Errorf("body %s: status=%d response=%s", body, response.Code, response.Body.String())
		}
	}
}

func TestBroadcastWriteReturnsWithoutWaitingForReply(t *testing.T) {
	link := &testLink{}
	assets := fstest.MapFS{"web/dist/index.html": {Data: []byte("ok")}}
	server, err := New(Settings{SerialTimeout: time.Second}, link, assets)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/write-single", strings.NewReader(`{"slave":0,"address":0,"value":24}`)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"broadcast":true`) {
		t.Fatalf("broadcast response: code=%d body=%s", response.Code, response.Body.String())
	}
	if got := link.requests[0][0]; got != 0 {
		t.Fatalf("frame address = %d, want broadcast address 0", got)
	}
}

func TestScanProbesRequestedSlaveRange(t *testing.T) {
	assets := fstest.MapFS{"web/dist/index.html": {Data: []byte("ok")}}
	server, err := New(Settings{SerialTimeout: time.Second}, &testLink{}, assets)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/scan", strings.NewReader(`{"start":14,"end":16}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("scan status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Scanned int          `json:"scanned"`
		Devices []scanDevice `json:"devices"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 3 || len(result.Devices) != 3 || result.Devices[1].Slave != 15 {
		t.Fatalf("scan result = %#v", result)
	}
}

func TestEmbeddedFrontendServesAssetsAndSPAPaths(t *testing.T) {
	handler := newTestServer(t).Handler()
	for path, want := range map[string]string{"/": "panel shell", "/assets/app.js": "console.log", "/configuration": "panel shell"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), want) {
			t.Errorf("%s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}
