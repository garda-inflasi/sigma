package http

import (
	"encoding/json"
	"strings"
)

// SaringDemoData membuang baris dan blok bertanda DEMO dari payload PostgreSQL ketika pemanggil meminta mode
// `prod` (X-Garda-Data-Env: prod), sehingga data demo tidak pernah bercampur dengan data produksi.
func SaringDemoData(data interface{}) interface{} {
	if data == nil {
		return nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return data
	}
	dibersihkan, berubah := saringNilai(decoded)
	if !berubah {
		return data
	}
	return dibersihkan
}

func adalahStringDemo(v any) bool {
	s, ok := v.(string)
	return ok && strings.EqualFold(strings.TrimSpace(s), "DEMO")
}

func adalahBlokDemo(m map[string]any) bool {
	for _, kunci := range []string{"sumber", "sumber_model", "sumber_narasi", "pemicu"} {
		if adalahStringDemo(m[kunci]) {
			return true
		}
	}
	return false
}

func saringNilai(v any) (any, bool) {
	switch x := v.(type) {
	case []any:
		out := make([]any, 0, len(x))
		berubah := false
		for _, elem := range x {
			if m, ok := elem.(map[string]any); ok {
				if adalahBlokDemo(m) || (m["demo"] == true && m["inflasi"] == nil && m["bronze"] == nil) {
					berubah = true
					continue
				}
			}
			bersih, b := saringNilai(elem)
			if b {
				berubah = true
			}
			out = append(out, bersih)
		}
		return out, berubah

	case map[string]any:
		berubah := false
		// PipelineResponse dengan demo=true: nolkan hitungan demo alih-alih menghapus bentuk respons.
		if x["demo"] == true && x["bronze"] != nil {
			x["demo"] = false
			x["bronze"] = 0
			x["silver"] = 0
			x["gold"] = 0
			if mutu, ok := x["mutu"].(map[string]any); ok {
				for k := range mutu {
					mutu[k] = nil
				}
			}
			return x, true
		}
		if x["demo"] == true {
			x["demo"] = false
			berubah = true
		}
		for k, val := range x {
			if sub, ok := val.(map[string]any); ok && adalahBlokDemo(sub) {
				x[k] = nil
				berubah = true
				continue
			}
			bersih, b := saringNilai(val)
			if b {
				berubah = true
				x[k] = bersih
			}
		}
		return x, berubah

	default:
		return v, false
	}
}
