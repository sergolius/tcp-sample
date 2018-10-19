package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	tcp "github.com/sergolius/tcp-sample"
)

const DefaultAddr = ":3000"

func main() {
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = DefaultAddr
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	l := tcp.Server{IdleTimeout: time.Minute * 5}
	go l.ListenAndServe(addr)

	<-signals

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

	l.Shutdown(ctx)
	cancel()
}
