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
	"github.com/g4i/local-clipboard/internal/proto"
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
		token := fs.String("token", "", "shared token (or LOCALCLIP_TOKEN env)")
		fs.Parse(os.Args[2:])
		serve(*addr, *token)

	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		server := fs.String("server", "", "hub address, e.g. nas:9100")
		name := fs.String("name", defaultName(), "device name")
		token := fs.String("token", "", "shared token")
		fs.Parse(os.Args[2:])
		mustServer(server)
		client.New(*server, *name, *token).Run()

	case "send-clipboard":
		fs := flag.NewFlagSet("send-clipboard", flag.ExitOnError)
		server := fs.String("server", "", "")
		name := fs.String("name", defaultName(), "")
		token := fs.String("token", "", "")
		to := fs.String("to", "", "target device name (empty = all online)")
		fs.Parse(os.Args[2:])
		mustServer(server)
		msg, _, isFileHint, err := client.DetectAndBuildClipboardMessage()
		die(err)
		if isFileHint {
			fmt.Fprintln(os.Stderr, "clipboard holds large text; save it to a file and use send-file")
			os.Exit(1)
		}
		msg.Target = *to
		die(client.SendPayload(*server, *name, *token, msg))
		fmt.Printf("sent %s (%s) to %s\n", msg.Type, humanBytes(msg.Size), targetDesc(*to))

	case "send-text":
		fs := flag.NewFlagSet("send-text", flag.ExitOnError)
		server := fs.String("server", "", "")
		name := fs.String("name", defaultName(), "")
		token := fs.String("token", "", "")
		to := fs.String("to", "", "")
		stdin := fs.Bool("stdin", false, "read from stdin")
		fs.Parse(os.Args[2:])
		mustServer(server)
		var text string
		if *stdin {
			b, _ := io.ReadAll(os.Stdin)
			text = string(b)
		} else if fs.Args() != nil && len(fs.Args()) > 0 {
			text = fs.Args()[0]
		}
		if text == "" {
			die(fmt.Errorf("nothing to send"))
		}
		msg := proto.NewMessage("", proto.TypeText)
		msg.Content = text
		msg.Size = int64(len(text))
		msg.Target = *to
		die(client.SendPayload(*server, *name, *token, msg))
		fmt.Printf("sent text (%s) to %s\n", humanBytes(msg.Size), targetDesc(*to))

	case "send-file":
		fs := flag.NewFlagSet("send-file", flag.ExitOnError)
		server := fs.String("server", "", "")
		name := fs.String("name", defaultName(), "")
		token := fs.String("token", "", "")
		to := fs.String("to", "", "")
		fs.Parse(os.Args[2:])
		mustServer(server)
		args := fs.Args()
		if len(args) == 0 {
			die(fmt.Errorf("usage: send-file [-to device] <file...>"))
		}
		for _, p := range args {
			fid, size, err := client.UploadFile(*server, *name, *token, p)
			die(err)
			msg := proto.NewMessage("", proto.TypeFile)
			msg.Name = base(p)
			msg.Size = size
			msg.FID = fid
			msg.Target = *to
			die(client.SendPayload(*server, *name, *token, msg))
			fmt.Printf("sent file %s (%s) to %s\n", msg.Name, humanBytes(size), targetDesc(*to))
		}

	case "accept":
		fs := flag.NewFlagSet("accept", flag.ExitOnError)
		server := fs.String("server", "", "")
		token := fs.String("token", "", "")
		out := fs.String("out", ".", "output directory")
		fs.Parse(os.Args[2:])
		mustServer(server)
		if len(fs.Args()) != 1 {
			die(fmt.Errorf("usage: accept -server nas:9100 <file-id>"))
		}
		path, err := client.DownloadFile(*server, *token, fs.Args()[0], *out)
		die(err)
		fmt.Printf("saved to %s\n", path)

	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `local-clipboard — LAN clipboard & file transfer

  serve -addr :9100 [-token secret]              run hub (on NAS)
  run -server nas:9100 [-token s] [-name dev]    daemon: receive -> clipboard / notify
  send-clipboard -server nas:9100                push local clipboard (text/screenshot) [-to dev]
  send-text -server nas:9100 "hello"             push raw text
  send-file -server nas:9100 f.pdf               upload + announce file [-to dev]
  accept -server nas:9100 <file-id>              download an offered file`)
}

func mustServer(server *string) {
	if *server == "" {
		fmt.Fprintln(os.Stderr, "-server is required")
		os.Exit(1)
	}
}

func targetDesc(to string) string {
	if to == "" {
		return "all online devices"
	}
	return to
}

func base(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}

func humanBytes(n int64) string {
	const kb, mb = 1 << 10, 1 << 20
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1fMB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.0fKB", float64(n)/kb)
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
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
	h, err := hub.New(token)
	die(err)
	mux := http.NewServeMux()
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	h.Routes(mux)
	log.Printf("[hub] listening on %s (auth: %v)", addr, token != "")
	log.Fatal(srv.ListenAndServe())
}
