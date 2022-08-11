package cmd

import (
	"os"
	"os/signal"

	"github.com/ViBiOh/kmux/pkg/output"
)

func waitForEnd(signals ...os.Signal) {
	signalsChan := make(chan os.Signal, len(signals))
	defer close(signalsChan)

	signal.Notify(signalsChan, signals...)
	defer signal.Stop(signalsChan)

	sig := <-signalsChan
	output.Warn("", "Signal %s received!", sig)
}
