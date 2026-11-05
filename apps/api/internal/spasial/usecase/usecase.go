// Package usecase berisi logika bisnis (baca-saja) modul spasial.
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, log: log}
}

func (uc *UseCase) ListMarker(ctx context.Context) ([]domain.Marker, error) {
	items, err := uc.repo.ListMarker(ctx)
	if err != nil {
		uc.log.Error("spasial: marker gagal", map[string]interface{}{"error": err.Error()})
	}
	return items, err
}

func (uc *UseCase) ListNeraca(ctx context.Context, q domain.NeracaQuery) ([]domain.Neraca, error) {
	items, err := uc.repo.ListNeraca(ctx, q)
	if err != nil {
		uc.log.Error("spasial: neraca gagal", map[string]interface{}{"error": err.Error()})
	}
	return items, err
}

func (uc *UseCase) ListRute(ctx context.Context) ([]domain.Rute, error) {
	items, err := uc.repo.ListRute(ctx)
	if err != nil {
		uc.log.Error("spasial: rute gagal", map[string]interface{}{"error": err.Error()})
	}
	return items, err
}

func (uc *UseCase) ListKandidat(ctx context.Context) ([]domain.Kandidat, error) {
	items, err := uc.repo.ListKandidat(ctx)
	if err != nil {
		uc.log.Error("spasial: kandidat gagal", map[string]interface{}{"error": err.Error()})
	}
	return items, err
}

// DefaultZoom adalah tingkat zoom bawaan untuk penyederhanaan geometri.
const DefaultZoom = 8

// Fitur adalah geometri satu wilayah yang sudah disederhanakan untuk tingkat zoom tertentu.
type Fitur struct {
	domain.GeometriWilayah
	// Geometri adalah dokumen GeoJSON hasil penyederhanaan.
	Geometri json.RawMessage
}

// Geometri mengembalikan geometri wilayah yang disederhanakan menurut zoom;
// zoom negatif memakai DefaultZoom. bbox (opsional) membatasi pada wilayah yang
// kotak pembatasnya beririsan.
func (uc *UseCase) Geometri(ctx context.Context, zoom int, bbox *domain.BBox) ([]Fitur, error) {
	if zoom < 0 {
		zoom = DefaultZoom
	}
	items, err := uc.repo.ListGeometri(ctx, bbox)
	if err != nil {
		uc.log.Error("spasial: geometri gagal", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	tol := domain.ToleransiZoom(zoom)
	out := make([]Fitur, len(items))
	for i, it := range items {
		g, err := domain.ParseGeoJSON([]byte(it.GeoJSON))
		if err != nil {
			uc.log.Error("spasial: geometri rusak", map[string]interface{}{"kabupaten": it.KabupatenKode, "error": err.Error()})
			return nil, fmt.Errorf("geometri %s: %w", it.KabupatenKode, err)
		}
		raw, err := g.Sederhanakan(tol).GeoJSON()
		if err != nil {
			return nil, fmt.Errorf("geometri %s: %w", it.KabupatenKode, err)
		}
		out[i] = Fitur{GeometriWilayah: it, Geometri: raw}
	}
	return out, nil
}

// ErrTidakAdaWilayah dikembalikan Lokasi bila belum ada wilayah sama sekali.
var ErrTidakAdaWilayah = fmt.Errorf("belum ada data wilayah")

// Lokasi mencari wilayah untuk satu titik: wilayah yang poligonnya memuat titik
// (bila poligon tumpang tindih, yang pusatnya terdekat), atau bila tak ada,
// wilayah dengan pusat terdekat.
func (uc *UseCase) Lokasi(ctx context.Context, lat, lng float64) (domain.LokasiWilayah, error) {
	pusat, err := uc.repo.ListMarker(ctx)
	if err != nil {
		uc.log.Error("spasial: lokasi gagal", map[string]interface{}{"error": err.Error()})
		return domain.LokasiWilayah{}, err
	}
	geometri, err := uc.repo.ListGeometri(ctx, nil)
	if err != nil {
		uc.log.Error("spasial: lokasi gagal", map[string]interface{}{"error": err.Error()})
		return domain.LokasiWilayah{}, err
	}

	jarak := make(map[string]float64, len(pusat))
	nama := make(map[string]string, len(pusat))
	nearest, nearestKm := "", math.Inf(1)
	for _, m := range pusat {
		km := domain.JarakKm(lat, lng, m.Lat, m.Lng)
		jarak[m.Kode], nama[m.Kode] = km, m.Nama
		if km < nearestKm {
			nearest, nearestKm = m.Kode, km
		}
	}
	if nearest == "" {
		return domain.LokasiWilayah{}, ErrTidakAdaWilayah
	}

	dalam, dalamKm := "", math.Inf(1)
	for _, it := range geometri {
		g, err := domain.ParseGeoJSON([]byte(it.GeoJSON))
		if err != nil {
			return domain.LokasiWilayah{}, fmt.Errorf("geometri %s: %w", it.KabupatenKode, err)
		}
		if km, ok := jarak[it.KabupatenKode]; ok && g.Mengandung(lat, lng) && km < dalamKm {
			dalam, dalamKm = it.KabupatenKode, km
		}
	}
	if dalam != "" {
		return domain.LokasiWilayah{Kode: dalam, Nama: nama[dalam], DiDalam: true, JarakPusatKm: dalamKm}, nil
	}
	return domain.LokasiWilayah{Kode: nearest, Nama: nama[nearest], JarakPusatKm: nearestKm}, nil
}
