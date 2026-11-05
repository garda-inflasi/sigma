package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func str(s string) *string { return &s }

func TestDiffState(t *testing.T) {
	cases := []struct {
		name             string
		sebelum, sesudah *string
		want             []Perubahan
	}{
		{
			name:    "bidang diubah, ditambah, dihapus; yang sama diabaikan",
			sebelum: str(`{"peran":"A","aktif":true,"lama":1}`),
			sesudah: str(`{"peran":"B","aktif":true,"baru":"x"}`),
			want: []Perubahan{
				{Path: "baru", Jenis: PerubahanDitambah, Sesudah: json.RawMessage(`"x"`)},
				{Path: "lama", Jenis: PerubahanDihapus, Sebelum: json.RawMessage(`1`)},
				{Path: "peran", Jenis: PerubahanDiubah, Sebelum: json.RawMessage(`"A"`), Sesudah: json.RawMessage(`"B"`)},
			},
		},
		{
			name:    "objek bersarang diratakan dengan path bertitik",
			sebelum: str(`{"alamat":{"kota":"Bogor","rt":1}}`),
			sesudah: str(`{"alamat":{"kota":"Depok","rt":1}}`),
			want: []Perubahan{
				{Path: "alamat.kota", Jenis: PerubahanDiubah, Sebelum: json.RawMessage(`"Bogor"`), Sesudah: json.RawMessage(`"Depok"`)},
			},
		},
		{
			name:    "larik dibandingkan utuh",
			sebelum: str(`{"tag":["a","b"]}`),
			sesudah: str(`{"tag":["a","c"]}`),
			want: []Perubahan{
				{Path: "tag", Jenis: PerubahanDiubah, Sebelum: json.RawMessage(`["a","b"]`), Sesudah: json.RawMessage(`["a","c"]`)},
			},
		},
		{
			name:    "angka 1 dan 1.0 dianggap sama",
			sebelum: str(`{"n":1}`),
			sesudah: str(`{"n":1.0}`),
			want:    []Perubahan{},
		},
		{
			name:    "tanpa sebelum, semua bidang ditambah",
			sebelum: nil,
			sesudah: str(`{"a":1,"b":null}`),
			want: []Perubahan{
				{Path: "a", Jenis: PerubahanDitambah, Sesudah: json.RawMessage(`1`)},
				{Path: "b", Jenis: PerubahanDitambah, Sesudah: json.RawMessage(`null`)},
			},
		},
		{
			name:    "tanpa sesudah, semua bidang dihapus",
			sebelum: str(`{"a":1}`),
			sesudah: nil,
			want:    []Perubahan{{Path: "a", Jenis: PerubahanDihapus, Sebelum: json.RawMessage(`1`)}},
		},
		{
			name:    "keadaan skalar memakai path akar",
			sebelum: str(`"OPS_MAHASISWA"`),
			sesudah: str(`"OPS_KOORDINATOR"`),
			want: []Perubahan{
				{Path: RootPath, Jenis: PerubahanDiubah, Sebelum: json.RawMessage(`"OPS_MAHASISWA"`), Sesudah: json.RawMessage(`"OPS_KOORDINATOR"`)},
			},
		},
		{name: "keduanya kosong", sebelum: nil, sesudah: nil, want: []Perubahan{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DiffState(tc.sebelum, tc.sesudah)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDiffState_RejectsInvalidJSON(t *testing.T) {
	_, err := DiffState(str(`{rusak`), nil)
	require.ErrorContains(t, err, "sebelum")

	_, err = DiffState(nil, str(`{rusak`))
	require.ErrorContains(t, err, "sesudah")
}
