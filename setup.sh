#!/bin/bash
set -e

# ANSI Color Codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Installation Log File
LOG_FILE="/tmp/nazploy_install.log"
echo "=== Nazploy Kurulum Günlüğü ($(date)) ===" > "$LOG_FILE"

# 1. Root Yetkisi Kontrolü
if [ "$EUID" -ne 0 ]; then
  echo -e "❌ ${RED}${BOLD}HATA:${NC} Lütfen bu betiği root yetkisiyle (sudo ile) çalıştırın."
  exit 1
fi

# Banner ve Logo Tanımı
print_banner() {
  echo -e "${CYAN}"
  echo -e "    _   __                 __            "
  echo -e "   / | / /___ _____  _____/ /___  __  __ "
  echo -e "  /  |/ / __ \`/ __ \/ ___/ / __ \/ / / / "
  echo -e " / /|  / /_/ / /_/ / /  / / /_/ / /_/ /  "
  echo -e "/_/ |_/\__,_/ .___/_/  /_/\____/\__, /   "
  echo -e "           /_/                 /____/    "
  echo -e "${NC}"
  echo -e "${BOLD}--- Nazploy Otomatik Kurulum Sihirbazı ---${NC}"
  echo -e "Bu betik gerekli bağımlılıkları kuracak, projeyi derleyecek ve servisi başlatacaktır."
  echo ""
}

