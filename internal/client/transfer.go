package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/g4i/local-clipboard/internal/proto"
)

const maxUpload = 1 << 30

func authQuery(server, token string) string {
	u := "http://" + server + "/"
	if token != "" {
		u += "?token=" + url.QueryEscape(token)
	}
	return u
}

func UploadFile(server, name, token, path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	fi, _ := f.Stat()
	size := fi.Size()

	pr := &progressReader{r: f, total: size}
	q := "name=" + url.QueryEscape(filepath.Base(path))
	if token != "" {
		q += "&token=" + url.QueryEscape(token)
	}
	req, err := http.NewRequest("POST", "http://"+server+"/upload?"+q, pr)
	if err != nil {
		return "", 0, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	client := &http.Client{Timeout: 6 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	fmt.Println()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", 0, fmt.Errorf("upload failed: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", 0, err
	}
	return out.ID, size, nil
}

type progressReader struct {
	r       io.Reader
	total   int64
	done    int64
	lastPct int
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.total > 0 {
		pct := int(p.done * 100 / p.total)
		if pct != p.lastPct && pct%5 == 0 {
			fmt.Printf("\ruploading %d%% (%s/%s)", pct, humanBytes(p.done), humanBytes(p.total))
			p.lastPct = pct
		}
	}
	return n, err
}

func DownloadFile(server, token, fid, outDir string) (string, error) {
	name := fid
	if i := strings.Index(fid, "-"); i > 0 && len(fid) > i+1 {
		name = fid[i+1:]
	}
	dst := filepath.Join(outDir, filepath.Base(name))
	q := "id=" + url.QueryEscape(fid)
	if token != "" {
		q += "&token=" + url.QueryEscape(token)
	}
	resp, err := http.Get("http://" + server + "/download?" + q)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("download failed: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	pr := &dlProgress{r: resp.Body, total: resp.ContentLength}
	_, err = io.Copy(out, pr)
	closeErr := out.Close()
	fmt.Println()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return dst, nil
}

type dlProgress struct {
	r     io.Reader
	total int64
	done  int64
	last  int
}

func (p *dlProgress) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.total > 0 {
		pct := int(p.done * 100 / p.total)
		if pct != p.last && pct%5 == 0 {
			fmt.Printf("\rdownloading %d%% (%s/%s)", pct, humanBytes(p.done), humanBytes(p.total))
			p.last = pct
		}
	}
	return n, err
}

func DetectAndBuildClipboardMessage() (*proto.Message, []byte, bool, error) {
	img, imgErr := ReadImage()
	if imgErr == nil && len(img) > 0 {
		m := proto.NewMessage("", proto.TypeImage)
		m.Size = int64(len(img))
		m.Content = encodeBase64(img)
		return m, nil, false, nil
	}
	text, txtErr := ReadText()
	if txtErr != nil || text == "" {
		return nil, nil, false, fmt.Errorf("clipboard is empty (image read also failed: %v)", imgErr)
	}
	m := proto.NewMessage("", proto.TypeText)
	m.Size = int64(len(text))
	if m.Size > maxInlineText {
		return nil, nil, true, fmt.Errorf("text too large for inline sync (%s); put it in a file and use send-file", humanBytes(m.Size))
	}
	m.Content = text
	return m, nil, false, nil
}
