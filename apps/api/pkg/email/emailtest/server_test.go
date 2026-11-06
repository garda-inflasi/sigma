package emailtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/pkg/email"
)

func TestServer_MenangkapPesanDanToken(t *testing.T) {
	srv := Start(t)
	sender := email.New(srv.Config("https://garda.test"))

	require.NoError(t, sender.SendUndangan("Budi@Garda.test", "Budi", "tok_123-abc", time.Hour))

	pesan := srv.Tunggu(t, "budi@garda.test", 1)
	require.Len(t, pesan, 1)
	assert.Equal(t, []string{"Budi@Garda.test"}, pesan[0].To)
	assert.Equal(t, "tok_123-abc", pesan[0].Token())
	assert.Empty(t, srv.Untuk("lain@garda.test"))
	assert.Equal(t, srv.Host(), "127.0.0.1")
	assert.NotEmpty(t, srv.Port())
	assert.GreaterOrEqual(t, srv.MaksKoneksiSerempak(), 1)
}

func TestServer_TolakPenerima(t *testing.T) {
	srv := Start(t)
	sender := email.New(srv.Config(""))
	srv.TolakPenerima(true)

	require.Error(t, sender.SendHTML("a@b.id", "Judul", "<p>isi</p>"))
	assert.Empty(t, srv.Messages())

	srv.TolakPenerima(false)
	require.NoError(t, sender.SendHTML("a@b.id", "Judul", "<p>isi</p>"))
	assert.Len(t, srv.Messages(), 1)
}

func TestMessage_TokenTanpaTautan(t *testing.T) {
	assert.Empty(t, Message{Data: "tanpa tautan"}.Token())
	assert.Equal(t, "x", Message{Data: "https://a.id/p?x=1&token=x&y=2"}.Token())
}
