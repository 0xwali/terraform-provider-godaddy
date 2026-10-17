#!/bin/bash

# GoDaddy Terraform Provider Import Helper
# This script helps import existing GoDaddy resources into Terraform

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
DOMAIN=""
RESOURCE_TYPE=""
API_KEY=""
API_SECRET=""
ENVIRONMENT=""

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -d, --domain DOMAIN          Domain name (required)"
    echo "  -t, --type TYPE             Resource type: dns_record, domain (default: dns_record)"
    echo "  -k, --api-key KEY           GoDaddy API Key (not recommended: visible in ps and shell history)"
    echo "  -s, --api-secret SECRET     GoDaddy API Secret (not recommended: visible in ps and shell history)"
    echo "  -e, --environment ENV       Environment: production, test (default: production)"
    echo "  -h, --help                  Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 -d example.com                           # List all DNS records for example.com"
    echo "  $0 -d example.com -t domain                 # Import domain example.com"
    echo ""
    echo "Environment variables (preferred for credentials):"
    echo "  GODADDY_API_KEY             GoDaddy API Key"
    echo "  GODADDY_API_SECRET          GoDaddy API Secret"
    echo "  GODADDY_ENVIRONMENT         Environment (production/test)"
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_requirements() {
    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi

    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed. Please install jq: https://stedolan.github.io/jq/"
        exit 1
    fi

    if ! command -v terraform &> /dev/null; then
        log_error "terraform is required but not installed"
        exit 1
    fi
}

setup_credentials() {
    # Use command line args or environment variables
    if [[ -z "$API_KEY" ]]; then
        API_KEY="${GODADDY_API_KEY}"
    fi
    
    if [[ -z "$API_SECRET" ]]; then
        API_SECRET="${GODADDY_API_SECRET}"
    fi
    
    if [[ -z "$ENVIRONMENT" ]]; then
        ENVIRONMENT="${GODADDY_ENVIRONMENT:-production}"
    fi

    if [[ -z "$API_KEY" || -z "$API_SECRET" ]]; then
        log_error "API credentials are required. Set GODADDY_API_KEY and GODADDY_API_SECRET environment variables or use -k and -s options."
        exit 1
    fi

    if [[ "$ENVIRONMENT" != "production" && "$ENVIRONMENT" != "test" ]]; then
        log_error "Environment must be production or test, got: $ENVIRONMENT"
        exit 1
    fi
}

# api_get PATH prints the JSON body of GET <api>/PATH. The Authorization
# header goes to curl on stdin (curl --config -), so the key and secret never
# appear on a command line where ps or shell history could show them.
api_get() {
    local path="$1"
    local key="${API_KEY//\\/\\\\}"
    key="${key//\"/\\\"}"
    local secret="${API_SECRET//\\/\\\\}"
    secret="${secret//\"/\\\"}"
    printf 'header = "Authorization: sso-key %s:%s"\n' "$key" "$secret" |
        curl -sS --fail-with-body --proto '=https' --max-redirs 0 --max-time 30 \
            --config - \
            -H "Accept: application/json" \
            "$(get_api_url)/$path"
}

