package webapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"nwm-hri-modbus-panel/internal/modbus"
)

type Settings struct {
	HTTPAddr      string
	SerialPort    string
	SerialBaud    int
	SerialTimeout time.Duration
	DefaultSlave  int
	BuildVersion  string
}

type Server struct {
	client   *modbus.Client
	link     Link
	settings Settings
	frontend http.Handler
	assets   fs.FS
}

type Link interface {
	modbus.ExchangeLink
	Transactions() []modbus.Transaction
}

func New(settings Settings, link Link, frontend fs.FS) (*Server, error) {
	assets, err := fs.Sub(frontend, "web/dist")
	if err != nil {
		return nil, err
	}
	return &Server{
		client: modbus.NewClient(link), link: link, settings: settings,
		frontend: http.FileServer(http.FS(assets)), assets: assets,
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/registers", s.registers)
	mux.HandleFunc("GET /api/transactions", s.transactions)
	mux.HandleFunc("POST /api/scan", s.scan)
	mux.HandleFunc("POST /api/read", s.read)
	mux.HandleFunc("POST /api/write-single", s.writeSingle)
	mux.HandleFunc("POST /api/write-multiple", s.writeMultiple)
	mux.HandleFunc("GET /", s.serveFrontend)
	return mux
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"httpAddr": s.settings.HTTPAddr, "serialPort": s.settings.SerialPort,
		"baudRate": s.settings.SerialBaud, "dataBits": 8, "parity": "none", "stopBits": 1,
		"timeoutMs": s.settings.SerialTimeout.Milliseconds(), "protocol": "Modbus RTU", "defaultSlave": s.settings.DefaultSlave,
		"version": s.settings.BuildVersion,
		"functions": []string{"0x03 Read Holding Registers", "0x06 Write Single Register", "0x10 Write Multiple Registers"},
	})
}

func (s *Server) registers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, modbus.RegisterMap)
}

func (s *Server) transactions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.link.Transactions())
}

type scanRequest struct {
	Start uint16 `json:"start"`
	End   uint16 `json:"end"`
}

type scanDevice struct {
	Slave        uint16 `json:"slave"`
	AddressValue uint16 `json:"addressValue"`
}

func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	var request scanRequest
	if !decode(w, r, &request) {
		return
	}
	if request.Start == 0 && request.End == 0 {
		request.Start, request.End = 1, 247
	} else {
		if request.Start == 0 {
			request.Start = 1
		}
		if request.End == 0 {
			request.End = 247
		}
	}
	if request.Start < 1 || request.End > 247 || request.Start > request.End {
		writeError(w, http.StatusBadRequest, "scan range must be within slave addresses 1..247")
		return
	}

	devices := make([]scanDevice, 0)
	scanned := 0
	probeTimeout := 100 * time.Millisecond
	if s.settings.SerialTimeout > 0 && s.settings.SerialTimeout < probeTimeout {
		probeTimeout = s.settings.SerialTimeout
	}
	for slave := request.Start; slave <= request.End; slave++ {
		scanned++
		probeCtx, cancel := context.WithTimeout(r.Context(), probeTimeout)
		values, err := s.client.ReadRegisters(probeCtx, byte(slave), 0, 1)
		cancel()
		if err == nil {
			devices = append(devices, scanDevice{Slave: slave, AddressValue: values[0]})
		}
		if r.Context().Err() != nil {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"devices": devices, "scanned": scanned,
		"probeTimeoutMs": probeTimeout.Milliseconds(),
	})
}

type readRequest struct {
	Slave   uint16 `json:"slave"`
	Address uint16 `json:"address"`
	Count   uint16 `json:"count"`
}

func (s *Server) read(w http.ResponseWriter, r *http.Request) {
	var request readRequest
	if !decode(w, r, &request) {
		return
	}
	if request.Slave < 1 || request.Slave > 247 {
		writeError(w, http.StatusBadRequest, "slave must be between 1 and 247 for reads")
		return
	}
	if request.Count < 1 || request.Count > modbus.MaxRegisters || uint32(request.Address)+uint32(request.Count) > 0x10000 {
		writeError(w, http.StatusBadRequest, "read count must be 1..125 and stay within the register address range")
		return
	}
	values, err := s.client.ReadRegisters(r.Context(), byte(request.Slave), request.Address, request.Count)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusRequestTimeout
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"slave": request.Slave, "address": request.Address, "values": values})
}

type singleWriteRequest struct {
	Slave   uint16 `json:"slave"`
	Address uint16 `json:"address"`
	Value   uint16 `json:"value"`
}

func (s *Server) writeSingle(w http.ResponseWriter, r *http.Request) {
	var request singleWriteRequest
	if !decode(w, r, &request) {
		return
	}
	if request.Slave > 247 {
		writeError(w, http.StatusBadRequest, "slave must be between 0 and 247")
		return
	}
	if err := s.client.WriteSingle(r.Context(), byte(request.Slave), request.Address, request.Value); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "broadcast": request.Slave == 0})
}

type multipleWriteRequest struct {
	Slave   uint16   `json:"slave"`
	Address uint16   `json:"address"`
	Values  []uint16 `json:"values"`
}

func (s *Server) writeMultiple(w http.ResponseWriter, r *http.Request) {
	var request multipleWriteRequest
	if !decode(w, r, &request) {
		return
	}
	if request.Slave > 247 {
		writeError(w, http.StatusBadRequest, "slave must be between 0 and 247")
		return
	}
	if len(request.Values) < 1 || len(request.Values) > modbus.MaxWriteRegisters || uint32(request.Address)+uint32(len(request.Values)) > 0x10000 {
		writeError(w, http.StatusBadRequest, "write count must be 1..123 and stay within the register address range")
		return
	}
	if err := s.client.WriteMultiple(r.Context(), byte(request.Slave), request.Address, request.Values); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "broadcast": request.Slave == 0})
}

func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	// The application is a single-page UI; unknown paths use the embedded index.
	if r.URL.Path != "/" {
		if _, err := fs.Stat(s.assets, strings.TrimPrefix(r.URL.Path, "/")); err == nil {
			s.frontend.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
	}
	s.frontend.ServeHTTP(w, r)
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode JSON response: %v", err)
	}
}
