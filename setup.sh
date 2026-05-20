#!/bin/bash
set -e

echo "=== Nazploy Otomatik Kurulum Sihirbazı ==="
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
if [ ! -d "/root/nazploy-src" ]; then
  echo "-> Proje GitHub'dan klonlanıyor..."
  git clone https://github.com/nazeg/nazploy.git /root/nazploy-src
  cd /root/nazploy-src
else
  echo "-> Proje güncelleniyor (git pull)..."
  cd /root/nazploy-src
  # Bekleyen yerel değişiklikler varsa temizle
  git reset --hard
  git pull
fi

# 4. Klasör Yapısının Hazırlanması
echo "-> Gerekli klasörler oluşturuluyor..."
mkdir -p /var/lib/dashboard/databases
mkdir -p /var/www
mkdir -p /root/nazploy

# Resmi PocketBase v0.30.2 temiz binary indirme (alt database örnekleri için)
if [ ! -f "/root/nazploy/pocketbase_bin" ]; then
  echo "-> PocketBase temiz resmi binary indiriliyor..."
  curl -L -o /tmp/pb.zip https://github.com/pocketbase/pocketbase/releases/download/v0.30.2/pocketbase_0.30.2_linux_amd64.zip
  unzip -o /tmp/pb.zip pocketbase -d /tmp/
  mv /tmp/pocketbase /root/nazploy/pocketbase_bin
  chmod +x /root/nazploy/pocketbase_bin
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
/snap/bin/go build -o /root/nazploy/nazploy .

# 7. Geçici dosyalar temizleniyor
echo "-> Temizlik yapılıyor..."
# Kaynak klasörü silmiyoruz, böylece bir sonraki güncellemede sadece git pull yapılabilir.

# 8. Systemd Servisi Oluşturma
echo "-> Arka plan servisi oluşturuluyor..."
DEPLOY_USER=${SUDO_USER:-root}
echo "-> Tespit edilen deploy kullanıcısı: $DEPLOY_USER"

# Eski dashboard servisini durdur ve sil (eğer varsa geçiş için kolaylık)
if systemctl is-active --quiet dashboard; then
  echo "-> Eski dashboard servisi durduruluyor..."
  systemctl stop dashboard
  systemctl disable dashboard
  rm -f /etc/systemd/system/dashboard.service
fi

cat <<EOF > /etc/systemd/system/nazploy.service
[Unit]
Description=Nazploy Site Manager
After=network.target nginx.service

[Service]
Type=simple
User=root
WorkingDirectory=/root/nazploy
ExecStart=/root/nazploy/nazploy serve --http=0.0.0.0:8090
Restart=always
Environment=DEPLOY_USER=$DEPLOY_USER

[Install]
WantedBy=multi-user.target
EOF

# 9. Servisleri Başlatma
echo "-> Servisler başlatılıyor..."
systemctl daemon-reload
systemctl enable nazploy
systemctl restart nazploy

# 10. Durum Kontrolü
echo ""
echo "=== KURULUM TAMAMLANDI ==="
echo "Nazploy başarıyla kuruldu ve başlatıldı!"
echo "Yönetim Paneli Adresi: http://$(curl -s https://ipinfo.io/ip):8090"
echo "--------------------------------------------------------"
# journalctl satırları kırpmaz, böylece kurulum linki tam haliyle ekranda görünür
journalctl -u nazploy -n 15 --no-pager