# resource_label turns a record name and type into a valid Terraform
# resource name: lowercase letters, digits and underscores only.
resource_label() {
    local label
    label=$(printf '%s_%s' "$1" "$2" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9_' '_' | sed 's/__*/_/g; s/^_//; s/_$//')
    if [[ -z "$label" || "$label" == "$(printf '%s' "$2" | tr '[:upper:]' '[:lower:]')" ]]; then
        label="root_$(printf '%s' "$2" | tr '[:upper:]' '[:lower:]')"
    fi
    if [[ "$label" =~ ^[0-9] ]]; then
        label="r_$label"
    fi
    printf '%s' "$label"
}

get_api_url() {
    if [[ "$ENVIRONMENT" == "test" ]]; then
        echo "https://api.ote-godaddy.com/v1"
    else
        echo "https://api.godaddy.com/v1"
    fi
}

list_dns_records() {
    local domain="$1"
    local response

    log_info "Fetching DNS records for domain: $domain"

    if ! response=$(api_get "domains/$domain/records"); then
        log_error "Failed to fetch DNS records: $response"
        return 1
    fi
    
    # Check if response contains error
    if echo "$response" | jq -e '.code' > /dev/null 2>&1; then
        local error_code=$(echo "$response" | jq -r '.code')
        local error_message=$(echo "$response" | jq -r '.message')
        log_error "API Error: $error_code - $error_message"
        return 1
    fi
    
    log_success "Found $(echo "$response" | jq '. | length') DNS records"
    echo ""
    
    # Display records in a table format
    printf "%-6s %-20s %-60s %-8s %-8s\n" "TYPE" "NAME" "DATA" "TTL" "PRIORITY"
    printf "%-6s %-20s %-60s %-8s %-8s\n" "----" "----" "----" "---" "--------"
    
    echo "$response" | jq -r '.[] | [.type, .name, .data, .ttl, (.priority // "")] | @tsv' | \
    while IFS=$'\t' read -r type name data ttl priority; do
        printf "%-6s %-20s %-60s %-8s %-8s\n" "$type" "$name" "$data" "$ttl" "$priority"
    done
    
    echo ""
    log_info "To import a DNS record, use:"
    echo "terraform import godaddy_dns_record.RESOURCE_NAME \"$domain/TYPE/NAME/DATA\""
    echo ""
    log_info "Or use the simplified format (auto-detect data for unique records):"
    echo "terraform import godaddy_dns_record.RESOURCE_NAME \"$domain/TYPE/NAME\""
    echo ""
    
    # Generate import commands
    echo "# Suggested import commands:"
    echo "$response" | jq -r '.[] | [.type, .name, .data] | @tsv' | \
    while IFS=$'\t' read -r type name data; do
        echo "# terraform import godaddy_dns_record.$(resource_label "$name" "$type") \"$domain/$type/$name/$data\""
    done
}

get_domain_info() {
    local domain="$1"
    local response

    log_info "Fetching domain information for: $domain"

    if ! response=$(api_get "domains/$domain"); then
        log_error "Failed to fetch domain information: $response"
        return 1
    fi
    
    # Check if response contains error
    if echo "$response" | jq -e '.code' > /dev/null 2>&1; then
        local error_code=$(echo "$response" | jq -r '.code')
        local error_message=$(echo "$response" | jq -r '.message')
        log_error "API Error: $error_code - $error_message"
        return 1
    fi
    
    log_success "Domain information retrieved"
    echo ""
    
    echo "Domain: $(echo "$response" | jq -r '.domain')"
    echo "Status: $(echo "$response" | jq -r '.status')"
    echo "Expires: $(echo "$response" | jq -r '.expires')"
    echo "Locked: $(echo "$response" | jq -r '.locked')"
    echo "Privacy: $(echo "$response" | jq -r '.privacy')"
    echo "Auto-renew: $(echo "$response" | jq -r '.renewAuto')"
    echo ""
    
    log_info "To import this domain, use:"
    echo "terraform import godaddy_domain.RESOURCE_NAME \"$domain\""
    echo ""
    echo "# Suggested import command:"
    echo "# terraform import godaddy_domain.$(echo "$domain" | tr '.' '_') \"$domain\""
}

generate_terraform_config() {
    local domain="$1"
    local output_file="${domain}.tf"
    
    log_info "Generating Terraform configuration for domain: $domain"
    
    local domain_response dns_response

    if ! domain_response=$(api_get "domains/$domain"); then
        log_error "Failed to fetch domain information: $domain_response"
        return 1
    fi

    if ! dns_response=$(api_get "domains/$domain/records"); then
        log_error "Failed to fetch DNS records: $dns_response"
        return 1
    fi
    
    cat > "$output_file" << EOF
# Terraform configuration for domain $domain
# Generated by GoDaddy Provider Import Helper

terraform {
  required_providers {
    godaddy = {
      source = "local/bearcode33/godaddy"
      version = "1.0.0"
    }
  }
}

provider "godaddy" {
  # Configure your credentials
  # api_key     = var.godaddy_api_key
  # api_secret  = var.godaddy_api_secret
  # environment = "$ENVIRONMENT"
}

# Domain resource
resource "godaddy_domain" "$(echo "$domain" | tr '.-' '__')" {
  domain     = "$domain"
  locked     = $(echo "$domain_response" | jq '.locked == true')
  renew_auto = $(echo "$domain_response" | jq '.renewAuto == true')
}

EOF

    # Add DNS records if available
    if ! echo "$dns_response" | jq -e '.code' > /dev/null 2>&1; then
        echo "# DNS Records" >> "$output_file"
        # Values come from the API. jq writes each string as a JSON string
        # (the same escapes HCL uses) and doubles $ and % before { so a
        # record cannot close the string or start an HCL template. Numbers
        # are written only when they are numbers.
        echo "$dns_response" | jq -r --arg domain "$domain" '
            def hcl: tostring | tojson | gsub("\\$\\{"; "$${") | gsub("%\\{"; "%%{");
            def rlabel:
                ((.name + "_" + .type) | ascii_downcase | gsub("[^a-z0-9_]"; "_") | gsub("_+"; "_")
                    | ltrimstr("_") | rtrimstr("_")) as $l
                | (.type | ascii_downcase) as $t
                | (if $l == "" or $l == $t then "root_" + $t else $l end)
                | if test("^[0-9]") then "r_" + . else . end;
            def num($key): if (.[$key] | type) == "number" then "  \($key) = \(.[$key])\n" else "" end;
            .[] |
            "\nresource \"godaddy_dns_record\" \"\(rlabel)\" {\n" +
            "  domain = \($domain | hcl)\n" +
            "  type   = \(.type | hcl)\n" +
            "  name   = \(.name | hcl)\n" +
            "  data   = \(.data | hcl)\n" +
            num("ttl") + num("priority") + num("port") + num("weight") +
            "}"' >> "$output_file"
    fi
    
    log_success "Terraform configuration generated: $output_file"
    log_info "Next steps:"
    echo "1. Review and customize the generated configuration"
    echo "2. Run: terraform init"
    echo "3. Import existing resources using the commands above"
    echo "4. Run: terraform plan"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -d|--domain)
            DOMAIN="$2"
            shift 2
            ;;
        -t|--type)
            RESOURCE_TYPE="$2"
            shift 2
            ;;
        -k|--api-key)
            API_KEY="$2"
            log_warning "-k puts the API key in ps output and shell history. Prefer GODADDY_API_KEY."
            shift 2
            ;;
        -s|--api-secret)
            API_SECRET="$2"
            log_warning "-s puts the API secret in ps output and shell history. Prefer GODADDY_API_SECRET."
            shift 2
            ;;
        -e|--environment)
            ENVIRONMENT="$2"
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

