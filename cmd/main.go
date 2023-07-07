package main

import (
	"fmt"
)

var (
	// This is set at compile time
	version = "unknown"
)

const driverName = "daos.csi.storage.gke.io"

func main() {
	fmt.Printf("hello world! version %s", version)
	select {}
}
