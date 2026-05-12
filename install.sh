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

# 2. Sistem Bağımlılıklarının Kurulması
echo -e "\n${BLUE}[Aşama 1/6] Sistem bağımlılıkları kontrol ediliyor...${NC}"
apt-get update
apt-get install -y build-essential gcc g++ make curl unzip git

# cc linki yoksa manuel oluştur (bazı minimal sistemlerde gerekebilir)
if ! command -v cc &> /dev/null; then
    echo -e "${YELLOW}cc komutu bulunamadı, symlink oluşturuluyor...${NC}"
    ln -sf /usr/bin/gcc /usr/bin/cc
fi
echo -e "${GREEN}✅ Sistem bağımlılıkları hazır.${NC}"

# 3. PocketBase Kurulumu
echo -e "\n${BLUE}[Aşama 2/6] PocketBase binary kontrol ediliyor...${NC}"
mkdir -p /opt/pocketbase
if [ ! -f "/opt/pocketbase/pocketbase" ]; then
    echo -e "${YELLOW}PocketBase bulunamadı, indiriliyor...${NC}"
    PB_VERSION=$(curl -s https://api.github.com/repos/pocketbase/pocketbase/releases/latest | grep tag_name | cut -d '"' -f 4 | sed 's/v//')
    curl -L -o /tmp/pb.zip "https://github.com/pocketbase/pocketbase/releases/download/v${PB_VERSION}/pocketbase_${PB_VERSION}_linux_amd64.zip"
    unzip -o /tmp/pb.zip -d /opt/pocketbase/
    chmod +x /opt/pocketbase/pocketbase
    rm /tmp/pb.zip
    echo -e "${GREEN}✅ PocketBase ${PB_VERSION} başarıyla kuruldu.${NC}"
else
    echo -e "${GREEN}✅ PocketBase zaten mevcut.${NC}"
fi

# 4. Dizinlerin Oluşturulması
echo -e "\n${BLUE}[Aşama 3/6] Dizinler hazırlanıyor...${NC}"
mkdir -p /var/www /var/pb-data
chown -R $USER:$USER /var/www /opt/pocketbase /var/pb-data
echo -e "${GREEN}✅ Dizinler ve izinler hazır.${NC}"

# 5. Bağımlılıkların Kurulması (Temiz Kurulum)
echo -e "\n${BLUE}[Aşama 4/6] Node.js bağımlılıkları temizleniyor ve kuruluyor...${NC}"
# Eski hatalı kurulumları temizle
rm -rf node_modules package-lock.json
npm cache clean --force

# Bağımlılıkları kur
npm install
echo -e "${GREEN}✅ Bağımlılıklar kuruldu.${NC}"

# 6. PM2 Kontrolü ve Kurulumu
echo -e "\n${BLUE}[Aşama 5/6] PM2 kontrol ediliyor...${NC}"
if ! command -v pm2 &> /dev/null; then
    echo -e "${YELLOW}PM2 bulunamadı, kuruluyor...${NC}"
    npm install -g pm2
else
    echo -e "${GREEN}PM2 zaten kurulu.${NC}"
fi

# 7. Başlatma
echo -e "\n${BLUE}[Aşama 6/6] Dashboard başlatılıyor...${NC}"
pm2 start index.js --name "dashboard"
pm2 save
# PM2 startup komutu root olarak çalıştırıldığında sistemi otomatik yapılandırır
pm2 startup | grep "sudo" | bash

echo -e "\n${GREEN}✨ Kurulum Başarıyla Tamamlandı!${NC}"
echo -e "${BLUE}🔗 Dashboard adresi:${NC} http://localhost:3000"
echo -e "${YELLOW}Not: Nginx yönetimi için uygulamanın root yetkisiyle çalışması gerekmektedir.${NC}"
