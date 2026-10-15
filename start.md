# Set environment variables (atau gunakan default)
export MONGO_URI="mongodb://localhost:27017"
export MONGO_DATABASE="garda_inflasi_analytics"

# Jalankan seeder
cd apps/api && go run ./cmd/seed-analytics


# 1. Pastikan MongoDB dan PostgreSQL running
# 2. Seed data analytics
cd apps/api && go run ./cmd/seed-analytics

# 3. Jalankan server
go run ./cmd/api

# 4. Buka browser
open http://localhost:8080/swagger/

Catatan
Semua endpoint menggunakan JWT Authentication — dapatkan token dari endpoint auth, lalu klik "Authorize" di Swagger UI
Endpoint dilindungi oleh permission middleware sesuai domain (dashboard.eksekutif, dashboard.operasional, prediksi.baca, lapangan.validasi, lapangan.baca)
Query params sudah memiliki default values di usecase layer, jadi endpoint tetap berfungsi meski tanpa parameter
