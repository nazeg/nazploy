#!/bin/bash
# Nazploy Temizlik ve Kaldırma Scripti

# Renkler
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${RED}⚠️  DİKKAT: Tüm veriler ve yapılandırmalar silinecektir!${NC}"

# Root Kontrolü
if [ "$EUID" -ne 0 ]; then 
  echo -e "${YELLOW}Lütfen bu scripti sudo ile çalıştırın.${NC}"
  exit 1
fi

# 1. PM2 Süreçlerini Durdur ve Sil
echo -e "\n${BLUE}[1/4] PM2 süreçleri temizleniyor...${NC}"
pm2 stop all
pm2 delete all
pm2 save
echo -e "${GREEN}✅ PM2 temizlendi.${NC}"

# 2. Dizinleri Sil
echo -e "\n${BLUE}[2/4] Veri dizinleri siliniyor...${NC}"
rm -rf /opt/pocketbase
rm -rf /var/pb-data
echo -e "${GREEN}✅ /opt/pocketbase ve /var/pb-data silindi.${NC}"

# 3. Nginx Yapılandırmalarını Temizle
echo -e "\n${BLUE}[3/4] Nginx yapılandırmaları temizleniyor...${NC}"
# Sadece nazploy tarafından oluşturulanları silmek daha güvenlidir
# Şimdilik genel bir temizlik yapıyoruz
rm -f /etc/nginx/sites-available/*
rm -f /etc/nginx/sites-enabled/*
systemctl reload nginx
echo -e "${GREEN}✅ Nginx temizlendi.${NC}"

# 4. Proje Temizliği
echo -e "\n${BLUE}[4/4] Yerel bağımlılıklar temizleniyor...${NC}"
rm -rf node_modules
rm -f package-lock.json
echo -e "${GREEN}✅ Temizlik tamamlandı.${NC}"

echo -e "\n${GREEN}✨ Her şey silindi. Sıfırdan kuruluma hazırsınız.${NC}"