# Validate required arguments
if [[ -z "$DOMAIN" ]]; then
    log_error "Domain is required"
    usage
    exit 1
fi

# The domain goes into API URLs and the generated file.
if [[ ! "$DOMAIN" =~ ^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)+$ ]]; then
    log_error "Domain must be a hostname such as example.com, got: $DOMAIN"
    exit 1
fi

# Set default resource type
if [[ -z "$RESOURCE_TYPE" ]]; then
    RESOURCE_TYPE="dns_record"
fi

# Main execution
check_requirements
setup_credentials

log_info "GoDaddy Terraform Provider Import Helper"
log_info "Domain: $DOMAIN"
log_info "Resource Type: $RESOURCE_TYPE"
log_info "Environment: $ENVIRONMENT"
echo ""

case "$RESOURCE_TYPE" in
    "dns_record")
        list_dns_records "$DOMAIN"
        ;;
    "domain")
        get_domain_info "$DOMAIN"
        ;;
    "all")
        get_domain_info "$DOMAIN"
        echo ""
        list_dns_records "$DOMAIN"
        echo ""
        generate_terraform_config "$DOMAIN"
        ;;
    *)
        log_error "Unknown resource type: $RESOURCE_TYPE"
        log_error "Supported types: dns_record, domain, all"
        exit 1
        ;;
esac