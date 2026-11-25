# Self-Order Cafe (QR Meja) — Backend

Backend Go untuk sistem pemesanan mandiri cafe/UMKM berdasarkan QR meja.

- **Stack:** Go (Gin), GORM + PostgreSQL, JWT + bcrypt, REST API (polling kasir 3 detik).
- **Payment gateway:** Midtrans Core API (QRIS) dengan fallback mock.

## Struktur

```text
backend/
├── cmd/server/main.go        # entry point + routing
├── internal/
│   ├── config/               # env loader
│   ├── models/               # GORM models (Table, Category, MenuItem, User, Order, OrderItem, Payment)
│   ├── database/             # koneksi DB + AutoMigrate
│   ├── seed/                 # seed awal (admin, kategori, menu, meja)
│   ├── middleware/           # JWT auth + CORS
│   ├── payment/              # interface Gateway + adapter Midtrans & mock
│   ├── handlers/             # public, admin, auth, webhook
│   └── jobs/                 # worker expire order (ticker 1 menit)
├── Dockerfile
├── docker-compose.yml
├── Caddyfile
└── .env.example
```

## Menjalankan

### Docker Compose (app + PostgreSQL)

```bash
cp .env.example .env   # sesuaikan kredensial DB, JWT, dan Midtrans
docker compose up --build
```

Server berjalan di `http://localhost:8080`.

### Lokal (Go + PostgreSQL)

```bash
cp .env.example .env
# isi DATABASE_URL dan kredensial Midtrans di .env
go run ./cmd/server
```

Saat aplikasi pertama kali jalan, `AUTO_MIGRATE` membuat skema tabel dan `SEED` mengisi data awal:
- **Admin default:** `admin` / `admin123` (atur lewat `ADMIN_USERNAME` & `ADMIN_PASSWORD`).
- **Menu:** 4 kategori contoh (Kopi, Non-Kopi, Makanan, Snack).
- **Meja:** 6 meja (nomor 1–6) dengan token QR acak.

## Konfigurasi Midtrans

Atur variabel berikut pada `.env` untuk mengaktifkan pembayaran QRIS asli:

```env
PAYMENT_GATEWAY=midtrans
MIDTRANS_SERVER_KEY=SB-Mid-server-xxxx
MIDTRANS_IS_PRODUCTION=false
```

Gunakan `PAYMENT_GATEWAY=mock` dan `PAYMENT_WEBHOOK_SECRET=dev-webhook-secret` jika ingin menguji tanpa koneksi ke Midtrans.

## Endpoint

### Publik (tanpa login)

| Method | Path | Fungsi |
|--------|------|--------|
| GET | `/api/public/tables/:token` | Validasi token QR dan ambil detail meja |
| GET | `/api/public/menu` | Ambil daftar kategori dan item yang tersedia |
| POST | `/api/public/orders` | Buat order (`table_token`, `items[]`) -> `code`, `total`, `qr_string`, `expires_at` |
| GET | `/api/public/orders/:code` | Cek status order dan pembayaran |
| POST | `/api/webhooks/payment` | Terima notifikasi Midtrans (verifikasi signature SHA-512, idempotent) |

### Kasir & Admin (`Authorization: Bearer <token>`)

| Method | Path | Role | Fungsi |
|--------|------|------|--------|
| POST | `/api/auth/login` | publik | Login dan dapatkan JWT |
| GET | `/api/admin/orders` | kasir/admin | List order aktif (filter `?status=`) |
| PATCH | `/api/admin/orders/:id/status` | kasir/admin | Ubah status order |
| GET | `/api/admin/menu-items` | admin | List semua item menu |
| POST | `/api/admin/menu-items` | admin | Tambah item menu |
| PATCH | `/api/admin/menu-items/:id` | admin | Ubah item menu |
| DELETE | `/api/admin/menu-items/:id` | admin | Hapus item menu (soft delete) |
| PATCH | `/api/admin/menu-items/:id/availability` | admin | Ubah status stok (tersedia/habis) |
| GET | `/api/admin/categories` | admin | List kategori |
| POST | `/api/admin/categories` | admin | Tambah kategori |
| GET | `/api/admin/tables` | admin | List meja beserta URL QR |
| POST | `/api/admin/tables` | admin | Tambah meja |
| PATCH | `/api/admin/tables/:id` | admin | Ubah nama atau status aktif meja |
| GET | `/api/admin/tables/:id/qr` | admin | Unduh gambar QR meja (PNG) |

## Aturan Bisnis

- **Kalkulasi harga:** Server menghitung ulang total dari harga database, mengabaikan nominal dari client. Harga disimpan sebagai integer rupiah.
- **Alur status:** `pending_payment -> paid -> preparing -> ready -> completed` (cabang: `expired` / `cancelled`). Kasir dan admin hanya mengubah transisi dari `paid` hingga `completed`.
- **Tampilan kasir:** Endpoint order kasir hanya mengembalikan transaksi berstatus `paid` ke atas.
- **Keamanan webhook:** Sistem memvalidasi `signature_key` dari Midtrans (`SHA512(order_id + status_code + gross_amount + ServerKey)`) dan menolak request tidak valid dengan kode `401`. Pemrosesan bersifat idempotent.
- **Kedaluwarsa otomatis:** Background job memeriksa dan mengubah order `pending_payment` yang melewati batas waktu menjadi `expired` setiap 1 menit.

## Pengujian Webhook

### Simulasi Midtrans (Sandbox)

Gunakan fitur **Test Notification** di Midtrans Dashboard atau kirim payload manual ke `/api/webhooks/payment`:

```bash
ORDER_ID="ORD-xxx"
STATUS_CODE="200"
GROSS_AMOUNT="26000.00"
SERVER_KEY="SB-Mid-server-xxxx"
SIG=$(printf '%s' "${ORDER_ID}${STATUS_CODE}${GROSS_AMOUNT}${SERVER_KEY}" | openssl dgst -sha512 -hex | awk '{print $2}')

curl -X POST http://localhost:8080/api/webhooks/payment \
  -H 'Content-Type: application/json' \
  -d "{
    \"order_id\": \"$ORDER_ID\",
    \"status_code\": \"$STATUS_CODE\",
    \"gross_amount\": \"$GROSS_AMOUNT\",
    \"transaction_status\": \"settlement\",
    \"signature_key\": \"$SIG\"
  }"
```

### Mode Mock (`PAYMENT_GATEWAY=mock`)

```bash
BODY='{"payment_id":"MOCK-xxx","order_code":"ORD-xxx","status":"paid","amount":26000}'
SIG=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac 'dev-webhook-secret' -hex | awk '{print $2}')

curl -X POST http://localhost:8080/api/webhooks/payment \
  -H 'Content-Type: application/json' \
  -H "X-Signature: $SIG" \
  -d "$BODY"
```