#!/usr/bin/env bash
set -euo pipefail

TOKEN=""
API_URL=""
BINARY_URL="https://github.com/andre-valadares/deskpilot-agent/releases/latest/download"

usage() {
  echo "Uso: bash install.sh --token=<token> --api=<url>"
  exit 1
}

for arg in "$@"; do
  case $arg in
    --token=*) TOKEN="${arg#*=}" ;;
    --api=*)   API_URL="${arg#*=}" ;;
    *) usage ;;
  esac
done

[[ -z "$TOKEN" || -z "$API_URL" ]] && usage

OS="$(uname -s)"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Arquitetura $ARCH não suportada"; exit 1 ;;
esac

case "$OS" in
  Linux)  PLATFORM="linux" ;;
  Darwin) PLATFORM="darwin" ;;
  *)      echo "OS $OS não suportado; use install.ps1 no Windows"; exit 1 ;;
esac

INSTALL_DIR="/usr/local/bin"
BINARY="$INSTALL_DIR/deskpilot-agent"

echo "Instalando DeskPilot Agent ($PLATFORM/$ARCH)..."

# Download do binário pré-compilado (substituir URL quando releases estiverem disponíveis)
if [[ -n "$BINARY_URL" ]]; then
  curl -fsSL "$BINARY_URL/deskpilot-agent-${PLATFORM}-${ARCH}" -o "$BINARY"
else
  # Compilar da fonte como fallback
  if ! command -v go &>/dev/null; then
    echo "Go não encontrado. Instale em https://go.dev/dl/ ou aguarde os binários pré-compilados."
    exit 1
  fi
  TMPDIR="$(mktemp -d)"
  trap "rm -rf $TMPDIR" EXIT
  cp -r "$(dirname "$0")" "$TMPDIR/agent"
  (cd "$TMPDIR/agent" && go build -o "$BINARY" .)
fi

chmod +x "$BINARY"

# Salvar configuração
"$BINARY" --token="$TOKEN" --api="$API_URL" --install

# Regra de firewall — permite receber WoL (UDP porta 9)
configure_firewall() {
  if [[ "$OS" == "Darwin" ]]; then
    local fw="/usr/libexec/ApplicationFirewall/socketfilterfw"
    if $fw --getglobalstate 2>/dev/null | grep -q "enabled"; then
      sudo "$fw" --add "$BINARY" 2>/dev/null || true
      sudo "$fw" --unblockapp "$BINARY" 2>/dev/null || true
      echo "Firewall macOS: exceção adicionada para o agente."
    fi
  else
    if command -v ufw &>/dev/null && ufw status 2>/dev/null | grep -q "active"; then
      sudo ufw allow 9/udp comment "DeskPilot Agent" >/dev/null
      echo "ufw: porta 9/udp liberada."
    elif command -v firewall-cmd &>/dev/null && firewall-cmd --state 2>/dev/null | grep -q "running"; then
      sudo firewall-cmd --permanent --add-port=9/udp >/dev/null
      sudo firewall-cmd --reload >/dev/null
      echo "firewalld: porta 9/udp liberada."
    elif command -v iptables &>/dev/null; then
      sudo iptables -C INPUT -p udp --dport 9 -j ACCEPT 2>/dev/null || \
        sudo iptables -I INPUT -p udp --dport 9 -j ACCEPT
      echo "iptables: porta 9/udp liberada."
    else
      echo "Aviso: firewall não detectado. Verifique manualmente se UDP porta 9 está liberada."
    fi
  fi
}
configure_firewall

# Registrar como serviço
if [[ "$OS" == "Darwin" ]]; then
  PLIST="$HOME/Library/LaunchAgents/com.deskpilot.agent.plist"
  cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.deskpilot.agent</string>
  <key>ProgramArguments</key>
  <array>
    <string>$BINARY</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>$HOME/.deskpilot/agent.log</string>
  <key>StandardErrorPath</key>
  <string>$HOME/.deskpilot/agent.log</string>
</dict>
</plist>
EOF
  launchctl load "$PLIST"
  echo "Serviço registrado como LaunchAgent (macOS). Iniciado automaticamente no login."

else
  # Linux — systemd
  SERVICE_FILE="/etc/systemd/system/deskpilot-agent.service"
  sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=DeskPilot Agent
After=network.target

[Service]
ExecStart=$BINARY
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  sudo systemctl daemon-reload
  sudo systemctl enable --now deskpilot-agent
  echo "Serviço registrado via systemd. Executando agora."
fi

echo "DeskPilot Agent instalado com sucesso."
