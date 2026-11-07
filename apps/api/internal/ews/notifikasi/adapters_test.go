package notifikasi

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEmail struct {
	to, subject, body string
	err               error
	block             chan struct{}
}

func (f *fakeEmail) SendHTML(to, subject, body string) error {
	if f.block != nil {
		<-f.block
	}
	f.to, f.subject, f.body = to, subject, body
	return f.err
}

func TestEmailPengirim_BuildsEscapedMessageWithLink(t *testing.T) {
	sender := &fakeEmail{}
	p := NewEmailPengirim(sender, "https://garda.example.id/")

	err := p.Kirim(t.Context(), domain.Pesan{Tujuan: "ops@x.id", Severity: "KRITIS", Judul: "Lonjakan <b>cabai</b>", Isi: "Naik & tinggi", TautanPath: "/ews?id=EWS-2000"})

	require.NoError(t, err)
	assert.Equal(t, domain.KanalEmail, p.Kanal())
	assert.Equal(t, "ops@x.id", sender.to)
	assert.Equal(t, "[EWS KRITIS] Lonjakan <b>cabai</b>", sender.subject)
	assert.NotContains(t, sender.body, "<b>cabai</b>", "isi alert di-escape di badan HTML")
	assert.Contains(t, sender.body, "Lonjakan &lt;b&gt;cabai&lt;/b&gt;")
	assert.Contains(t, sender.body, "Naik &amp; tinggi")
	assert.Contains(t, sender.body, `href="https://garda.example.id/ews?id=EWS-2000"`)
}

func TestEmailPengirim_WithoutBaseURLOmitsLink(t *testing.T) {
	sender := &fakeEmail{}
	require.NoError(t, NewEmailPengirim(sender, "").Kirim(t.Context(), domain.Pesan{Tujuan: "a@b.id", TautanPath: "/ews"}))
	assert.NotContains(t, sender.body, "href")
}

func TestEmailPengirim_Errors(t *testing.T) {
	assert.ErrorIs(t, NewEmailPengirim(&fakeEmail{}, "").Kirim(t.Context(), domain.Pesan{Tujuan: "  "}), domain.ErrTanpaTujuan)

	assert.ErrorIs(t, NewEmailPengirim(&fakeEmail{err: assert.AnError}, "").Kirim(t.Context(), domain.Pesan{Tujuan: "a@b.id"}), assert.AnError)

	block := make(chan struct{})
	defer close(block)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	err := NewEmailPengirim(&fakeEmail{block: block}, "").Kirim(ctx, domain.Pesan{Tujuan: "a@b.id"})
	assert.ErrorIs(t, err, context.DeadlineExceeded, "SMTP yang menggantung tidak menahan pekerja")
}

// fakeStore adalah LanggananStore dalam memori.
type fakeStore struct {
	mu      sync.Mutex
	subs    []domain.LanggananPush
	dihapus []string
	listErr error
}

func (f *fakeStore) ListLangganan(context.Context, string) ([]domain.LanggananPush, error) {
	return f.subs, f.listErr
}

func (f *fakeStore) HapusLanggananKadaluwarsa(_ context.Context, endpoint string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dihapus = append(f.dihapus, endpoint)
	return nil
}

// kunciKlien membuat pasangan kunci langganan yang sah (p256dh dan auth) seperti dari browser.
func kunciKlien(t *testing.T) (p256dh, auth string) {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	require.NoError(t, err)
	secret := make([]byte, 16)
	_, err = rand.Read(secret)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(secret)
}

func vapidUji(t *testing.T) VAPID {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	return VAPID{PublicKey: pub, PrivateKey: priv, Subject: "mailto:ews@example.id"}
}

// layananPush menjalankan endpoint push palsu di localhost (tanpa layanan luar).
func layananPush(t *testing.T, status int) (url string, hits *int) {
	t.Helper()
	var n int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		mu.Unlock()
		assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "vapid "), "ditandatangani VAPID")
		assert.Equal(t, "aes128gcm", r.Header.Get("Content-Encoding"), "payload terenkripsi")
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &n
}

