#!/bin/bash

# Generate signature for specific Yaya Wallet webhook payload with current timestamp
echo "🔐 Generating HMAC-SHA256 signature for specific payload with current timestamp"

# Secret key
SECRET_KEY="secret"

# Get current timestamp
CURRENT_TIMESTAMP=$(date +%s)

# Generate random UUID for ID
if command -v uuidgen &> /dev/null; then
    # Use uuidgen if available (Linux/macOS)
    ID=$(uuidgen)
elif command -v python3 &> /dev/null; then
    # Use Python if uuidgen not available
    ID=$(python3 -c "import uuid; print(str(uuid.uuid4()))")
else
    # Fallback: generate a simple random string
    ID="$(date +%s)-$(openssl rand -hex 8)"
fi

# Payload data from user (with updated timestamps and random ID)
AMOUNT="100"
CURRENCY="ETB"
CREATED_AT_TIME="$CURRENT_TIMESTAMP"
TIMESTAMP="$CURRENT_TIMESTAMP"
CAUSE="Testing"
FULL_NAME="Abebe Kebede"
ACCOUNT_NAME="abebekebede1"
INVOICE_URL="https://yayawallet.com/en/invoice/xxxx"

echo "📝 Payload data:"
echo "  ID: $ID (randomly generated)"
echo "  Amount: $AMOUNT"
echo "  Currency: $CURRENCY"
echo "  Created At Time: $CREATED_AT_TIME (current)"
echo "  Timestamp: $TIMESTAMP (current)"
echo "  Cause: $CAUSE"
echo "  Full Name: $FULL_NAME"
echo "  Account Name: $ACCOUNT_NAME"
echo "  Invoice URL: $INVOICE_URL"
echo "  Secret Key: $SECRET_KEY"
echo ""

# Concatenate exactly as Go code does: %s%s%s%d%d%s%s%s%s
CONCATENATED="${ID}${AMOUNT}${CURRENCY}${CREATED_AT_TIME}${TIMESTAMP}${CAUSE}${FULL_NAME}${ACCOUNT_NAME}${INVOICE_URL}"

echo "🔗 Concatenated string for signature:"
echo "$CONCATENATED"
echo ""
echo "📏 String length: ${#CONCATENATED}"
echo "🔑 Secret key length: ${#SECRET_KEY}"
echo ""

# Generate HMAC-SHA256 signature (hex encoded)
SIGNATURE=$(echo -n "$CONCATENATED" | openssl dgst -sha256 -hmac "$SECRET_KEY" | cut -d' ' -f2)

echo "✅ Generated signature (hex):"
echo "$SIGNATURE"
echo ""

# Test command
echo "📤 Test command (copy and paste):"
echo "curl -X POST http://localhost:8080/webhook \\"
echo "  -H \"Content-Type: application/json\" \\"
echo "  -H \"YAYA-SIGNATURE: $SIGNATURE\" \\"
echo "  -d '{"
echo "    \"id\": \"$ID\","
echo "    \"amount\": $AMOUNT,"
echo "    \"currency\": \"$CURRENCY\","
echo "    \"created_at_time\": $CREATED_AT_TIME,"
echo "    \"timestamp\": $TIMESTAMP,"
echo "    \"cause\": \"$CAUSE\","
echo "    \"full_name\": \"$FULL_NAME\","
echo "    \"account_name\": \"$ACCOUNT_NAME\","
echo "    \"invoice_url\": \"$INVOICE_URL\""
echo "  }'"
echo ""
echo "✅ Now using current timestamp: $CURRENT_TIMESTAMP"
echo "✅ Random ID generated: $ID"
echo "   This should pass both signature and timestamp validation!"
echo ""
echo "💡 Each time you run this script, you'll get a new random ID!"
