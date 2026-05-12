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
