#!/bin/bash

# Security check script for GoDaddy Terraform Provider
# Run this before pushing to public repositories

set -e

echo "🔍 Security Check for GoDaddy Terraform Provider"
echo "================================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ISSUES_FOUND=0

# Check for hardcoded API keys
echo "🔑 Checking for hardcoded API keys..."
# Look for credential-shaped literals, not for one specific key. Matching a
# real key fragment here would publish that fragment.
if find . -type f \( -name "*.tf" -o -name "*.tfvars" -o -name "*.go" -o -name "*.md" -o -name "*.sh" \) -not -path "./.git/*" -not -path "./.terraform/*" -not -path "./scripts/security-check.sh" -print0 | xargs -0 grep -lE 'sso-key [A-Za-z0-9_]{10,}:[A-Za-z0-9_]{10,}|api_(key|secret|token)[[:space:]]*=[[:space:]]*"[A-Za-z0-9_.-]{16,}"' 2>/dev/null; then
    echo -e "${RED}❌ DANGER: Hardcoded API credentials found!${NC}"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
    echo -e "${GREEN}✅ No hardcoded API credentials found${NC}"
fi

# Check for Terraform state files
echo "📁 Checking for Terraform state files..."
if find . -name "*.tfstate*" -o -name ".terraform" -o -name "*.tfplan" | grep -q .; then
    echo -e "${RED}❌ DANGER: Terraform state files found!${NC}"
    find . -name "*.tfstate*" -o -name ".terraform" -o -name "*.tfplan"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
    echo -e "${GREEN}✅ No Terraform state files found${NC}"
fi

# Check for test files with credentials
echo "🧪 Checking for test files with potential credentials..."
if find . -name "test-*.tf" -o -name "*-test.tf" | grep -q .; then
    echo -e "${YELLOW}⚠️  Test files found (may contain credentials):${NC}"
    find . -name "test-*.tf" -o -name "*-test.tf"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
    echo -e "${GREEN}✅ No test files with potential credentials found${NC}"
fi

# Check for environment files
echo "🌍 Checking for environment files..."
if find . -name ".env*" -o -name "*credentials*" -o -name "*secrets*" | grep -q .; then
    echo -e "${RED}❌ DANGER: Environment/credential files found!${NC}"
    find . -name ".env*" -o -name "*credentials*" -o -name "*secrets*"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
    echo -e "${GREEN}✅ No environment/credential files found${NC}"
fi

# Check .gitignore
echo "📝 Checking .gitignore completeness..."
if [ -f .gitignore ]; then
    if grep -q "test-\*\.tf\|\.tfstate\|\.terraform\|\.env" .gitignore; then
        echo -e "${GREEN}✅ .gitignore includes security patterns${NC}"
    else
        echo -e "${YELLOW}⚠️  .gitignore may be incomplete${NC}"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
    fi
else
    echo -e "${RED}❌ DANGER: No .gitignore file found!${NC}"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
fi

# Check for sensitive patterns in code
echo "🔍 Checking for sensitive patterns..."
SENSITIVE_PATTERNS=(
    "password.*=.*[\"'].*[\"']"
    "secret.*=.*[\"'].*[\"']"
    "token.*=.*[\"'].*[\"']"
    "key.*=.*[\"'][a-zA-Z0-9_]{15,}[\"']"
)

for pattern in "${SENSITIVE_PATTERNS[@]}"; do
    if find . -type f \( -name "*.tf" -o -name "*.go" -o -name "*.md" \) -not -path "./.git/*" -not -path "./scripts/*" -not -path "./internal/provider/*_test.go" -not -path "./docs/*" | xargs grep -l "$pattern" 2>/dev/null; then
        echo -e "${YELLOW}⚠️  Potential sensitive pattern found: $pattern${NC}"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
    fi
done

# Summary
echo ""
echo "📊 Security Check Summary"
echo "========================="

if [ $ISSUES_FOUND -eq 0 ]; then
    echo -e "${GREEN}🎉 All security checks passed! Safe to push to public repository.${NC}"
    exit 0
else
    echo -e "${RED}❌ $ISSUES_FOUND security issues found. Please fix before pushing to public repository.${NC}"
    echo ""
    echo "🔧 Recommended actions:"
    echo "  1. Remove any hardcoded credentials"
    echo "  2. Delete Terraform state files: rm -rf .terraform* *.tfstate*"
    echo "  3. Remove test files with credentials: rm test-*.tf *-test.tf"
    echo "  4. Update .gitignore if needed"
    echo "  5. Use environment variables for API credentials"
    exit 1
fi