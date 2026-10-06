package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"nwm-hri-modbus-panel/internal/modbus"
	"nwm-hri-modbus-panel/internal/webapi"
)

//go:embed web/dist
var frontend embed.FS

var buildVersion string

func main() {
	settings := webapi.Settings{
		HTTPAddr:      envString("HTTP_ADDR", "127.0.0.1:8080"),
		SerialPort:    envString("SERIAL_PORT", "/dev/ttyUSB0"),
		SerialBaud:    envInt("SERIAL_BAUD", 9600),
		SerialTimeout: time.Duration(envInt("SERIAL_TIMEOUT_MS", 1000)) * time.Millisecond,
		DefaultSlave:  envInt("DEFAULT_SLAVE_ID", 15),
		BuildVersion:  strings.TrimSpace(buildVersion),
	}
	if settings.SerialBaud < 300 || settings.SerialTimeout <= 0 || settings.DefaultSlave < 1 || settings.DefaultSlave > 247 {
		log.Fatal("SERIAL_BAUD must be at least 300, SERIAL_TIMEOUT_MS positive, and DEFAULT_SLAVE_ID 1..247")
	}
	link := &modbus.SerialLink{PortName: settings.SerialPort, BaudRate: settings.SerialBaud, Timeout: settings.SerialTimeout}
	app, err := webapi.New(settings, link, frontend)
	if err != nil {
		log.Fatalf("load embedded frontend: %v", err)
	}

	server := &http.Server{
		Addr: settings.HTTPAddr, Handler: app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("NWM-HRI Modbus panel listening on http://%s (serial %s, %d baud 8N1)", settings.HTTPAddr, settings.SerialPort, settings.SerialBaud)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server: %v", err)
	}
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("invalid %s=%q; using %d", name, value, fallback)
		return fallback
	}
	return parsed
}
