#!/bin/bash
set -e

# curl | bash ile çalıştırıldığında root değilse sudo ile yeniden başlat
if [ "$EUID" -ne 0 ]; then
  echo "🔐 Root yetkisi gerekiyor, sudo ile yeniden başlatılıyor..."
  SELF="$(mktemp /tmp/nazploy_setup.XXXXXX.sh)"
  if [ -f "$0" ] && [ "$0" != "bash" ] && [ "$0" != "/bin/bash" ]; then
    cp "$0" "$SELF"
  else
    # pipe ile geldi, kendini /proc/self/fd/0 üzerinden oku
    cat /proc/$$/fd/255 > "$SELF" 2>/dev/null || cat "$0" > "$SELF" 2>/dev/null || true
  fi
  chmod +x "$SELF"
  exec sudo bash "$SELF" "$@"
fi


# Global temizlik: beklenmedik çıkışlarda spinner'ı öldür, terminali düzelt
_SPINNER_PID=""
cleanup() {
  if [ -n "$_SPINNER_PID" ] && kill -0 "$_SPINNER_PID" 2>/dev/null; then
    kill "$_SPINNER_PID" 2>/dev/null
    wait "$_SPINNER_PID" 2>/dev/null || true
  fi
  tput cnorm 2>/dev/null || true  # cursor'ı geri getir
  echo ""
}
trap cleanup EXIT

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
  cat <<'LOGO'
  ███╗   ██╗ █████╗ ███████╗██████╗ ██╗      ██████╗ ██╗   ██╗
  ████╗  ██║██╔══██╗╚══███╔╝██╔══██╗██║     ██╔═══██╗╚██╗ ██╔╝
  ██╔██╗ ██║███████║  ███╔╝ ██████╔╝██║     ██║   ██║ ╚████╔╝
  ██║╚██╗██║██╔══██║ ███╔╝  ██╔═══╝ ██║     ██║   ██║  ╚██╔╝
  ██║ ╚████║██║  ██║███████╗██║     ███████╗╚██████╔╝   ██║
  ╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝╚═╝     ╚══════╝ ╚═════╝    ╚═╝
LOGO
  echo -e "${NC}"
  echo -e "${BOLD}--- Nazploy Otomatik Kurulum Sihirbazı ---${NC}"
  echo -e "Bu betik gerekli bağımlılıkları kuracak, projeyi derleyecek ve servisi başlatacaktır."
  echo ""
}

# Sistem Teşhis Bilgileri
print_system_diagnostics() {
  echo -e "🔍 ${BOLD}Sunucu Sistem Bilgileri:${NC}"
  
  if [ -f /etc/os-release ]; then
    OS_NAME=$(grep -w "PRETTY_NAME" /etc/os-release | cut -d= -f2 | tr -d '"')
    echo -e "  🌐 ${CYAN}İşletim Sistemi:${NC} $OS_NAME"
  else
    echo -e "  🌐 ${CYAN}İşletim Sistemi:${NC} Bilinmiyor"
  fi

  if command -v nproc &> /dev/null; then
    CPU_CORES=$(nproc)
    echo -e "  🧠 ${CYAN}CPU Çekirdek Sayısı:${NC} $CPU_CORES"
  else
    echo -e "  🧠 ${CYAN}CPU Çekirdek Sayısı:${NC} Bilinmiyor"
  fi

  if command -v free &> /dev/null; then
    RAM_TOTAL=$(free -h | awk '/^Mem:/{print $2}')
    echo -e "  ⚡ ${CYAN}Toplam RAM:${NC} $RAM_TOTAL"
  elif [ -f /proc/meminfo ]; then
    RAM_TOTAL=$(grep MemTotal /proc/meminfo | awk '{print $2/1024 " MB"}')
    echo -e "  ⚡ ${CYAN}Toplam RAM:${NC} $RAM_TOTAL"
  else
    echo -e "  ⚡ ${CYAN}Toplam RAM:${NC} Bilinmiyor"
  fi

  if command -v df &> /dev/null; then
    DISK_FREE=$(df -h / | awk 'NR==2 {print $4}')
    echo -e "  💾 ${CYAN}Boş Disk Alanı (Root):${NC} $DISK_FREE"
  else
    echo -e "  💾 ${CYAN}Boş Disk Alanı (Root):${NC} Bilinmiyor"
  fi

  LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")
  echo -e "  🆔 ${CYAN}Sunucu IP Adresi:${NC} $LOCAL_IP"
  echo ""
}

