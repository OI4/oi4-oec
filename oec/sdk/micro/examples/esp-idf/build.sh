#!/usr/bin/env bash
# Builds the ESP-IDF example of the OEC Micro SDK (C-096, opt-in, not in
# the gate): needs ESP-IDF >= 6.0 (IDF_PATH, export.sh) and fetches esp-mqtt
# from the ESP Component Registry on the first build (dependencies.lock).
#
#   examples/esp-idf/build.sh [esp32|esp32s3|qemu ...]   (default: all three)
#
# Output: oec/sdk/micro/target/esp-idf/<variant>/ (oecm_example.bin,
# flash_args, sdkconfig). The variant qemu is an esp32 with OpenCores
# Ethernet for the Espressif QEMU (sdkconfig.qemu, host/cmd/oecm-qemu).
# After every build: idf.py size and the size of the oecm component.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
out="$(cd "$here/../.." && pwd)/target/esp-idf"
if [ -z "${IDF_PATH:-}" ] || ! command -v idf.py >/dev/null; then
    echo "build.sh: ESP-IDF not set up (. \$IDF_PATH/export.sh)" >&2
    exit 2
fi
variants=("$@")
[ ${#variants[@]} -gt 0 ] || variants=(esp32 esp32s3 qemu)
for v in "${variants[@]}"; do
    case "$v" in
    esp32 | esp32s3) target=$v defaults="sdkconfig.defaults" ;;
    qemu) target=esp32 defaults="sdkconfig.defaults;sdkconfig.qemu" ;;
    *)
        echo "build.sh: unknown variant $v" >&2
        exit 2
        ;;
    esac
    b="$out/$v"
    mkdir -p "$b"
    echo "== $v ($target) → $b"
    idf.py -C "$here" -B "$b" -D SDKCONFIG="$b/sdkconfig" -D SDKCONFIG_DEFAULTS="$defaults" \
        -D IDF_TARGET="$target" build
    idf.py -C "$here" -B "$b" -D SDKCONFIG="$b/sdkconfig" size
    idf.py -C "$here" -B "$b" -D SDKCONFIG="$b/sdkconfig" size-components | grep -E "Archive File|liboecm|libespressif__mqtt|libmbedtls|libmbedcrypto|liblwip|Total"
done
