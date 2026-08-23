package client

import _ "embed"

//go:embed trayicon.png
var trayIconData []byte

func trayIcon() []byte { return trayIconData }
