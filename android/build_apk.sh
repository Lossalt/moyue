#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
SDK="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$HOME/Android/Sdk}}"
BT="${ANDROID_BUILD_TOOLS:-$SDK/build-tools/35.0.0}"
PLATFORM="$SDK/platforms/android-35/android.jar"
JAVAC="${JAVAC:-javac}"
D8="$BT/d8"
AAPT2="$BT/aapt2"
APKSIGNER="$BT/apksigner"
ZIPALIGN="$BT/zipalign"
DIST="${DIST:-$ROOT/../dist}"
BUILD="$ROOT/build"
OUT="$DIST/MoYue.apk"

if [ ! -f "$PLATFORM" ]; then
  echo "Android platform not found: $PLATFORM" >&2
  echo "Install it with: sdkmanager 'platforms;android-35' 'build-tools;35.0.0'" >&2
  echo "or set ANDROID_SDK_ROOT / ANDROID_BUILD_TOOLS." >&2
  exit 1
fi
if [ ! -x "$AAPT2" ] || [ ! -x "$D8" ] || [ ! -x "$APKSIGNER" ] || [ ! -x "$ZIPALIGN" ]; then
  echo "Android build-tools not found under: $BT" >&2
  exit 1
fi

KEYSTORE="${ANDROID_KEYSTORE:-$ROOT/../.android-keystore/moyue.jks}"
ALIAS="${ANDROID_KEY_ALIAS:-moyue}"
STORE_PASS="${MOYUE_KEYSTORE_PASS:-$(cat "$ROOT/../.android-keystore/pass" 2>/dev/null || true)}"
KEY_PASS="${MOYUE_KEY_PASS:-$STORE_PASS}"
if [ ! -f "$KEYSTORE" ] || [ -z "$STORE_PASS" ]; then
  echo "Signing keystore/password missing." >&2
  echo "Set ANDROID_KEYSTORE, ANDROID_KEY_ALIAS and MOYUE_KEYSTORE_PASS," >&2
  echo "or create $ROOT/../.android-keystore/moyue.jks plus a 'pass' file." >&2
  exit 1
fi

echo "[1/6] resources"
rm -rf "$BUILD"
mkdir -p "$BUILD/res" "$BUILD/gen" "$BUILD/classes" "$BUILD/dex" "$DIST"
"$AAPT2" compile --dir "$ROOT/res" -o "$BUILD/res.zip"
"$AAPT2" link -o "$BUILD/base.apk" \
  -I "$PLATFORM" \
  --manifest "$ROOT/AndroidManifest.xml" \
  --java "$BUILD/gen" \
  --min-sdk-version 26 \
  --target-sdk-version 34 \
  --version-code 1 \
  --version-name 1.0.0 \
  "$BUILD/res.zip"

echo "[2/6] javac"
"$JAVAC" -encoding UTF-8 -source 8 -target 8 \
  -bootclasspath "$PLATFORM" -d "$BUILD/classes" \
  $(find "$ROOT/src" "$BUILD/gen" -name '*.java')

echo "[3/6] d8"
"$D8" --lib "$PLATFORM" --min-api 26 --output "$BUILD/dex" \
  $(find "$BUILD/classes" -name '*.class')

echo "[4/6] package"
cp "$BUILD/base.apk" "$BUILD/unsigned.apk"
(cd "$BUILD/dex" && zip -q -j "$BUILD/unsigned.apk" classes.dex)
"$ZIPALIGN" -f -p 4 "$BUILD/unsigned.apk" "$BUILD/aligned.apk"

echo "[5/6] sign"
export MOYUE_KEYSTORE_PASS="$STORE_PASS"
export MOYUE_KEY_PASS="$KEY_PASS"
"$APKSIGNER" sign \
  --ks "$KEYSTORE" --ks-key-alias "$ALIAS" \
  --ks-pass env:MOYUE_KEYSTORE_PASS --key-pass env:MOYUE_KEY_PASS \
  --out "$OUT" "$BUILD/aligned.apk"

echo "[6/6] verify"
"$APKSIGNER" verify --verbose --print-certs "$OUT" | head -12
"$AAPT2" dump badging "$OUT" | grep -E "^package:|^application-label:|^sdkVersion:|^targetSdkVersion:|^launchable-activity:" || true
ls -lh "$OUT"
sha256sum "$OUT"
