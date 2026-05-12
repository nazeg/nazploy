#!/bin/bash
# Nazploy Dashboard Kurulum Scripti

# Renkler
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Nazploy Kurulumu Başlatılıyor...${NC}"

# 1. Root Kontrolü
if [ "$EUID" -ne 0 ]; then 
  echo -e "${YELLOW}⚠️  Hata: Bu scripti root yetkileriyle çalıştırmalısınız.${NC}"
  echo -e "Lütfen şu komutu kullanın: ${GREEN}sudo ./install.sh${NC}"
  exit 1
fi

# 2. Dizinlerin Oluşturulması
echo -e "\n${BLUE}[Aşama 1/4] Dizinler hazırlanıyor...${NC}"
mkdir -p /var/www /opt/pocketbase /var/pb-data
chown -R $USER:$USER /var/www /opt/pocketbase /var/pb-data
echo -e "${GREEN}✅ Dizinler ve izinler hazır.${NC}"

# 3. Bağımlılıkların Kurulması
echo -e "\n${BLUE}[Aşama 2/4] Node.js bağımlılıkları kuruluyor...${NC}"
npm install
echo -e "${GREEN}✅ Bağımlılıklar kuruldu.${NC}"

# 4. PM2 Kontrolü ve Kurulumu
echo -e "\n${BLUE}[Aşama 3/4] PM2 kontrol ediliyor...${NC}"
if ! command -v pm2 &> /dev/null; then
    echo -e "${YELLOW}PM2 bulunamadı, kuruluyor...${NC}"
    npm install -g pm2
else
    echo -e "${GREEN}PM2 zaten kurulu.${NC}"
fi

# 5. Başlatma
echo -e "\n${BLUE}[Aşama 4/4] Dashboard başlatılıyor...${NC}"
pm2 start index.js --name "dashboard"
pm2 save
# PM2 startup komutu root olarak çalıştırıldığında sistemi otomatik yapılandırır
pm2 startup | grep "sudo" | bash

echo -e "\n${GREEN}✨ Kurulum Başarıyla Tamamlandı!${NC}"
echo -e "${BLUE}🔗 Dashboard adresi:${NC} http://localhost:3000"
echo -e "${YELLOW}Not: Nginx yönetimi için uygulamanın root yetkisiyle çalışması gerekmektedir.${NC}"
