package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/g4i/local-clipboard/internal/client"
	"github.com/g4i/local-clipboard/internal/hub"
)

func main() {
	log.SetFlags(log.Ltime)
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		addr := fs.String("addr", ":9100", "listen address")
		token := fs.String("token", "", "shared token (empty = open)")
		fs.Parse(os.Args[2:])
		serve(*addr, *token)
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		server := fs.String("server", "", "hub address, e.g. nas:9100")
		name := fs.String("name", defaultName(), "device name")
		token := fs.String("token", "", "shared token")
		fs.Parse(os.Args[2:])
		if *server == "" {
			fmt.Fprintln(os.Stderr, "-server is required")
			os.Exit(1)
		}
		client.New(*server, *name, *token).Run()
	case "send":
		fs := flag.NewFlagSet("send", flag.ExitOnError)
		server := fs.String("server", "", "hub address")
		name := fs.String("name", defaultName(), "sender name")
		token := fs.String("token", "", "shared token")
		stdin := fs.Bool("stdin", false, "read from stdin")
		fs.Parse(os.Args[2:])
		if *server == "" {
			fmt.Fprintln(os.Stderr, "-server is required")
			os.Exit(1)
		}
		send(*server, *name, *token, *stdin, fs.Args())
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  local-clipboard serve [-addr :9100] [-token secret]   run hub (on NAS)
  local-clipboard run -server nas:9100 [-token secret]  run sync daemon
  local-clipboard send -server nas:9100 text            push a message`)
}

func defaultName() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func serve(addr, token string) {
	if token == "" {
		token = os.Getenv("LOCALCLIP_TOKEN")
	}
	mux := http.NewServeMux()
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	hub.New(token).Routes(mux)
	log.Printf("[hub] listening on %s (auth: %v)", addr, token != "")
	log.Fatal(srv.ListenAndServe())
}

func send(server, name, token string, useStdin bool, args []string) {
	var text string
	switch {
	case useStdin:
		b, _ := io.ReadAll(os.Stdin)
		text = string(b)
	case len(args) > 0:
		text = args[0]
	default:
		fmt.Fprintln(os.Stderr, "nothing to send")
		os.Exit(1)
	}
	if err := client.SendOnce(server, name, token, text); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sent %d bytes via %s\n", len(text), server)
}
