package storage

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/docker/notary"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/utils"
)

// NewMemoryStore returns a MetadataStore that operates entirely in memory.
// Very useful for testing
func NewMemoryStore(initial map[data.RoleName][]byte) *MemoryStore {
	var consistent = make(map[data.RoleName][]byte)
	if initial == nil {
		initial = make(map[data.RoleName][]byte)
	} else {
		// add all seed meta to consistent
		for name, d := range initial {
			checksum := sha256.Sum256(d)
			path := utils.ConsistentName(name, checksum[:])
			consistent[data.RoleName(path)] = d
		}
	}
	return &MemoryStore{
		data:       initial,
		consistent: consistent,
	}
}

// MemoryStore implements a mock RemoteStore entirely in memory.
// For testing purposes only.
type MemoryStore struct {
	data       map[data.RoleName][]byte
	consistent map[data.RoleName][]byte
}

// GetSized returns up to size bytes of data references by name.
// If size is "NoSizeLimit", this corresponds to "infinite," but we cut off at a
// predefined threshold "notary.MaxDownloadSize", as we will always know the
// size for everything but a timestamp and sometimes a root,
// neither of which should be exceptionally large
func (m MemoryStore) GetSized(name string, size int64) ([]byte, error) {
	d, ok := m.data[data.RoleName(name)]
	if ok {
		if size == NoSizeLimit {
			size = notary.MaxDownloadSize
		}
		if int64(len(d)) < size {
			return d, nil
		}
		return d[:size], nil
	}
	d, ok = m.consistent[data.RoleName(name)]
	if ok {
		if int64(len(d)) < size {
			return d, nil
		}
		return d[:size], nil
	}
	return nil, ErrMetaNotFound{Resource: name}
}

// Get returns the data associated with name
func (m MemoryStore) Get(name string) ([]byte, error) {
	if d, ok := m.data[data.RoleName(name)]; ok {
		return d, nil
	}
	if d, ok := m.consistent[data.RoleName(name)]; ok {
		return d, nil
	}
	return nil, ErrMetaNotFound{Resource: name}
}

// Set sets the metadata value for the given name
func (m *MemoryStore) Set(name string, meta []byte) error {
	m.data[data.RoleName(name)] = meta

	parsedMeta := &data.SignedMeta{}
	err := json.Unmarshal(meta, parsedMeta)
	if err == nil {
		// no parse error means this is metadata and not a key, so store by version
		version := parsedMeta.Signed.Version
		versionedName := fmt.Sprintf("%d.%s", version, name)
		m.data[data.RoleName(versionedName)] = meta
	}

	checksum := sha256.Sum256(meta)
	path := utils.ConsistentName(data.RoleName(name), checksum[:])
	m.consistent[data.RoleName(path)] = meta
	return nil
}

// SetMulti sets multiple pieces of metadata for multiple names
// in a single operation.
func (m *MemoryStore) SetMulti(metas map[string][]byte) error {
	for role, blob := range metas {
		m.Set(role, blob)
	}
	return nil
}

// Remove removes the metadata for a single role - if the metadata doesn't
// exist, no error is returned
func (m *MemoryStore) Remove(name string) error {
	if meta, ok := m.data[data.RoleName(name)]; ok {
		checksum := sha256.Sum256(meta)
		path := utils.ConsistentName(data.RoleName(name), checksum[:])
		delete(m.data, data.RoleName(name))
		delete(m.consistent, data.RoleName(path))
	}
	return nil
}

// RemoveAll clears the existing memory store by setting this store as new empty one
func (m *MemoryStore) RemoveAll() error {
	*m = *NewMemoryStore(nil)
	return nil
}

// Location provides a human readable name for the storage location
func (m MemoryStore) Location() string {
	return "memory"
}

// ListFiles returns a list of all files. The names returned should be
// usable with Get directly, with no modification.
func (m *MemoryStore) ListFiles() []string {
	names := make([]string, 0, len(m.data))
	for n := range m.data {
		names = append(names, n.String())
	}
	return names
}
