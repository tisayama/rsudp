#!/bin/bash

# GoRSUDP systemd service installation script
# This script sets up GoRSUDP as a systemd service

set -euo pipefail

# Configuration
GORSUDP_USER="gorsudp"
GORSUDP_GROUP="gorsudp"
GORSUDP_HOME="/var/lib/gorsudp"
GORSUDP_CONFIG_DIR="/etc/gorsudp"
GORSUDP_LOG_DIR="/var/log/gorsudp"
BINARY_PATH="/usr/local/bin/gorsudp"
SERVICE_FILE="/etc/systemd/system/gorsudp.service"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

# Create user and group
create_user() {
    log_info "Creating user and group: $GORSUDP_USER"
    
    if ! getent group "$GORSUDP_GROUP" > /dev/null 2>&1; then
        groupadd --system "$GORSUDP_GROUP"
        log_info "Created group: $GORSUDP_GROUP"
    else
        log_info "Group $GORSUDP_GROUP already exists"
    fi
    
    if ! getent passwd "$GORSUDP_USER" > /dev/null 2>&1; then
        useradd --system \
                --gid "$GORSUDP_GROUP" \
                --create-home \
                --home-dir "$GORSUDP_HOME" \
                --shell /bin/false \
                --comment "GoRSUDP service user" \
                "$GORSUDP_USER"
        log_info "Created user: $GORSUDP_USER"
    else
        log_info "User $GORSUDP_USER already exists"
    fi
}

# Create directories
create_directories() {
    log_info "Creating directories"
    
    directories=(
        "$GORSUDP_HOME"
        "$GORSUDP_HOME/data"
        "$GORSUDP_HOME/screenshots"
        "$GORSUDP_CONFIG_DIR"
        "$GORSUDP_LOG_DIR"
    )
    
    for dir in "${directories[@]}"; do
        if [[ ! -d "$dir" ]]; then
            mkdir -p "$dir"
            log_info "Created directory: $dir"
        fi
    done
    
    # Set ownership
    chown -R "$GORSUDP_USER:$GORSUDP_GROUP" "$GORSUDP_HOME"
    chown -R "$GORSUDP_USER:$GORSUDP_GROUP" "$GORSUDP_LOG_DIR"
    chown root:root "$GORSUDP_CONFIG_DIR"
    chmod 755 "$GORSUDP_CONFIG_DIR"
    
    log_info "Set directory permissions"
}

# Install binary
install_binary() {
    local binary_source="${1:-./gorsudp}"
    
    if [[ ! -f "$binary_source" ]]; then
        log_error "Binary not found: $binary_source"
        log_error "Please build the binary first: go build -o gorsudp ./cmd/gorsudp"
        exit 1
    fi
    
    log_info "Installing binary to $BINARY_PATH"
    cp "$binary_source" "$BINARY_PATH"
    chmod +x "$BINARY_PATH"
    chown root:root "$BINARY_PATH"
}

# Install service file
install_service() {
    local service_source="${1:-./gorsudp.service}"
    
    if [[ ! -f "$service_source" ]]; then
        log_error "Service file not found: $service_source"
        exit 1
    fi
    
    log_info "Installing systemd service file"
    cp "$service_source" "$SERVICE_FILE"
    chmod 644 "$SERVICE_FILE"
    chown root:root "$SERVICE_FILE"
}

# Create default configuration
create_default_config() {
    local config_file="$GORSUDP_CONFIG_DIR/config.json"
    
    if [[ ! -f "$config_file" ]]; then
        log_info "Creating default configuration"
        
        "$BINARY_PATH" --generate-config --config "$config_file"
        chown root:"$GORSUDP_GROUP" "$config_file"
        chmod 640 "$config_file"
        
        log_info "Default configuration created at: $config_file"
        log_warn "Please edit the configuration file before starting the service"
    else
        log_info "Configuration file already exists: $config_file"
    fi
}

# Setup log rotation
setup_logrotate() {
    log_info "Setting up log rotation"
    
    cat > /etc/logrotate.d/gorsudp << 'EOF'
/var/log/gorsudp/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0644 gorsudp gorsudp
    postrotate
        /bin/systemctl reload gorsudp.service > /dev/null 2>&1 || true
    endscript
}
EOF
    
    log_info "Log rotation configured"
}

# Reload systemd and enable service
setup_systemd() {
    log_info "Reloading systemd daemon"
    systemctl daemon-reload
    
    log_info "Enabling GoRSUDP service"
    systemctl enable gorsudp.service
    
    log_info "Service enabled. Use 'systemctl start gorsudp' to start"
}

# Validate installation
validate_installation() {
    log_info "Validating installation"
    
    # Check binary
    if [[ ! -x "$BINARY_PATH" ]]; then
        log_error "Binary not executable: $BINARY_PATH"
        return 1
    fi
    
    # Check service file
    if [[ ! -f "$SERVICE_FILE" ]]; then
        log_error "Service file not found: $SERVICE_FILE"
        return 1
    fi
    
    # Check configuration
    local config_file="$GORSUDP_CONFIG_DIR/config.json"
    if [[ ! -f "$config_file" ]]; then
        log_error "Configuration file not found: $config_file"
        return 1
    fi
    
    # Test health check
    if ! "$BINARY_PATH" --health-check --config "$config_file"; then
        log_warn "Health check failed. Please check configuration."
        return 1
    fi
    
    log_info "Installation validation successful"
    return 0
}

# Display usage information
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -b, --binary PATH    Path to GoRSUDP binary (default: ./gorsudp)"
    echo "  -s, --service PATH   Path to service file (default: ./gorsudp.service)"
    echo "  -h, --help          Show this help message"
    echo ""
    echo "Example:"
    echo "  $0 --binary /path/to/gorsudp --service /path/to/gorsudp.service"
}

# Main installation function
main() {
    local binary_path="./gorsudp"
    local service_path="./gorsudp.service"
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -b|--binary)
                binary_path="$2"
                shift 2
                ;;
            -s|--service)
                service_path="$2"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    log_info "Starting GoRSUDP installation"
    
    check_root
    create_user
    create_directories
    install_binary "$binary_path"
    install_service "$service_path"
    create_default_config
    setup_logrotate
    setup_systemd
    
    if validate_installation; then
        log_info "Installation completed successfully!"
        echo ""
        log_info "Next steps:"
        echo "  1. Edit configuration: $GORSUDP_CONFIG_DIR/config.json"
        echo "  2. Start service: systemctl start gorsudp"
        echo "  3. Check status: systemctl status gorsudp"
        echo "  4. View logs: journalctl -u gorsudp -f"
    else
        log_error "Installation completed with warnings. Please check configuration."
        exit 1
    fi
}

# Run main function
main "$@"