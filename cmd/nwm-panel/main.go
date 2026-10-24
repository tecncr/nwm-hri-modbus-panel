// Command nwm-panel is a Modbus RTU master, recorder and dashboard for
// NWM-HRI pulse encoders.
package main

import (
	"os"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
