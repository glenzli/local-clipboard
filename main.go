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
		config := fs.String("config", "", "saved Windows client configuration")
		server := fs.String("server", "", "hub address, e.g. nas:9100")
		name := fs.String("name", defaultName(), "device name")
		token := fs.String("token", os.Getenv("LOCALCLIP_TOKEN"), "shared token (or LOCALCLIP_TOKEN env)")
		uiPort := fs.Int("ui-port", client.DefaultUIPort, "local UI port (0 = off)")
		trayOn := fs.Bool("tray", true, "show tray icon (disable on headless linux)")
		hotkeySpec := fs.String("hotkey", client.DefaultHotkey, "global hotkey to copy and send (empty = off)")
		fs.Parse(os.Args[2:])
		if *config != "" {
			die(startRunLog(*config))
			die(applyRunConfig(*config, fs, server, name, token))
		}
		mustServer(server)
		c := client.New(*server, *name, *token)
		if *uiPort > 0 {
			go func() {
				if err := client.StartUI(c, *uiPort); err != nil {
					log.Printf("[ui] disabled: %v", err)
				}
			}()
		}
		go c.Run()
		if *hotkeySpec != "" {
			go func() {
				if err := client.RegisterHotkey(c, *hotkeySpec); err != nil {
					log.Printf("[hotkey] %v", err)
				}
			}()
		}
		if *trayOn {
			client.RunTray(c, *uiPort, *hotkeySpec)
		} else {
			select {}
		}

	case "send-clipboard":
		fs := flag.NewFlagSet("send-clipboard", flag.ExitOnError)
		server := fs.String("server", "", "")
		name := fs.String("name", defaultName(), "")
		token := fs.String("token", "", "")
		to := fs.String("to", "", "target device name (empty = all online)")
		fs.Parse(os.Args[2:])
		if *server == "" {
			viaDaemon, derr := client.SendClipboardViaDaemon(*to)
			if viaDaemon {
				die(derr)
				fmt.Printf("sent clipboard to %s via daemon\n", targetDesc(*to))
				return
			}
			fmt.Fprintln(os.Stderr, "local daemon not running; pass -server nas:9100 for direct mode")
			os.Exit(1)
		}
		mustServer(server)
		c := client.New(*server, *name, *token)
		die(client.SendCurrentClipboard(c, *to, c.StoreHooks()))
		fmt.Printf("sent clipboard to %s\n", targetDesc(*to))

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
		args := fs.Args()
		if len(args) == 0 {
			die(fmt.Errorf("usage: send-file [-server nas:9100] [-to device] <file...>"))
		}
		if *server != "" || len(args) > 1 {
			for _, p := range args {
				fid, size, err := client.UploadFile(*server, *token, p, base(p), nil)
				die(err)
				msg := proto.NewMessage("", proto.TypeFile)
				msg.Name = base(p)
				msg.Size = size
				msg.FID = fid
				msg.Target = *to
				die(client.SendPayload(*server, *name, *token, msg))
				fmt.Printf("sent file %s (%s) to %s\n", msg.Name, humanBytes(size), targetDesc(*to))
			}
			return
		}
		viaDaemon, derr := client.SendFileViaDaemon(args[0], *to)
		if !viaDaemon {
			fmt.Fprintln(os.Stderr, "local daemon not running; pass -server nas:9100 for direct mode")
			os.Exit(1)
		}
		die(derr)
		fmt.Printf("sent file %s via daemon\n", base(args[0]))

	case "detect":
		fs := flag.NewFlagSet("detect", flag.ExitOnError)
		fs.Parse(os.Args[2:])
		if refs, _ := client.ReadFileRefs(); len(refs) > 0 {
			fmt.Printf("kind=file, %d item(s):\n", len(refs))
			for _, p := range refs {
				fmt.Println("  " + p)
			}
			return
		}
		msg, hint, err := client.DetectAndBuildClipboardMessage()
		die(err)
		if hint {
			fmt.Println("text too large for inline sync")
			return
		}
		preview := msg.Content
		if msg.Type == proto.TypeImage {
			fmt.Printf("kind=%s size=%d (binary)\n", msg.Type, msg.Size)
			return
		}
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		fmt.Printf("kind=%s size=%d content=%q\n", msg.Type, msg.Size, preview)

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
		path, err := client.DownloadTo(*server, *token, fs.Args()[0], *out, nil)
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
  detect                                         inspect what the clipboard resolves to
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
