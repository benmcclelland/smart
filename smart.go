package main

import "flag"

func main() {
	var device string
	flag.StringVar(&device, "dev", "/dev/sg0", "scsi device name")
	flag.Parse()

	err := Inquire(device)
	if err != nil {
		panic(err)
	}
}
