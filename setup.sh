#!/bin/bash
set -e

echo "=== VPS Dashboard Otomatik Kurulum Sihirbazı ==="
echo "Bu betik gerekli paketleri kuracak, projeyi derleyecek ve servisi başlatacaktır."
echo ""

# 1. Root Yetkisi Kontrolü
if [ "$EUID" -ne 0 ]; then
  echo "Lütfen bu betiği root yetkisiyle (sudo ile) çalıştırın."
  exit 1
fi

# 2. Bağımlılıkların Kurulumu
echo "-> Sistem paketleri güncelleniyor ve bağımlılıklar yükleniyor..."
apt-get update
apt-get install -y nginx certbot python3-certbot-nginx git curl unzip snapd

# Go Kurulumu (Snap ile en güncel sürüm)
if ! command -v go &> /dev/null; then
  echo "-> Go programlama dili kuruluyor..."
  snap install go --classic
fi

# Node.js Kurulumu (NodeSource 20.x)
if ! command -v node &> /dev/null; then
  echo "-> Node.js kuruluyor..."
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  apt-get install -y nodejs
fi

# 3. Projenin Klonlanması
if [ ! -d "/root/dashboard-src" ]; then
  echo "-> Proje GitHub'dan klonlanıyor..."
  git clone https://github.com/nazeg/dashboard.git /root/dashboard-src
  cd /root/dashboard-src
else
  echo "-> Proje güncelleniyor (git pull)..."
  cd /root/dashboard-src
  # Bekleyen yerel değişiklikler varsa temizle
  git reset --hard
  git pull
fi

# 4. Klasör Yapısının Hazırlanması
echo "-> Gerekli klasörler oluşturuluyor..."
mkdir -p /var/lib/dashboard/databases
mkdir -p /var/www
mkdir -p /root/dashboard

# Resmi PocketBase v0.30.2 temiz binary indirme (alt database örnekleri için)
if [ ! -f "/root/dashboard/pocketbase_bin" ]; then
  echo "-> PocketBase temiz resmi binary indiriliyor..."
  curl -L -o /tmp/pb.zip https://github.com/pocketbase/pocketbase/releases/download/v0.30.2/pocketbase_0.30.2_linux_amd64.zip
  unzip -o /tmp/pb.zip pocketbase -d /tmp/
  mv /tmp/pocketbase /root/dashboard/pocketbase_bin
  chmod +x /root/dashboard/pocketbase_bin
  rm -f /tmp/pb.zip
fi

# 5. Frontend Derleme (Build)
echo "-> Frontend bağımlılıkları yükleniyor ve derleniyor..."
cd web
rm -rf node_modules
npm install --unsafe-perm=true --include=dev
npm run build --unsafe-perm=true
cd ..

# 6. Backend Derleme (Build)
echo "-> Backend derleniyor..."
/snap/bin/go build -o /root/dashboard/dashboard .

# 7. Geçici dosyalar temizleniyor
echo "-> Temizlik yapılıyor..."
# Kaynak klasörü silmiyoruz, böylece bir sonraki güncellemede sadece git pull yapılabilir.

# 8. Systemd Servisi Oluşturma
echo "-> Arka plan servisi oluşturuluyor..."
DEPLOY_USER=${SUDO_USER:-root}
echo "-> Tespit edilen deploy kullanıcısı: $DEPLOY_USER"

cat <<EOF > /etc/systemd/system/dashboard.service
[Unit]
Description=VPS Dashboard Manager
After=network.target nginx.service

[Service]
Type=simple
User=root
WorkingDirectory=/root/dashboard
ExecStart=/root/dashboard/dashboard serve --http=0.0.0.0:8090
Restart=always
Environment=DEPLOY_USER=$DEPLOY_USER

[Install]
WantedBy=multi-user.target
EOF

# 9. Servisleri Başlatma
echo "-> Servisler başlatılıyor..."
systemctl daemon-reload
systemctl enable dashboard
systemctl restart dashboard

# 10. Durum Kontrolü
echo ""
echo "=== KURULUM TAMAMLANDI ==="
echo "VPS Dashboard başarıyla kuruldu ve başlatıldı!"
echo "Yönetim Paneli Adresi: http://$(curl -s https://ipinfo.io/ip):8090"
echo "--------------------------------------------------------"
# journalctl satırları kırpmaz, böylece kurulum linki tam haliyle ekranda görünür
journalctl -u dashboard -n 15 --no-pager
