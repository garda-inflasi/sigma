package seedkit

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Registry menyimpan seeder terdaftar.
type Registry struct {
	byName map[string]Seeder
}

// NewRegistry membuat registry kosong.
func NewRegistry() *Registry {
	return &Registry{byName: map[string]Seeder{}}
}

var defaultRegistry = NewRegistry()

// Register mendaftarkan seeder ke registry bawaan. Dipanggil dari init() di
// cmd/seed/register_<modul>.go; seeder tak valid atau ganda adalah kesalahan
// pemrograman sehingga langsung panic.
func Register(s Seeder) {
	if err := defaultRegistry.Register(s); err != nil {
		panic("seedkit: " + err.Error())
	}
}

// Plan menyusun urutan jalan seeder bawaan untuk lapis yang dipilih.
func Plan(tiers ...Tier) ([]Seeder, error) {
	return defaultRegistry.Plan(tiers...)
}

// Register menambahkan seeder; galat bila tak valid atau namanya sudah ada.
func (r *Registry) Register(s Seeder) error {
	if err := s.validate(); err != nil {
		return err
	}
	if _, exists := r.byName[s.Name]; exists {
		return fmt.Errorf("seeder %q terdaftar dua kali", s.Name)
	}
	r.byName[s.Name] = s
	return nil
}

// Plan mengembalikan seeder pada lapis yang dipilih, terurut topologis menurut
// DependsOn dan After. Dependensi yang berada di lapis yang tidak dipilih
// dianggap sudah dijalankan. Hasil deterministik: di antara seeder yang siap,
// lapis lebih awal dan nama lebih kecil lebih dulu.
func (r *Registry) Plan(tiers ...Tier) ([]Seeder, error) {
	if err := r.checkDependencies(); err != nil {
		return nil, err
	}

	selected := map[string]Seeder{}
	for _, s := range r.byName {
		if slices.Contains(tiers, s.Tier) {
			selected[s.Name] = s
		}
	}
	return sortTopologically(selected)
}

func (r *Registry) checkDependencies() error {
	var problems []string
	for _, s := range r.byName {
		for _, dep := range s.DependsOn {
			if _, ok := r.byName[dep]; !ok {
				problems = append(problems, fmt.Sprintf("%s bergantung pada %s yang tidak terdaftar", s.Name, dep))
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("dependensi seeder tidak lengkap: %s", strings.Join(problems, "; "))
}

// sortTopologically memakai algoritma Kahn. Prasyarat tiap seeder adalah
// DependsOn dan After yang ikut terpilih.
func sortTopologically(selected map[string]Seeder) ([]Seeder, error) {
	waiting := make(map[string]map[string]bool, len(selected))
	for name, s := range selected {
		waiting[name] = map[string]bool{}
		for _, dep := range append(append([]string{}, s.DependsOn...), s.After...) {
			if _, ok := selected[dep]; ok {
				waiting[name][dep] = true
			}
		}
	}

	ordered := make([]Seeder, 0, len(selected))
	for len(waiting) > 0 {
		next, ok := firstReady(waiting, selected)
		if !ok {
			return nil, fmt.Errorf("dependensi seeder berputar: %s", strings.Join(sortedKeys(waiting), ", "))
		}
		ordered = append(ordered, selected[next])
		delete(waiting, next)
		for _, pending := range waiting {
			delete(pending, next)
		}
	}
	return ordered, nil
}

// firstReady memilih seeder tanpa prasyarat tersisa: lapis lebih awal dulu, lalu nama.
func firstReady(waiting map[string]map[string]bool, selected map[string]Seeder) (string, bool) {
	best, found := "", false
	for name, pending := range waiting {
		if len(pending) > 0 {
			continue
		}
		if !found || comesBefore(selected[name], selected[best]) {
			best, found = name, true
		}
	}
	return best, found
}

func comesBefore(a, b Seeder) bool {
	if a.Tier.rank() != b.Tier.rank() {
		return a.Tier.rank() < b.Tier.rank()
	}
	return a.Name < b.Name
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
