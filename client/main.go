package main

import (
	tcp "github.com/sergolius/tcp-sample"
	"os"
)

const DefaultAddr = ":3000"

func main() {
	addr := os.Getenv("CLIENT_ADDR")
	if addr == "" {
		addr = DefaultAddr
	}

	client := tcp.Client{}
	client.Dial(addr)
}
