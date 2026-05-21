#!/bin/bash
set -e

# ─── Colors ───
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ─── Config ───
PROJECT_DIR="/root/nazploy-src"
DEPLOY_DIR="/root/nazploy"
BINARY_NAME="nazploy"
SERVICE_NAME="nazploy"

# ─── Usage ───
usage() {
  echo -e "${BOLD}Nazploy Hızlı Derleme & Deploy${NC}"
  echo ""
  echo -e "Kullanım: ${CYAN}sudo ./dev-deploy.sh [seçenek]${NC}"
  echo ""
  echo -e "  ${GREEN}--all${NC}        Git pull + Frontend + Backend derle, servisi yeniden başlat (varsayılan)"
  echo -e "  ${GREEN}--local${NC}      Sadece yerel kodları derle, servisi yeniden başlat (git pull yok)"
  echo -e "  ${GREEN}--frontend${NC}   Sadece frontend derle, servisi yeniden başlat"
  echo -e "  ${GREEN}--backend${NC}    Sadece backend derle, servisi yeniden başlat"
  echo -e "  ${GREEN}--restart${NC}    Sadece servisi yeniden başlat (derleme yok)"
  echo -e "  ${GREEN}--status${NC}     Servis durumunu göster"
  echo -e "  ${GREEN}--logs${NC}       Son logları göster"
  echo -e "  ${GREEN}--help${NC}       Bu yardım mesajı"
  echo ""
  exit 0
}

# ─── Root check ───
if [ "$EUID" -ne 0 ]; then
  echo -e "❌ ${RED}Root yetkisi gerekli.${NC} → ${CYAN}sudo ./dev-deploy.sh${NC}"
  exit 1
fi

# ─── Timer ───
SECONDS=0

# ─── Step runner ───
step() {
  local msg=$1; shift
  echo -ne "  ⚙️  $msg... "
  if "$@" > /dev/null 2>&1; then
    echo -e "\r\033[K  ✅ ${GREEN}$msg${NC}"
  else
    echo -e "\r\033[K  ❌ ${RED}$msg BAŞARISIZ${NC}"
    exit 1
  fi
}

# ─── Actions ───
do_git_pull() {
  cd "$PROJECT_DIR"
  step "Güncel kod çekiliyor (git pull)" bash -c "git reset --hard && git pull"
}

do_frontend() {
  echo -e "\n📦 ${BOLD}Frontend Derleniyor...${NC}"
  cd "$PROJECT_DIR/web"
  
  # node_modules yoksa veya package.json değiştiyse install yap
  if [ ! -d "node_modules" ] || [ "package.json" -nt "node_modules/.package-lock.json" ]; then
    step "npm install" npm install --prefer-offline --no-audit --no-fund
  else
    echo -e "  ⏭️  ${YELLOW}node_modules güncel, install atlanıyor${NC}"
  fi
  
  step "Vite build" npm run build
  cd "$PROJECT_DIR"
}

do_backend() {
  echo -e "\n🔨 ${BOLD}Backend Derleniyor...${NC}"
  cd "$PROJECT_DIR"
  export GOTOOLCHAIN=local
  export GOFLAGS="-trimpath"
  step "Go build" /usr/local/go/bin/go build -o "$DEPLOY_DIR/$BINARY_NAME" .
}

do_restart() {
  echo -e "\n🔄 ${BOLD}Servis Yeniden Başlatılıyor...${NC}"
  step "systemctl restart $SERVICE_NAME" systemctl restart "$SERVICE_NAME"
  
  # Kısa bekleme sonrası durum kontrolü
  sleep 1
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    echo -e "  🟢 ${GREEN}${BOLD}$SERVICE_NAME aktif ve çalışıyor${NC}"
  else
    echo -e "  🔴 ${RED}${BOLD}Servis başlatılamadı!${NC}"
    journalctl -u "$SERVICE_NAME" -n 15 --no-pager
    exit 1
  fi
}

do_status() {
  echo -e "\n📊 ${BOLD}Servis Durumu:${NC}"
  systemctl status "$SERVICE_NAME" --no-pager -l
  exit 0
}

do_logs() {
  echo -e "\n📋 ${BOLD}Son Loglar:${NC}"
  journalctl -u "$SERVICE_NAME" -n 50 --no-pager -f
  exit 0
}

# ─── Parse args ───
MODE="${1:---all}"

case "$MODE" in
  --help|-h)
    usage
    ;;
  --status)
    do_status
    ;;
  --logs)
    do_logs
    ;;
  --frontend|-f)
    do_frontend
    do_restart
    ;;
  --backend|-b)
    do_backend
    do_restart
    ;;
  --restart|-r)
    do_restart
    ;;
  --pull|-p|--all|-a|"")
    do_git_pull
    do_frontend
    do_backend
    do_restart
    ;;
  --local|-l)
    do_frontend
    do_backend
    do_restart
    ;;
  *)
    echo -e "❌ ${RED}Bilinmeyen seçenek: $MODE${NC}"
    usage
    ;;
esac

# ─── Summary ───
echo ""
echo -e "⏱️  ${CYAN}Toplam süre: ${BOLD}${SECONDS}s${NC}"
echo -e "🌐 ${CYAN}Panel: ${BOLD}http://$(hostname -I 2>/dev/null | awk '{print $1}'):8090${NC}"
echo ""
