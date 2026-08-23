//go:build darwin

package client

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

const furlScript = `ObjC.import('AppKit')
const pb = $.NSPasteboard.generalPasteboard
const classes = $.NSArray.arrayWithObject($.NSURL)
const items = pb.readObjectsForClassesOptions(classes, $())
let out = ''
if (!items.isNil()) {
  const n = items.count
  for (let i = 0; i < n; i++) out += items.objectAtIndex(i).path.js + '\n'
}
out`

func ReadFileRefs() ([]string, error) {
	cmd := exec.Command("osascript", "-l", "JavaScript", "-")
	cmd.Stdin = strings.NewReader(furlScript)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("clipboard file refs: %v %s", err, strings.TrimSpace(stderr.String()))
	}
	var refs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			refs = append(refs, line)
		}
	}
	return refs, nil
}
