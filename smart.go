package main

import (
	"flag"
	"log"
)

func main() {
	var device string
	flag.StringVar(&device, "dev", "/dev/sg0", "scsi device name")
	flag.BoolVar(&Debug, "debug", false, "enable debug output")
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	err := TestUnitReady(device)
	if err != nil {
		log.Println("TUR ERR:", err)
	}

	err = Inquire(device)
	if err != nil {
		log.Println("INQUIRY ERR:", err)
	}
}
