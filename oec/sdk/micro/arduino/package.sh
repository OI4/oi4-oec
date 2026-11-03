#!/usr/bin/env bash
# Assembles the OEC Micro SDK as an Arduino library (ADR-0024, C-097): an
# Arduino library cannot reference files outside its folder, so the core
# (src/, include/), the ESP-IDF port (port/esp-idf/oecm/) and this folder are
# copied into <out>/OECMicro, and every example gets the shared sensor of
# examples/sensor. Then <out>/OECMicro-<version>.zip for "Add .ZIP Library".
#
# Usage: arduino/package.sh [out]   (default: target/arduino)
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
micro=$(dirname "$here")
out=${1:-$micro/target/arduino}
lib=$out/OECMicro
version=$(sed -n 's/^version=//p' "$here/library.properties")

rm -rf "$lib" "$out"/OECMicro-*.zip
mkdir -p "$lib/src" "$lib/examples"
cp "$here/library.properties" "$here/README.md" "$lib/"
cp "$micro/../../LICENSE" "$lib/LICENSE"
cp "$micro"/src/*.c "$micro"/src/*.h "$micro/include/oecm.h" \
   "$micro/port/esp-idf/oecm/oecm_esp.c" "$micro/port/esp-idf/oecm/include/oecm_esp.h" \
   "$here/OECMicro.h" "$lib/src/"
for ex in "$here"/examples/*/; do
    name=$(basename "$ex")
    cp -R "$ex" "$lib/examples/$name"
    cp "$micro/examples/sensor/oecm_sensor.c" "$micro/examples/sensor/oecm_sensor.h" "$lib/examples/$name/"
done
if command -v zip >/dev/null; then
    (cd "$out" && zip -qrX "OECMicro-$version.zip" OECMicro)
    echo "arduino: $lib, $out/OECMicro-$version.zip"
else
    echo "arduino: $lib (no zip installed: no OECMicro-$version.zip)"
fi