func TestPushPengirim_SendsEncryptedPayloadToEveryDevice(t *testing.T) {
	url1, hits1 := layananPush(t, http.StatusCreated)
	url2, hits2 := layananPush(t, http.StatusCreated)
	k1, a1 := kunciKlien(t)
	k2, a2 := kunciKlien(t)
	store := &fakeStore{subs: []domain.LanggananPush{
		{Endpoint: url1, P256dh: k1, Auth: a1}, {Endpoint: url2, P256dh: k2, Auth: a2},
	}}
	p := NewPushPengirim(store, vapidUji(t))

	err := p.Kirim(t.Context(), domain.Pesan{PenggunaID: "u", AlertID: 12, Severity: "KRITIS", Judul: "Lonjakan", Isi: "Naik", TautanPath: "/ews?id=12"})

	require.NoError(t, err)
	assert.Equal(t, domain.KanalPush, p.Kanal())
	assert.Equal(t, 1, *hits1)
	assert.Equal(t, 1, *hits2)
	assert.Empty(t, store.dihapus)
}

func TestPushPengirim_DropsExpiredSubscriptionsAndSucceedsIfAnyDeviceReceives(t *testing.T) {
	urlOK, _ := layananPush(t, http.StatusCreated)
	urlGone, _ := layananPush(t, http.StatusGone)
	k, a := kunciKlien(t)
	store := &fakeStore{subs: []domain.LanggananPush{{Endpoint: urlGone, P256dh: k, Auth: a}, {Endpoint: urlOK, P256dh: k, Auth: a}}}

	err := NewPushPengirim(store, vapidUji(t)).Kirim(t.Context(), domain.Pesan{PenggunaID: "u"})

	require.NoError(t, err)
	assert.Equal(t, []string{urlGone}, store.dihapus)
}

func TestPushPengirim_NoUsableSubscriptionIsNoDestination(t *testing.T) {
	k, a := kunciKlien(t)
	urlGone, _ := layananPush(t, http.StatusNotFound)

	for name, store := range map[string]*fakeStore{
		"tanpa langganan":      {},
		"semua langganan mati": {subs: []domain.LanggananPush{{Endpoint: urlGone, P256dh: k, Auth: a}}},
	} {
		err := NewPushPengirim(store, vapidUji(t)).Kirim(t.Context(), domain.Pesan{PenggunaID: "u"})
		assert.ErrorIs(t, err, domain.ErrTanpaTujuan, name)
	}
}

func TestPushPengirim_ServerErrorsAreRetriable(t *testing.T) {
	k, a := kunciKlien(t)
	url, _ := layananPush(t, http.StatusInternalServerError)
	store := &fakeStore{subs: []domain.LanggananPush{{Endpoint: url, P256dh: k, Auth: a}}}

	err := NewPushPengirim(store, vapidUji(t)).Kirim(t.Context(), domain.Pesan{PenggunaID: "u"})

	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrTanpaTujuan)
	assert.ErrorContains(t, err, "500")

	store.listErr = errors.New("db putus")
	assert.ErrorContains(t, NewPushPengirim(store, vapidUji(t)).Kirim(t.Context(), domain.Pesan{}), "db putus")
}

func TestDariEnv_OnlyConfiguredAdaptersAreCreated(t *testing.T) {
	for _, k := range []string{"SMTP_HOST", "VAPID_PUBLIC_KEY", "VAPID_PRIVATE_KEY", "VAPID_SUBJECT"} {
		t.Setenv(k, "")
	}
	assert.Empty(t, DariEnv(&fakeStore{}), "tanpa env: hanya outbox, tidak ada pengiriman nyata")

	t.Setenv("SMTP_HOST", "smtp.example.com")
	got := DariEnv(&fakeStore{})
	require.Len(t, got, 1)
	assert.Equal(t, domain.KanalEmail, got[0].Kanal())

	v := vapidUji(t)
	t.Setenv("VAPID_PUBLIC_KEY", v.PublicKey)
	t.Setenv("VAPID_PRIVATE_KEY", v.PrivateKey)
	got = DariEnv(&fakeStore{})
	require.Len(t, got, 2)
	assert.Equal(t, domain.KanalPush, got[1].Kanal())

	t.Setenv("VAPID_PRIVATE_KEY", "")
	assert.Len(t, DariEnv(&fakeStore{}), 1, "kunci VAPID harus lengkap")
}
