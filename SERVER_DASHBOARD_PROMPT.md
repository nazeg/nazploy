# Server Dashboard — Build Prompt

## Proje Özeti

Ubuntu sunucusunda birden fazla web projesini yönetmek için minimalist bir **admin dashboard** yap. Dokploy benzeri ama çok daha hafif. Tek binary ya da basit `node index.js` ile ayağa kalksın.

---

## Teknoloji Kararları

- **Backend:** Node.js + Express
- **Veritabanı:** SQLite (better-sqlite3)
- **Process Manager:** PM2 (sistem genelinde kurulu varsayılır)
- **Reverse Proxy:** Nginx (sistem genelinde kurulu varsayılır)
- **Dosya Yöneticisi:** Dashboard içinde hem web UI hem SFTP desteği
- **Opsiyonel DB:** PocketBase (her projeye bağımsız instance)
- **Frontend:** Vanilla JS + minimal CSS (framework yok, build adımı yok)
- **Auth:** Basit kullanıcı adı + şifre (bcrypt), JWT session

---

## Dizin Yapısı

Aşağıdaki yapıyı oluştur:

```
/opt/dashboard/
├── index.js                  # Express uygulaması, giriş noktası
├── package.json
├── .env                      # PORT, SECRET_KEY, ADMIN_USER, ADMIN_PASS
├── dashboard.db              # SQLite veritabanı (otomatik oluşturulsun)
├── routes/
│   ├── auth.js               # Login/logout
│   ├── projects.js           # Proje CRUD
│   ├── services.js           # PM2 yönetimi
│   ├── pocketbase.js         # PocketBase instance yönetimi
│   ├── files.js              # Dosya yöneticisi API
│   └── nginx.js              # Nginx config yönetimi
├── lib/
│   ├── db.js                 # SQLite bağlantısı ve migration
│   ├── pm2.js                # PM2 wrapper (exec ile)
│   ├── nginx.js              # Config generate + reload
│   ├── pocketbase.js         # PocketBase binary yönetimi
│   └── files.js              # Dosya sistemi işlemleri
└── public/
    ├── index.html            # Login sayfası
    ├── dashboard.html        # Ana dashboard
    └── assets/
        ├── app.js            # Frontend JS (vanilla)
        └── style.css         # Minimal CSS
```

Proje dosyaları şu konuma deploy edilsin:
```
/var/www/{proje-slug}/        # Her proje için ayrı dizin
```

PocketBase binary'leri:
```
/opt/pocketbase/              # PocketBase executable'ları
/var/pb-data/{proje-slug}/    # Her PocketBase instance'ının data dizini
```

---

## Veritabanı Şeması

`lib/db.js` içinde şu tabloları oluştur:

