package main

import (
	"flag"
	"github.com/299m/udp-dist/engine"
	"github.com/299m/util/util"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	cfgdir := ""
	bufsize := 0
	queuesize := 0
	flag.StringVar(&cfgdir, "config", "", "config directory")
	flag.IntVar(&bufsize, "bufsize", 2049, "size of the buffer")
	flag.IntVar(&queuesize, "queuesize", 1000, "size of the queue")
	flag.Parse()

	// Load the config
	engcfg := engine.Config{}
	cfgmap := map[string]util.Expandable{
		"udp-dist": &engcfg,
	}
	util.ReadConfig(cfgdir, cfgmap)

	eng := engine.NewEngine(bufsize, queuesize, &engcfg)
	defer eng.Close()
	//// Just wait for the signal to quit

	// Create a channel to receive OS signals
	sigs := make(chan os.Signal, 1)

	// Register the channel to receive SIGINT signals
	signal.Notify(sigs, syscall.SIGINT)

	// Wait for a SIGINT signal
	<-sigs
}
