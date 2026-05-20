# Nazploy (VPS Dashboard)

Web sitelerini yönetmek için **Pocketbase tabanlı** bir dashboard. Nginx konfigürasyonlarını otomatik oluşturur, Let's Encrypt SSL yönetimi yapar, ve her site için ayrı Pocketbase veritabanı oluşturmanıza olanak tanır.

## Mimari

```
┌─────────────────────────────────────────┐
│             Nazploy (Go Binary)         │
│  ┌─────────────────────────────────┐    │
│  │  Pocketbase (Embedded)          │    │
│  │  - Admin UI: /_/                │    │
│  │  - API: /api/dashboard/*        │    │
│  │  - Koleksiyonlar: sites, db     │    │
│  └─────────────────────────────────┘    │
│  ┌─────────────────────────────────┐    │
│  │  React SPA (Go embed ile gömülü)│    │
│  │  - Dashboard                    │    │
│  │  - Site CRUD                    │    │
│  │  - SSL Yönetimi                 │    │
│  │  - Veritabanı Yönetimi          │    │
│  └─────────────────────────────────┘    │
│  ┌─────────────────────────────────┐    │
│  │  Nginx Manager                  │    │
│  │  - Config oluşturma             │    │
│  │  - Config yazma / sembolik link │    │
│  │  - nginx reload                 │    │
│  └─────────────────────────────────┘    │
│  ┌─────────────────────────────────┐    │
│  │  SSL Manager                    │    │
│  │  - Certbot ile Let's Encrypt    │    │
│  │  - Sertifika sorgulama/silme    │    │
│  └─────────────────────────────────┘    │
│  ┌─────────────────────────────────┐    │
│  │  Port Manager                   │    │
│  │  - Otomatik port tahsisi        │    │
│  │  - Port havuzu (10000-20000)    │    │
│  └─────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

## Özellikler

- **Web Sitesi Yönetimi** — Statik site veya proxy olarak site ekleme/düzenleme/silme
- **Otomatik Nginx Config** — Her site için /etc/nginx/sites-available/config yazılır, sembolik link ile aktifleştirilir
- **Otomatik Port Tahsisi** — 10000-20000 port aralığında otomatik port ataması
- **SSL / Let's Encrypt** — Certbot ile tek tıkla SSL sertifikası oluşturma ve kaldırma
- **Pocketbase Veritabanı** — Her site için ayrı Pocketbase instance'ı oluşturma
- **Tek Binary** — Frontend Go embed ile binary'nin içinde, tek dosya kurulum

## Gereksinimler

- **Go 1.22+** (derlemek için)
- **Node.js 18+** (frontend build için)
- **Nginx** (VPS'te)
- **Certbot** (SSL için)

## Kurulum

### Otomatik Kurulum (Önerilen)

Tek bir komutla tüm bağımlılıkları (`go`, `nodejs`, `nginx`, `certbot` vb.) kurabilir, projeyi derleyebilir ve arka planda çalışan bir `systemd` servisi olarak başlatabilirsiniz:

```bash
curl -sSL https://raw.githubusercontent.com/nazeg/nazploy/main/setup.sh | sudo bash
```

*Not: Bu betik projeyi otomatik olarak `/root/nazploy-src` dizinine klonlar, gerekli derleme işlemlerini yapar ve uygulamayı `/root/nazploy/` konumunda bir systemd servisi (`nazploy.service`) olarak kurup başlatır.*

### Manuel Kurulum / Güncelleme

Eğer depoyu zaten klonladıysanız veya yerel kopyadan kurmak istiyorsanız:

```bash
git clone https://github.com/nazeg/nazploy.git
cd nazploy
sudo chmod +x setup.sh
sudo ./setup.sh
```

### Tarayıcıdan Erişim

Kurulum tamamlandıktan sonra tarayıcınızdan aşağıdaki adreslere erişebilirsiniz:

- **Dashboard:** `http://SUNUCU_IP:8090`
- **Pocketbase Admin:** `http://SUNUCU_IP:8090/_/`

İlk açılışta bir admin hesabı oluşturun. Bu hesapla dashboard'a giriş yapabilirsiniz.

## Kullanım

### Site Ekleme

1. Dashboard'dan "Yeni Site" butonuna tıklayın
2. Site adı, domain ve türünü girin:
   - **Statik**: HTML/CSS/JS dosyaları `/var/www/domainadi/` dizinine kopyalanır
   - **Proxy**: İstekler belirttiğiniz adrese yönlendirilir (ör. localhost:3000)
3. Nginx config'i otomatik oluşturulur ve nginx reload edilir

### SSL Ekleme

1. Site detay sayfasına gidin
2. "SSL Ekle" butonuna tıklayın
3. Certbot Let's Encrypt sertifikası oluşturur ve Nginx config güncellenir
4. SSL otomatik olarak HTTP'den HTTPS'e yönlendirme ekler

### Veritabanı Ekleme

1. Site detay sayfasında "Veritabanı Ekle"ye tıklayın
2. Veritabanı adı ve admin email girin
3. Yeni bir Pocketbase instance'ı ayrı bir portta başlatılır
4. Bağlantı bilgileri dashboard'da görüntülenir

## Proje Yapısı

```
nazploy/
├── main.go                          # Giriş noktası
├── go.mod                           # Go modül
├── Makefile                         # Build komutları
├── README.md                        # Bu dosya
├── migrations/
│   └── 001_init.go                  # Pocketbase koleksiyon migration'ları
├── internal/
│   └── dashboard/
│       ├── models.go                # Veri modelleri ve sabitler
│       ├── server.go                # API handler'lar
│       ├── nginx.go                 # Nginx config yönetimi
│       ├── ssl.go                   # Let's Encrypt / Certbot
│       └── portmanager.go           # Port tahsisi
└── web/                             # React frontend
    ├── index.html
    ├── package.json
    ├── vite.config.ts
    ├── tailwind.config.js
    ├── tsconfig.json
    └── src/
        ├── main.tsx
        ├── App.tsx
        ├── types.ts
        ├── lib/pocketbase.ts
        ├── components/Sidebar.tsx
        └── pages/
            ├── LoginPage.tsx
            ├── DashboardOverview.tsx
            ├── SitesList.tsx
            ├── SiteForm.tsx
            ├── SiteDetail.tsx
            └── NginxStatus.tsx
```

## API Endpoints

| Metot | Endpoint | Açıklama |
|-------|----------|----------|
| GET | /api/dashboard/stats | Dashboard istatistikleri |
| GET | /api/dashboard/sites | Tüm siteleri listele |
| POST | /api/dashboard/sites | Yeni site ekle |
| GET | /api/dashboard/sites/:id | Site detayı |
| PATCH | /api/dashboard/sites/:id | Siteyi güncelle |
| DELETE | /api/dashboard/sites/:id | Siteyi sil |
| POST | /api/dashboard/sites/:id/deploy | Siteyi deploy et (nginx reload) |
| POST | /api/dashboard/sites/:id/ssl/enable | SSL etkinleştir |
| POST | /api/dashboard/sites/:id/ssl/disable | SSL devre dışı |
| GET | /api/dashboard/sites/:id/ssl/status | SSL durumu |
| GET | /api/dashboard/sites/:id/databases | Veritabanlarını listele |
| POST | /api/dashboard/sites/:id/databases | Veritabanı ekle |
| DELETE | /api/dashboard/sites/:id/databases/:dbId | Veritabanını sil |
| GET | /api/dashboard/nginx/status | Nginx durumu |
| POST | /api/dashboard/nginx/reload | Nginx yeniden yükle |