# Spinner Animasyonu ile Adım Çalıştırma Fonksiyonu
run_step() {
  local msg=$1
  shift

  local spin_chars='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
  local spin_i=0

  tput civis 2>/dev/null || true  # cursor'ı gizle

  # Spinner arka planda döner; PID'i global değişkene yaz
  (
    while true; do
      local c="${spin_chars:$spin_i:1}"
      echo -ne "\r\033[K  ${CYAN}${c}${NC}  $msg... "
      spin_i=$(( (spin_i + 1) % ${#spin_chars} ))
      sleep 0.1
    done
  ) &
  _SPINNER_PID=$!

  # Asıl komutu çalıştır
  local exit_code=0
  "$@" >> "$LOG_FILE" 2>&1 || exit_code=$?

  # Spinner'ı durdur
  kill "$_SPINNER_PID" 2>/dev/null
  wait "$_SPINNER_PID" 2>/dev/null || true
  _SPINNER_PID=""

  tput cnorm 2>/dev/null || true  # cursor'ı geri getir

  if [ "$exit_code" -eq 0 ]; then
    echo -e "\r\033[K  ✔️  ${GREEN}$msg${NC}"
  else
    echo -e "\r\033[K  ❌  ${RED}$msg BAŞARISIZ OLDU!${NC}"
    echo -e "      ${YELLOW}HATA DETAYI:${NC} Son 10 satır:"
    tail -n 10 "$LOG_FILE" | sed 's/^/      /'
    echo -e "      ${YELLOW}Tüm kurulum günlükleri için inceleyin: ${BOLD}$LOG_FILE${NC}"
    exit 1
  fi
}

# Sürüm karşılaştırma yardımcısı
# Kullanım: version_gte "1.21.0" "1.20.5"  → 0 (true) eğer ilk >= ikinci
version_gte() {
  [ "$(printf '%s\n' "$1" "$2" | sort -V | head -1)" = "$2" ]
}

# Dinamik Sürüm Tespit Fonksiyonları

get_latest_go_version() {
  local ver
  # \r karakterini temizle (bazı sistemlerde curl çıktısında bulunabilir)
  ver=$(curl -sSL --max-time 10 "https://go.dev/VERSION?m=text" 2>/dev/null \
    | head -1 \
    | tr -d '\r' \
    | sed 's/^go//')
  if [ -z "$ver" ] || ! echo "$ver" | grep -qE '^[0-9]+\.[0-9]+(\.[0-9]+)?$'; then
    echo "1.24.3" # fallback — güncel tutun
  else
    echo "$ver"
  fi
}

get_latest_node_lts_major() {
  local major
  major=$(curl -sSL --max-time 10 "https://nodejs.org/dist/index.json" 2>/dev/null | \
    python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    for v in data:
        if v.get('lts'):
            print(v['version'].lstrip('v').split('.')[0])
            break
except Exception:
    pass
" 2>/dev/null)
  if [ -z "$major" ] || ! echo "$major" | grep -qE '^[0-9]+$'; then
    echo "22" # fallback — güncel tutun
  else
    echo "$major"
  fi
}

get_latest_node_version() {
  local ver
  ver=$(curl -sSL --max-time 10 "https://nodejs.org/dist/index.json" 2>/dev/null | \
    python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    for v in data:
        if v.get('lts'):
            print(v['version'].lstrip('v'))
            break
except Exception:
    pass
" 2>/dev/null)
  if [ -z "$ver" ] || ! echo "$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "22.0.0" # fallback
  else
    echo "$ver"
  fi
}

get_latest_pocketbase_version() {
  local ver
  ver=$(curl -sSL --max-time 10 "https://api.github.com/repos/pocketbase/pocketbase/releases/latest" 2>/dev/null | \
    grep -oP '"tag_name":\s*"v\K[0-9]+\.[0-9]+\.[0-9]+' | head -1)
  if [ -z "$ver" ] || ! echo "$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "0.28.2" # fallback — güncel tutun
  else
    echo "$ver"
  fi
}

install_go() {
  local GO_VERSION
  GO_VERSION=$(get_latest_go_version)
  echo "  → Go $GO_VERSION indiriliyor..." >> "$LOG_FILE"

  snap remove go 2>/dev/null || true
  DEBIAN_FRONTEND=noninteractive apt-get remove -y golang-go 2>/dev/null || true
  rm -rf /usr/local/go
  rm -f /usr/bin/go /usr/local/bin/go /usr/bin/gofmt /usr/local/bin/gofmt

  curl -L --max-time 120 -o /tmp/go.tar.gz "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz"
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm -f /tmp/go.tar.gz
  ln -sf /usr/local/go/bin/go /usr/bin/go
  ln -sf /usr/local/go/bin/gofmt /usr/bin/gofmt
  hash -r 2>/dev/null || true
}

# -- AKIŞ BAŞLANGICI --
clear
print_banner
print_system_diagnostics

echo -e "📦 ${BOLD}Sürüm Tespiti Yapılıyor...${NC}"
DETECTED_GO=$(get_latest_go_version)
DETECTED_NODE=$(get_latest_node_lts_major)
DETECTED_NODE_VER=$(get_latest_node_version)
DETECTED_PB=$(get_latest_pocketbase_version)
echo -e "  📌 ${CYAN}Go:${NC} v${DETECTED_GO}  ${CYAN}Node.js LTS:${NC} v${DETECTED_NODE}.x  ${CYAN}PocketBase:${NC} v${DETECTED_PB}"
echo ""

echo -e "🚀 ${BOLD}Kurulum Başlatılıyor...${NC}"

# 2. Bağımlılıkların Kurulumu
run_step "Sistem paket listesi güncelleniyor (apt update)" apt-get update
run_step "Temel sistem bağımlılıkları yükleniyor" bash -c "DEBIAN_FRONTEND=noninteractive apt-get install -y nginx certbot python3-certbot-nginx git curl unzip"

# Node.js Kurulumu — sürüm kontrolü eklendi
INSTALLED_NODE_VER=""
if command -v node &> /dev/null; then
  INSTALLED_NODE_VER=$(node -v 2>/dev/null | tr -d '\r' | sed 's/^v//')
fi

if [ -z "$INSTALLED_NODE_VER" ]; then
  NODE_MAJOR=$(get_latest_node_lts_major)
  run_step "NodeSource Node.js ${NODE_MAJOR}.x deposu ekleniyor" bash -c "curl -fsSL https://deb.nodesource.com/setup_${NODE_MAJOR}.x | bash -"
  run_step "Node.js paketleri kuruluyor" bash -c "DEBIAN_FRONTEND=noninteractive apt-get install -y --allow-downgrades --allow-change-held-packages nodejs"
elif ! version_gte "$INSTALLED_NODE_VER" "$DETECTED_NODE_VER"; then
  echo -e "  ⚠️  ${YELLOW}Kurulu Node.js (v${INSTALLED_NODE_VER}) yetersiz, v${DETECTED_NODE_VER} gerekiyor. Güncelleniyor...${NC}"
  NODE_MAJOR=$(get_latest_node_lts_major)
  run_step "NodeSource Node.js ${NODE_MAJOR}.x deposu güncelleniyor" bash -c "curl -fsSL https://deb.nodesource.com/setup_${NODE_MAJOR}.x | bash -"
  run_step "Node.js güncelleniyor" bash -c "DEBIAN_FRONTEND=noninteractive apt-get install -y --allow-downgrades --allow-change-held-packages nodejs"
else
  echo -e "  ✔️  ${GREEN}Node.js zaten güncel${NC} (v${INSTALLED_NODE_VER})"
fi

# 3. Projenin Klonlanması veya Güncellenmesi (Go kontrolünden ÖNCE)
if [ ! -d "/root/nazploy-src" ]; then
  run_step "Proje deposu GitHub'dan klonlanıyor" git clone https://github.com/nazeg/nazploy.git /root/nazploy-src
else
  cd /root/nazploy-src
  run_step "Proje güncelleniyor (git pull)" bash -c "git reset --hard && git pull"
fi
cd /root/nazploy-src

# Go Kurulumu — go.mod'dan gereken sürümü oku, karşılaştır
REQUIRED_GO=$(sed -n 's/^go \([0-9][0-9.]*\).*/\1/p' /root/nazploy-src/go.mod | head -1 | tr -d '\r')

INSTALLED_GO=""
if [ -x /usr/local/go/bin/go ]; then
  INSTALLED_GO=$(/usr/local/go/bin/go version 2>/dev/null \
    | sed -n 's/.*go\([0-9][0-9.]*\).*/\1/p' \
    | head -1 \
    | tr -d '\r')
fi

if [ -z "$INSTALLED_GO" ]; then
  run_step "Go programlama dili kuruluyor (v${DETECTED_GO})" install_go
elif [ -n "$REQUIRED_GO" ] && ! version_gte "$INSTALLED_GO" "$REQUIRED_GO"; then
  echo -e "  ⚠️  ${YELLOW}Kurulu Go (v${INSTALLED_GO}) yetersiz, go.mod v${REQUIRED_GO}+ gerektiriyor. Güncelleniyor...${NC}"
  run_step "Go programlama dili güncelleniyor (v${DETECTED_GO})" install_go
else
  echo -e "  ✔️  ${GREEN}Go programlama dili zaten güncel${NC} (v${INSTALLED_GO})"
fi

# 4. Klasör Yapısının Hazırlanması
run_step "Gerekli dizin yapısı hazırlanıyor" mkdir -p /var/lib/dashboard/databases /var/www /root/nazploy

# PocketBase binary indirme (alt database örnekleri için)
if [ ! -f "/root/nazploy/pocketbase_bin" ]; then
  PB_VERSION=$(get_latest_pocketbase_version)
  run_step "PocketBase v${PB_VERSION} resmi binary indiriliyor" bash -c "
    curl -L --max-time 120 -o /tmp/pb.zip \
      https://github.com/pocketbase/pocketbase/releases/download/v${PB_VERSION}/pocketbase_${PB_VERSION}_linux_amd64.zip && \
    unzip -o /tmp/pb.zip pocketbase -d /tmp/ && \
    mv /tmp/pocketbase /root/nazploy/pocketbase_bin && \
    chmod +x /root/nazploy/pocketbase_bin && \
    rm -f /tmp/pb.zip"
else
  echo -e "  ✔️  ${GREEN}PocketBase şablon binary zaten mevcut${NC}"
fi

# 5. Frontend Derleme (Build)
cd web
run_step "Frontend bağımlılıkları temizlenip yükleniyor" bash -c "rm -rf node_modules && npm install --unsafe-perm=true --include=dev"
run_step "Frontend derleniyor (Vite build)" npm run build --unsafe-perm=true
cd ..

# 6. Disk alanı temizliği
run_step "Geçici dosyalar temizleniyor (disk alanı açılıyor)" bash -c "rm -rf /root/nazploy-src/web/node_modules && apt-get clean && rm -rf /var/lib/apt/lists/*"

# 7. Backend Derleme (Build)
export GOTOOLCHAIN=local
export GOFLAGS="-trimpath"
run_step "Backend Go uygulaması derleniyor" /usr/local/go/bin/go build -o /root/nazploy/nazploy .

# 8. Systemd Servisi Oluşturma
create_systemd_service() {
  DEPLOY_USER=${SUDO_USER:-root}
  
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
LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "SUNUCU_IP")

SETUP_LINK=""
for i in $(seq 1 15); do
  sleep 1
  SETUP_LINK=$(journalctl -u nazploy --since "1 minute ago" --no-pager --full 2>/dev/null \
    | grep -oE 'http://[^[:space:]]*(pbinstall|setup)[^[:space:]]*' \
    | tail -1)
  [ -n "$SETUP_LINK" ] && break
done

echo ""
echo -e "${GREEN}${BOLD}========================================================${NC}"
echo -e "🎉 ${GREEN}${BOLD}KURULUM BAŞARIYLA TAMAMLANDI!${NC}"
echo -e "🌐 Yönetim Paneli: ${CYAN}${BOLD}http://${LOCAL_IP}:8090${NC}"
echo -e "${GREEN}${BOLD}========================================================${NC}"
echo ""

if [ -n "$SETUP_LINK" ]; then
  SETUP_LINK=${SETUP_LINK//0.0.0.0/$LOCAL_IP}
  echo -e "🔑 ${YELLOW}${BOLD}İLK KURULUM:${NC} Admin hesabı oluşturmak için bu linki tarayıcında aç:"
  echo -e "   ${CYAN}${BOLD}${SETUP_LINK}${NC}"
else
  echo -e "⚠️  ${YELLOW}Setup linki alınamadı. Manuel kontrol:${NC}"
  echo -e "   journalctl -u nazploy -n 30 --no-pager"
fi
echo ""