#!/bin/bash
# LogMonitor Deployment Script
# Run with sudo

set -e

INSTALL_DIR="/opt/logmonitor"
DATA_DIR="/opt/logmonitor/data"
USER="logmonitor"
SERVICE_NAME="logmonitor-collector"

echo "=== LogMonitor Deployment Script ==="

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (sudo)"
    exit 1
fi

# Create user
echo "Creating user '$USER'..."
if ! id "$USER" &>/dev/null; then
    useradd -r -s /bin/false -d "$INSTALL_DIR" "$USER"
fi

# Create directories
echo "Creating directories..."
mkdir -p "$INSTALL_DIR/collector"
mkdir -p "$DATA_DIR"
mkdir -p "$INSTALL_DIR/dashboard/dist"

# Copy files
echo "Copying collector binary..."
if [ -f "./collector/logmonitor-collector" ]; then
    cp ./collector/logmonitor-collector "$INSTALL_DIR/collector/"
    chmod +x "$INSTALL_DIR/collector/logmonitor-collector"
else
    echo "Warning: Collector binary not found. Please build it first:"
    echo "  cd collector && go build -o logmonitor-collector"
fi

echo "Copying config..."
if [ -f "./collector/config.yaml" ]; then
    cp ./collector/config.yaml "$INSTALL_DIR/collector/"
else
    echo "Creating default config..."
    cat > "$INSTALL_DIR/collector/config.yaml" << EOF
server:
  port: 9200
  cors: true

database:
  path: $DATA_DIR/logmonitor.db
  retention_days: 30

buffer:
  size: 10000
  flush_interval_ms: 2000
  flush_batch_size: 500

alert:
  check_interval_ms: 60000
EOF
fi

echo "Copying dashboard files..."
if [ -d "./dashboard/dist" ]; then
    cp -r ./dashboard/dist/* "$INSTALL_DIR/dashboard/dist/"
else
    echo "Warning: Dashboard build not found. Please build it first:"
    echo "  cd dashboard && npm install && npm run build"
fi

# Set permissions
echo "Setting permissions..."
chown -R "$USER:$USER" "$INSTALL_DIR"
chown -R "$USER:$USER" "$DATA_DIR"
chmod 755 "$INSTALL_DIR/collector"
chmod 644 "$INSTALL_DIR/collector/config.yaml"

# Install systemd service
echo "Installing systemd service..."
cp ./deploy/logmonitor-collector.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"

echo ""
echo "=== Installation Complete ==="
echo ""
echo "Next steps:"
echo "1. Start the service: systemctl start $SERVICE_NAME"
echo "2. Check status: systemctl status $SERVICE_NAME"
echo "3. View logs: journalctl -u $SERVICE_NAME -f"
echo "4. Configure nginx: cp ./deploy/logmonitor.conf /etc/nginx/sites-available/logmonitor"
echo "5. Enable nginx site: ln -s /etc/nginx/sites-available/logmonitor /etc/nginx/sites-enabled/"
echo "6. Reload nginx: systemctl reload nginx"
echo ""
echo "The dashboard will be available at http://your-server-ip"
echo "The API will be available at http://your-server-ip/api/"
