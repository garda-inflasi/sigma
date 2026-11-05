package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// Jenis perubahan pada satu bidang antara keadaan sebelum dan sesudah.
const (
	PerubahanDitambah = "DITAMBAH"
	PerubahanDiubah   = "DIUBAH"
	PerubahanDihapus  = "DIHAPUS"
)

// RootPath adalah path perubahan bila keadaan bukan objek JSON (mis. satu nilai).
const RootPath = "$"

// Perubahan adalah satu bidang yang berbeda antara sebelum dan sesudah. Sebelum
// kosong pada DITAMBAH, Sesudah kosong pada DIHAPUS.
type Perubahan struct {
	Path    string
	Jenis   string
	Sebelum json.RawMessage
	Sesudah json.RawMessage
}

// DiffState membandingkan dua keadaan objek (JSON) dan mengembalikan bidang yang
// berbeda, terurut menurut path. Objek bersarang diratakan dengan path bertitik
// ("alamat.kota"); larik dan nilai skalar dibandingkan utuh. Nilai nil berarti
// keadaan tidak dicatat: semua bidang pihak lain dianggap ditambah/dihapus.
func DiffState(sebelum, sesudah *string) ([]Perubahan, error) {
	before, err := flatten(sebelum)
	if err != nil {
		return nil, fmt.Errorf("sebelum: %w", err)
	}
	after, err := flatten(sesudah)
	if err != nil {
		return nil, fmt.Errorf("sesudah: %w", err)
	}

	out := []Perubahan{}
	for path, b := range before {
		a, ok := after[path]
		switch {
		case !ok:
			out = append(out, Perubahan{Path: path, Jenis: PerubahanDihapus, Sebelum: b})
		case !equalJSON(b, a):
			out = append(out, Perubahan{Path: path, Jenis: PerubahanDiubah, Sebelum: b, Sesudah: a})
		}
	}
	for path, a := range after {
		if _, ok := before[path]; !ok {
			out = append(out, Perubahan{Path: path, Jenis: PerubahanDitambah, Sesudah: a})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// flatten meratakan objek JSON ke peta path → nilai mentah.
func flatten(raw *string) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	if raw == nil {
		return out, nil
	}
	var root any
	if err := json.Unmarshal([]byte(*raw), &root); err != nil {
		return nil, fmt.Errorf("JSON tidak valid: %w", err)
	}
	if err := walk(out, "", root); err != nil {
		return nil, err
	}
	return out, nil
}

func walk(out map[string]json.RawMessage, path string, value any) error {
	if object, ok := value.(map[string]any); ok && len(object) > 0 {
		for key, child := range object {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			if err := walk(out, childPath, child); err != nil {
				return err
			}
		}
		return nil
	}
	if path == "" {
		path = RootPath
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	out[path] = encoded
	return nil
}

func equalJSON(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return bytes.Equal(a, b)
	}
	return reflect.DeepEqual(x, y)
}
