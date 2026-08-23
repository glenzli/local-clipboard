package hub

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	mobileTrustFile         = "mobile_devices.json"
	mobileTrustVersion      = 1
	maxTrustedMobileDevices = 128
)

type trustedMobileDevice struct {
	Name      string `json:"name,omitempty"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
}

type mobileTrustDocument struct {
	Version int                            `json:"version"`
	Devices map[string]trustedMobileDevice `json:"devices"`
}

// mobileTrustStore owns durable browser credentials. Map keys are keyed
// digests of random cookie values; neither the shared token nor cookie value
// is written to disk.
type mobileTrustStore struct {
	mu      sync.Mutex
	path    string
	devices map[string]trustedMobileDevice
}

func loadMobileTrustStore(path string) (*mobileTrustStore, error) {
	store := emptyMobileTrustStore(path)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	var document mobileTrustDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if document.Version != mobileTrustVersion {
		return nil, errors.New("unsupported trusted mobile device store version")
	}
	if document.Devices != nil {
		store.devices = document.Devices
	}
	return store, nil
}

func emptyMobileTrustStore(path string) *mobileTrustStore {
	return &mobileTrustStore{path: path, devices: make(map[string]trustedMobileDevice)}
}

func (s *mobileTrustStore) valid(key string, now time.Time) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.devices[key]
	return ok && now.Unix() < device.ExpiresAt
}

func (s *mobileTrustStore) remember(key, name string, now, expires time.Time) error {
	if s == nil {
		return errors.New("trusted mobile device store is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	next := cloneTrustedMobileDevices(s.devices)
	pruneTrustedMobileDevices(next, now)
	if _, exists := next[key]; !exists && len(next) >= maxTrustedMobileDevices {
		forgetOldestTrustedMobileDevice(next)
	}
	next[key] = trustedMobileDevice{
		Name:      normalizeMobileName(name, "Mobile Web"),
		CreatedAt: now.Unix(),
		ExpiresAt: expires.Unix(),
	}
	if err := writeMobileTrustDocument(s.path, next); err != nil {
		return err
	}
	s.devices = next
	return nil
}

func (s *mobileTrustStore) forget(key string, now time.Time) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	next := cloneTrustedMobileDevices(s.devices)
	pruneTrustedMobileDevices(next, now)
	if _, ok := next[key]; !ok {
		return nil
	}
	delete(next, key)
	if err := writeMobileTrustDocument(s.path, next); err != nil {
		return err
	}
	s.devices = next
	return nil
}

func cloneTrustedMobileDevices(devices map[string]trustedMobileDevice) map[string]trustedMobileDevice {
	clone := make(map[string]trustedMobileDevice, len(devices))
	for key, device := range devices {
		clone[key] = device
	}
	return clone
}

func pruneTrustedMobileDevices(devices map[string]trustedMobileDevice, now time.Time) {
	for key, device := range devices {
		if now.Unix() >= device.ExpiresAt {
			delete(devices, key)
		}
	}
}

func forgetOldestTrustedMobileDevice(devices map[string]trustedMobileDevice) {
	var oldestKey string
	var oldestCreated int64
	for key, device := range devices {
		if oldestKey == "" || device.CreatedAt < oldestCreated {
			oldestKey = key
			oldestCreated = device.CreatedAt
		}
	}
	delete(devices, oldestKey)
}

func writeMobileTrustDocument(path string, devices map[string]trustedMobileDevice) error {
	data, err := json.Marshal(mobileTrustDocument{Version: mobileTrustVersion, Devices: devices})
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".mobile-devices-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
