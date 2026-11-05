package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// kunciFake adalah domain.KunciRepository dalam memori. Aturannya datang dari metode domain.KunciLogin yang
// sama dengan repositori Postgres, jadi yang diuji di sini adalah perilaku usecase terhadap aturan itu;
// SQL-nya diuji di paket integration. Nilai nol siap dipakai (tertanam di mockRepository).
type kunciFake struct {
	mu    sync.Mutex
	baris map[string]*domain.KunciLogin

	// Galat yang dikembalikan metode terkait bila diisi (untuk menguji kegagalan penyimpanan).
	errMulai, errKembali, errHapus, errAktif error
	// Penghitung panggilan.
	nMulai, nKembali, nHapus int
}

var _ domain.KunciRepository = (*kunciFake)(nil)

func (f *kunciFake) simpanan() map[string]*domain.KunciLogin {
	if f.baris == nil {
		f.baris = map[string]*domain.KunciLogin{}
	}
	return f.baris
}

// kunciTerkunci memasang kunci yang masih berlaku 10 menit untuk email.
func (f *kunciFake) kunciTerkunci(email string) { f.kunciSampai(email, jam().Add(10*time.Minute)) }

// kunciSampai memasang kunci untuk email sampai waktu tertentu.
func (f *kunciFake) kunciSampai(email string, sampai time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	hash := HashEmailKunci(email)
	f.simpanan()[hash] = &domain.KunciLogin{EmailHash: hash, Gagal: 5, JendelaMulai: sampai.Add(-15 * time.Minute), TerkunciSampai: &sampai, DiperbaruiPada: jam()}
}

// keadaan mengembalikan salinan baris email, nil bila tidak ada.
func (f *kunciFake) keadaan(email string) *domain.KunciLogin {
	f.mu.Lock()
	defer f.mu.Unlock()
	if b, ok := f.simpanan()[HashEmailKunci(email)]; ok {
		salinan := *b
		return &salinan
	}
	return nil
}

func (f *kunciFake) MulaiPercobaan(_ context.Context, emailHash string, now time.Time, maks int, durasi time.Duration) (domain.KunciLogin, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nMulai++
	if f.errMulai != nil {
		return domain.KunciLogin{}, false, f.errMulai
	}
	saat, ada := f.simpanan()[emailHash]
	if !ada {
		saat = &domain.KunciLogin{EmailHash: emailHash, JendelaMulai: now}
	}
	baru, boleh := saat.CatatPercobaan(now, maks, durasi)
	if boleh {
		f.simpanan()[emailHash] = &baru
	}
	return baru, boleh, nil
}

func (f *kunciFake) KembalikanPercobaan(_ context.Context, emailHash string, maks int, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nKembali++
	if f.errKembali != nil {
		return f.errKembali
	}
	if saat, ada := f.simpanan()[emailHash]; ada {
		baru := saat.Kembalikan(maks, now)
		f.simpanan()[emailHash] = &baru
	}
	return nil
}

func (f *kunciFake) HapusKunci(_ context.Context, emailHash string) (*domain.KunciLogin, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nHapus++
	if f.errHapus != nil {
		return nil, f.errHapus
	}
	saat, ada := f.simpanan()[emailHash]
	if !ada {
		return nil, nil
	}
	delete(f.simpanan(), emailHash)
	return saat, nil
}

func (f *kunciFake) KunciAktif(_ context.Context, emailHash []string, now time.Time) (map[string]time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errAktif != nil {
		return nil, f.errAktif
	}
	aktif := map[string]time.Time{}
	for _, h := range emailHash {
		if b, ok := f.simpanan()[h]; ok && b.Terkunci(now) {
			aktif[h] = *b.TerkunciSampai
		}
	}
	return aktif, nil
}