```sql
CREATE TABLE IF NOT EXISTS projects (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,       -- nginx ve dizin adı için
  domain TEXT,                     -- örn: myapp.example.com
  type TEXT NOT NULL,              -- 'static' | 'node' | 'php'
  root_path TEXT NOT NULL,         -- /var/www/{slug}
  port INTEGER,                    -- node app portu (3001, 3002...)
  entry_file TEXT,                 -- node için: server.js, index.js vb.
  env_vars TEXT,                   -- JSON string
  status TEXT DEFAULT 'stopped',   -- 'running' | 'stopped' | 'error'
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pocketbase_instances (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
  port INTEGER UNIQUE NOT NULL,    -- 8090'dan başlayarak otomatik atanır
  data_path TEXT NOT NULL,
  status TEXT DEFAULT 'stopped',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS nginx_configs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
  config_path TEXT NOT NULL,       -- /etc/nginx/sites-available/{slug}
  is_active INTEGER DEFAULT 1,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## Özellikler ve API Endpoint'leri

### 1. Auth (`routes/auth.js`)
```
POST /api/auth/login     → { username, password } → JWT token
POST /api/auth/logout    → token sil
GET  /api/auth/me        → oturum bilgisi
```
Tüm `/api/*` endpoint'leri JWT middleware ile korunsun.

---

### 2. Proje Yönetimi (`routes/projects.js`)

```
GET    /api/projects          → tüm projeleri listele
POST   /api/projects          → yeni proje ekle
PUT    /api/projects/:id      → proje güncelle
DELETE /api/projects/:id      → projeyi sil (dizin dahil, confirm flag'i ile)
GET    /api/projects/:id/logs → PM2 loglarını getir (son 100 satır)
```

**POST /api/projects body:**
```json
{
  "name": "My App",
  "slug": "my-app",
  "domain": "myapp.example.com",
  "type": "node",
  "port": 3001,
  "entry_file": "server.js",
  "env_vars": { "NODE_ENV": "production" }
}
```

Proje oluşturulduğunda otomatik olarak:
1. `/var/www/{slug}/` dizini oluşturulsun
2. Nginx config oluşturulsun ve `sites-enabled`'a symlink atılsın
3. `sudo nginx -t && sudo systemctl reload nginx` çalıştırılsın

---

### 3. Servis Yönetimi (`routes/services.js`)

```
POST /api/projects/:id/start    → PM2 ile başlat
POST /api/projects/:id/stop     → PM2 ile durdur
POST /api/projects/:id/restart  → PM2 ile yeniden başlat
GET  /api/projects/:id/status   → PM2 process durumu (CPU, RAM, uptime)
GET  /api/services/overview     → tüm PM2 process'leri özet
```

PM2 komutları `child_process.exec` ile çalıştırılsın:
```js
// Örnek start komutu
pm2 start /var/www/{slug}/{entry_file} --name {slug} --cwd /var/www/{slug}
```

Static projeler için PM2 gerekmez, sadece nginx serve eder.

---

### 4. PocketBase Yönetimi (`routes/pocketbase.js`)

```
POST   /api/projects/:id/pocketbase          → PocketBase instance ekle
DELETE /api/projects/:id/pocketbase          → instance sil
POST   /api/projects/:id/pocketbase/start    → başlat
POST   /api/projects/:id/pocketbase/stop     → durdur
GET    /api/projects/:id/pocketbase/status   → durum
```

**Instance ekleme mantığı:**
- Mevcut instance portlarına bak, en yüksek port + 1 ata (başlangıç: 8090)
- Data dizini: `/var/pb-data/{slug}/`
- PocketBase binary: `/opt/pocketbase/pocketbase`
- Başlatma: `./pocketbase serve --http="127.0.0.1:{port}" --dir="{data_path}"`
- Nginx'e `/_pocketbase/` path'i veya subdomain olarak ekle (opsiyonel)

**Önemli:** PocketBase binary'nin sistemde kurulu olup olmadığını kontrol et. Yoksa indirme URL'si ver: `https://github.com/pocketbase/pocketbase/releases`

---

### 5. Dosya Yöneticisi (`routes/files.js`)

```
GET    /api/files?path=/var/www/my-app       → dizin listele
GET    /api/files/read?path=...              → dosya içeriğini oku
POST   /api/files/write                      → dosya yaz/güncelle
POST   /api/files/upload                     → dosya yükle (multipart)
DELETE /api/files?path=...                   → dosya/dizin sil
POST   /api/files/mkdir                      → dizin oluştur
POST   /api/files/rename                     → yeniden adlandır
```

**Güvenlik:** Tüm path işlemlerinde `/var/www/` dışına çıkışı engelle (path traversal koruması). `path.resolve` ile kontrol et.

---

### 6. Nginx Config Generator (`lib/nginx.js`)

Her proje tipi için template:

**Static site:**
```nginx
server {
    listen 80;
    server_name {domain};
    root /var/www/{slug};
    index index.html;
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

**Node.js app:**
```nginx
server {
    listen 80;
    server_name {domain};
    location / {
        proxy_pass http://127.0.0.1:{port};
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

Config dosyası `/etc/nginx/sites-available/{slug}` olarak yazılsın. Symlink: `/etc/nginx/sites-enabled/{slug}`.

---

## Frontend (Dashboard UI)

`public/dashboard.html` tek sayfalık uygulama olsun. Aşağıdaki bölümleri içersin:

### Sol sidebar
- Dashboard logosu / başlığı
- Navigasyon: Projeler, Servisler, Dosyalar, Ayarlar
- Sistem özeti (CPU, RAM — `/api/system/stats` endpoint'i, `os` modülüyle)

### Ana içerik alanı

**Projeler sayfası:**
- Proje kartları: isim, domain, tip, durum (yeşil/kırmızı badge)
- Her kartta: Başlat / Durdur / Yeniden Başlat / Dosyaları Aç / Sil butonları
- PocketBase varsa: "PocketBase: port 8090 ✓" göster, admin paneline link ver
- Yeni Proje butonu → modal form

**Dosya Yöneticisi sayfası:**
- Proje seç (dropdown)
- Klasör ağacı (sol) + dosya listesi (sağ)
- Dosya yükleme (drag & drop)
- Dosya düzenleme (basit textarea — syntax highlight gerekmez)

**Loglar:**
- Proje seç, son 100 satır PM2 logu göster
- 5 saniyede bir otomatik yenile (polling, websocket gerekmez)

### UI Tasarım Prensipleri
- Renk paleti: koyu arka plan tercih edilir (dark theme), minimal renkler
- Font: system-ui
- Çerçeve/kütüphane yok — vanilla JS fetch API kullan
- Animasyon yok, sade tablo/kart layout
- Mobile uyumluluk zorunlu değil, masaüstü öncelikli

---

## Kurulum Script'i

`install.sh` dosyası oluştur:

```bash
#!/bin/bash
# Dashboard kurulum scripti

# Gerekli dizinleri oluştur
sudo mkdir -p /var/www /opt/pocketbase /var/pb-data
sudo chown -R $USER:$USER /var/www /var/pb-data

# npm bağımlılıklarını kur
cd /opt/dashboard && npm install

# PM2'yi global kur (yoksa)
which pm2 || sudo npm install -g pm2

# Dashboard'u PM2 ile başlat
pm2 start index.js --name "dashboard"
pm2 save
pm2 startup

echo "Dashboard çalışıyor: http://localhost:3000"
```

---

## .env Örneği

`.env.example` dosyası oluştur:

```env
PORT=3000
SECRET_KEY=change-this-to-random-string
ADMIN_USER=admin
ADMIN_PASS=changeme
NGINX_SITES_AVAILABLE=/etc/nginx/sites-available
NGINX_SITES_ENABLED=/etc/nginx/sites-enabled
WWW_ROOT=/var/www
PB_BINARY=/opt/pocketbase/pocketbase
PB_DATA_ROOT=/var/pb-data
PB_BASE_PORT=8090
```

---

## Güvenlik Notları

1. Dashboard sadece `localhost:3000`'de dinlesin, Nginx ile dışarı aç
2. Tüm `exec` çağrılarında argümanları sanitize et, shell injection'a karşı
3. Dosya yöneticisinde `/var/www` dışına çıkışı engelle
4. JWT token 8 saat expire olsun
5. Rate limiting ekle login endpoint'ine (express-rate-limit)

---

## Teslim Edilecekler

Şu dosyaların hepsini yaz:

- [ ] `package.json` (bağımlılıklar: express, better-sqlite3, bcrypt, jsonwebtoken, multer, express-rate-limit, dotenv)
- [ ] `index.js`
- [ ] `lib/db.js` (schema + migration)
- [ ] `lib/pm2.js`
- [ ] `lib/nginx.js`
- [ ] `lib/pocketbase.js`
- [ ] `lib/files.js`
- [ ] `routes/auth.js`
- [ ] `routes/projects.js`
- [ ] `routes/services.js`
- [ ] `routes/pocketbase.js`
- [ ] `routes/files.js`
- [ ] `routes/nginx.js`
- [ ] `public/index.html` (login)
- [ ] `public/dashboard.html` (ana UI)
- [ ] `public/assets/app.js`
- [ ] `public/assets/style.css`
- [ ] `install.sh`
- [ ] `.env.example`
- [ ] `README.md` (kurulum adımları)

---

## Kapsam Dışı (yapma)

- Docker entegrasyonu
- Git deploy / webhook
- SSL/Let's Encrypt otomasyonu (Nginx'te manuel yapılır)
- Çoklu kullanıcı / rol sistemi
- Metrik geçmişi / grafik

---

## Başlangıç Komutu

Tüm dosyaları oluşturduktan sonra şunu çalıştır ve hata varsa düzelt:

```bash
cd /opt/dashboard && npm install && node index.js
```