# Sistem Teşhis Bilgileri
print_system_diagnostics() {
  echo -e "🔍 ${BOLD}Sunucu Sistem Bilgileri:${NC}"
  
  # OS Name
  if [ -f /etc/os-release ]; then
    OS_NAME=$(grep -w "PRETTY_NAME" /etc/os-release | cut -d= -f2 | tr -d '"')
    echo -e "  🌐 ${CYAN}İşletim Sistemi:${NC} $OS_NAME"
  else
    echo -e "  🌐 ${CYAN}İşletim Sistemi:${NC} Bilinmiyor"
  fi

  # CPU Cores
  if command -v nproc &> /dev/null; then
    CPU_CORES=$(nproc)
    echo -e "  🧠 ${CYAN}CPU Çekirdek Sayısı:${NC} $CPU_CORES"
  else
    echo -e "  🧠 ${CYAN}CPU Çekirdek Sayısı:${NC} Bilinmiyor"
  fi

  # RAM Info
  if command -v free &> /dev/null; then
    RAM_TOTAL=$(free -h | awk '/^Mem:/{print $2}')
    echo -e "  ⚡ ${CYAN}Toplam RAM:${NC} $RAM_TOTAL"
  elif [ -f /proc/meminfo ]; then
    RAM_TOTAL=$(grep MemTotal /proc/meminfo | awk '{print $2/1024 " MB"}')
    echo -e "  ⚡ ${CYAN}Toplam RAM:${NC} $RAM_TOTAL"
  else
    echo -e "  ⚡ ${CYAN}Toplam RAM:${NC} Bilinmiyor"
  fi

  # Disk Info
  if command -v df &> /dev/null; then
    DISK_FREE=$(df -h / | awk 'NR==2 {print $4}')
    echo -e "  💾 ${CYAN}Boş Disk Alanı (Root):${NC} $DISK_FREE"
  else
    echo -e "  💾 ${CYAN}Boş Disk Alanı (Root):${NC} Bilinmiyor"
  fi

  # Public IP Info
  PUBLIC_IP=$(curl -s --max-time 3 https://ipinfo.io/ip || echo "Bilinmiyor")
  echo -e "  🆔 ${CYAN}Sunucu IP Adresi:${NC} $PUBLIC_IP"
  echo ""
}

# Adım Çalıştırma Fonksiyonu
run_step() {
  local msg=$1
  shift
  echo -ne "  ⚙️  $msg... "
  if "$@" >> "$LOG_FILE" 2>&1; then
    echo -e "\r\033[K  ✔️  ${GREEN}$msg${NC}"
  else
    echo -e "\r\033[K  ❌  ${RED}$msg BAŞARISIZ OLDU!${NC}"
    echo -e "      ${YELLOW}HATA DETAYI:${NC} Son 10 satır:"
    tail -n 10 "$LOG_FILE" | sed 's/^/      /'
    echo -e "      ${YELLOW}Tüm kurulum günlükleri için inceleyin: ${BOLD}$LOG_FILE${NC}"
    exit 1
  fi
}

# --- AKIŞ BAŞLANGICI ---
clear
print_banner
print_system_diagnostics

echo -e "🚀 ${BOLD}Kurulum Başlatılıyor...${NC}"

# 2. Bağımlılıkların Kurulumu
run_step "Sistem paket listesi güncelleniyor (apt update)" apt-get update
run_step "Temel sistem bağımlılıkları yükleniyor" apt-get install -y nginx certbot python3-certbot-nginx git curl unzip snapd

# Go Kurulumu
if ! command -v go &> /dev/null; then
  run_step "Go programlama dili snap ile kuruluyor" snap install go --classic
else
  echo -e "  ✔️  ${GREEN}Go programlama dili zaten kurulu${NC} ($(go version | awk '{print $3}'))"
fi

# Node.js Kurulumu
if ! command -v node &> /dev/null; then
  run_step "NodeSource Node.js 20.x deposu ekleniyor" bash -c "curl -fsSL https://deb.nodesource.com/setup_20.x | bash -"
  run_step "Node.js paketleri kuruluyor" apt-get install -y nodejs
else
  echo -e "  ✔️  ${GREEN}Node.js zaten kurulu${NC} ($(node -v))"
fi

# 3. Projenin Klonlanması veya Güncellenmesi
if [ ! -d "/root/nazploy-src" ]; then
  run_step "Proje deposu GitHub'dan klonlanıyor" git clone https://github.com/nazeg/nazploy.git /root/nazploy-src
  cd /root/nazploy-src
else
  cd /root/nazploy-src
  run_step "Proje güncelleniyor (git pull)" bash -c "git reset --hard && git pull"
fi

# 4. Klasör Yapısının Hazırlanması
run_step "Gerekli dizin yapısı hazırlanıyor" mkdir -p /var/lib/dashboard/databases /var/www /root/nazploy

# Resmi PocketBase v0.30.2 temiz binary indirme (alt database örnekleri için)
if [ ! -f "/root/nazploy/pocketbase_bin" ]; then
  run_step "PocketBase resmi binary indiriliyor" bash -c "curl -L -o /tmp/pb.zip https://github.com/pocketbase/pocketbase/releases/download/v0.30.2/pocketbase_0.30.2_linux_amd64.zip && unzip -o /tmp/pb.zip pocketbase -d /tmp/ && mv /tmp/pocketbase /root/nazploy/pocketbase_bin && chmod +x /root/nazploy/pocketbase_bin && rm -f /tmp/pb.zip"
else
  echo -e "  ✔️  ${GREEN}PocketBase şablon binary zaten mevcut${NC}"
fi

# 5. Frontend Derleme (Build)
cd web
run_step "Frontend bağımlılıkları temizlenip yükleniyor" bash -c "rm -rf node_modules && npm install --unsafe-perm=true --include=dev"
run_step "Frontend derleniyor (Vite build)" npm run build --unsafe-perm=true
cd ..

# 6. Backend Derleme (Build)
GO_BIN=$(command -v go || echo "/snap/bin/go")
run_step "Backend Go uygulaması derleniyor" "$GO_BIN" build -o /root/nazploy/nazploy .

# 8. Systemd Servisi Oluşturma
create_systemd_service() {
  DEPLOY_USER=${SUDO_USER:-root}
  
  # Eski dashboard servisini durdur ve sil (varsa geçiş için)
  if systemctl is-active --quiet dashboard; then
    systemctl stop dashboard &>/dev/null || true
    systemctl disable dashboard &>/dev/null || true
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
}

run_step "Systemd servis yapılandırması oluşturuluyor" create_systemd_service

# 9. Servisleri Başlatma
run_step "Servisler etkinleştiriliyor ve başlatılıyor" bash -c "systemctl daemon-reload && systemctl enable nazploy && systemctl restart nazploy"

# 10. Bitiş Ekranı
PUBLIC_IP=$(curl -s --max-time 3 https://ipinfo.io/ip || echo "SUNUCU_IP")
echo ""
echo -e "${GREEN}${BOLD}========================================================${NC}"
echo -e "🎉 ${GREEN}${BOLD}KURULUM BAŞARIYLA TAMAMLANDI!${NC}"
echo -e "🚀 Nazploy başarıyla kuruldu ve arka planda başlatıldı."
echo -e "🌐 Yönetim Paneli Adresi: ${CYAN}${BOLD}http://${PUBLIC_IP}:8090${NC}"
echo -e "${GREEN}${BOLD}========================================================${NC}"
echo ""
echo -e "${YELLOW}${BOLD}Servis Durumu (journalctl):${NC}"
journalctl -u nazploy -n 10 --no-pager | sed 's/^/  /'
echo ""
