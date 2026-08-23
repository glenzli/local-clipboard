#!/bin/bash
# One-time: create a local code-signing cert so TCC permissions survive reinstalls.
set -e
CN=${1:-localclipboard-dev}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

openssl req -x509 -newkey rsa:2048 -keyout "$TMP/key" -out "$TMP/crt" \
  -days 3650 -nodes -subj "/CN=$CN" \
  -addext "keyUsage=digitalSignature" \
  -addext "extendedKeyUsage=codeSigning" 2>/dev/null

( openssl pkcs12 -export -out "$TMP/p12" -inkey "$TMP/key" -in "$TMP/crt" \
    -passout pass:tmppass -legacy 2>/dev/null ||
  openssl pkcs12 -export -out "$TMP/p12" -inkey "$TMP/key" -in "$TMP/crt" \
    -passout pass:tmppass -keypbe PBE-SHA1-3DES -certpbe PBE-SHA1-3DES -macalg sha1 )

security import "$TMP/p12" -k ~/Library/Keychains/login.keychain-db -P tmppass -T /usr/bin/codesign
security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db -p codeSign "$TMP/crt"
echo "created. verify with: security find-identity -v -p codesigning"
