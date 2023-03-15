package cmd

import (
	"os"
	"os/signal"
)

func waitForEnd(signals ...os.Signal) {
	signalsChan := make(chan os.Signal, len(signals))
	defer close(signalsChan)

	signal.Notify(signalsChan, signals...)
	defer signal.Stop(signalsChan)

	<-signalsChan
}
