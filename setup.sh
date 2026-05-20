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
echo "-> Proje GitHub'dan çekiliyor..."
rm -rf /root/dashboard-install
git clone https://github.com/nazeg/dashboard.git /root/dashboard-install
cd /root/dashboard-install

# 4. Klasör Yapısının Hazırlanması
echo "-> Gerekli klasörler oluşturuluyor..."
mkdir -p /var/lib/dashboard/databases
mkdir -p /var/www

# 5. Frontend Derleme (Build)
echo "-> Frontend bağımlılıkları yükleniyor ve derleniyor..."
cd web
npm install
npm run build
cd ..

# 6. Backend Derleme (Build)
echo "-> Backend derleniyor..."
/snap/bin/go build -o /root/dashboard-install/vps-dashboard-bin .

# 7. Çalıştırılabilir Dosyayı Taşıma
echo "-> Uygulama taşınıyor..."
mkdir -p /root/dashboard
mv /root/dashboard-install/vps-dashboard-bin /root/dashboard/dashboard
rm -rf /root/dashboard-install

# 8. Systemd Servisi Oluşturma
echo "-> Arka plan servisi oluşturuluyor..."
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
systemctl status dashboard --no-pager
