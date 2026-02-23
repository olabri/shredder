package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

const defaultSocketPath = "/tmp/guitar_ear.sock"

type PitchMessage struct {
	Freq float64 `json:"freq"`
	Conf float64 `json:"conf"`
	TsMs int64   `json:"ts_ms"`
}

func main() {
	socketPath := flag.String("socket", defaultSocketPath, "unix socket path")
	duration := flag.Duration("duration", 0, "exit after duration (0 = run forever)")
	flag.Parse()

	_ = os.Remove(*socketPath)
	if err := os.MkdirAll(filepath.Dir(*socketPath), 0o755); err != nil {
		log.Fatalf("mkdir socket dir: %v", err)
	}

	listener, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	log.Printf("Headless server listening on %s", *socketPath)

	var count uint64
	var lastFreq atomic.Value
	lastFreq.Store(PitchMessage{})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("accept: %v", err)
				return
			}
			go handleConn(conn, &count, &lastFreq)
		}
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ticker.C:
			msg := lastFreq.Load().(PitchMessage)
			fmt.Printf("messages=%d last_freq=%.2f conf=%.2f\n", atomic.LoadUint64(&count), msg.Freq, msg.Conf)
		case <-time.After(50 * time.Millisecond):
		}

		if *duration > 0 && time.Since(start) >= *duration {
			fmt.Printf("done after %s\n", duration.String())
			return
		}
	}
}

func handleConn(conn net.Conn, count *uint64, lastFreq *atomic.Value) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg PitchMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		atomic.AddUint64(count, 1)
		lastFreq.Store(msg)
	}
}
