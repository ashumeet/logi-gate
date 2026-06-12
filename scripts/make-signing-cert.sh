#!/usr/bin/env bash
# Idempotently create a STABLE self-signed code-signing identity in the login
# keychain, so `make` can sign with a cert-anchored Designated Requirement even
# on a machine with no Apple Development / Developer ID certificate.
#
# Why this exists: ad-hoc signing (codesign -s -) gives TCC only a cdhash-based
# DR with no certificate anchor, so macOS drops Accessibility + Input Monitoring
# grants on every reboot. A self-signed code-signing cert (100-year validity)
# yields a cert-anchored DR that survives reboot AND rebuild — and never expires.
#
# FIRST RUN IS INTERACTIVE: macOS requires your login password once to (a) let
# codesign use the new private key without prompting on every build, and (b)
# trust the cert so it lists as a code-signing identity. Subsequent runs are a
# no-op. This is unavoidable for self-signed certs on macOS.
#
# Usage: make-signing-cert.sh "LogiGate Local Signing"
set -euo pipefail

CN="${1:-LogiGate Local Signing}"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"

# Already present? (match by cert common name, not the codesigning policy — a
# freshly-created self-signed cert may not pass the policy filter yet.)
if security find-certificate -c "$CN" "$KEYCHAIN" >/dev/null 2>&1; then
  echo "→ Signing identity '$CN' already present — nothing to do."
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "→ Generating self-signed code-signing certificate '$CN' (100-year)..."
# PEM key + cert. extendedKeyUsage=codeSigning is what makes it a code-signing
# identity. We import key/cert SEPARATELY below because OpenSSL 3's PKCS#12 MAC
# is rejected by macOS `security import`.
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout "$TMP/key.pem" -out "$TMP/cert.pem" -days 36500 \
  -subj "/CN=$CN" \
  -addext "basicConstraints=critical,CA:false" \
  -addext "keyUsage=critical,digitalSignature" \
  -addext "extendedKeyUsage=critical,codeSigning" >/dev/null 2>&1

echo "→ Importing certificate + private key into the login keychain..."
security import "$TMP/cert.pem" -k "$KEYCHAIN" -A >/dev/null
security import "$TMP/key.pem"  -k "$KEYCHAIN" -A -T /usr/bin/codesign >/dev/null

echo ""
echo "→ Authorizing codesign to use the key + trusting the cert."
echo "  macOS needs your Mac LOGIN password once (input hidden):"
read -r -s -p "  login password: " LOGIN_PW
echo ""

# Let codesign access the private key without a GUI prompt on every build.
security set-key-partition-list -S apple-tool:,apple:,codesign: \
  -s -k "$LOGIN_PW" "$KEYCHAIN" >/dev/null 2>&1 || {
    echo "✗ set-key-partition-list failed (wrong password?). Re-run: make signing-cert"; exit 1; }

# Trust the cert for code signing so it lists under `find-identity -p codesigning`.
# Login-keychain trust avoids sudo; may show one GUI confirmation the first time.
security add-trusted-cert -r trustRoot -p codeSign -k "$KEYCHAIN" "$TMP/cert.pem" \
  >/dev/null 2>&1 || true

unset LOGIN_PW

echo "✓ Created code-signing identity '$CN'."
security find-identity -v -p codesigning | grep -i "$CN" || \
  security find-certificate -c "$CN" -Z "$KEYCHAIN" | awk '/SHA-1/{print "  SHA-1:"$NF}'
echo ""
echo "  Back up this identity so rebuilds stay stable on this machine:"
echo "    security export -k \"$KEYCHAIN\" -t identities -f pkcs12 -P '' -o ~/logigate-signing-id.p12"
