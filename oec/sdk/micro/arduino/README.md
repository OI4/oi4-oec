# OECMicro – the OEC Micro SDK as an Arduino library

The OEC Micro SDK (D9) as an Arduino library for the ESP32 family: an OI4 Open
Edge Computing (OEC) v2.0 smart device with MQTT 3.1.1 over TLS, messaging
levels 1 to 3. It is the portable C17 core of the SDK with its ESP-IDF port
(esp-mqtt, NVS) – the same code as the ESP-IDF component, assembled by
`package.sh` (ADR-0024, C-097). The SDK documentation is
`oec/sdk/micro/README.md` in the OEC repository.

## Install

```sh
oec/sdk/micro/arduino/package.sh     # → target/arduino/OECMicro/ and OECMicro-<version>.zip
```

Arduino IDE: *Sketch → Include Library → Add .ZIP Library…* with the zip, or
copy `target/arduino/OECMicro/` into the `libraries/` folder of the sketchbook.
arduino-cli: `arduino-cli compile --library target/arduino/OECMicro …`.

Boards: the Arduino-ESP32 core (`esp32:esp32`) 3.3 or later, which brings
esp-mqtt with it. The examples are built with 3.3.12 (ESP-IDF 5.5) and
4.0.0-RC1 (ESP-IDF 6.1) for the ESP32 Dev Module, ESP32-S3 Dev Module,
M5Core2, M5CoreS3 and M5Core, without warnings in the library. On the M5Core
(4 MB flash, default partition scheme) the `M5Stack` example fills 93 % (3.3)
or 98 % (4.0) of the app partition: choose a larger partition scheme there.

## Examples

| Example | Board | What it shows |
|---|---|---|
| `Sensor` | any ESP32 with Wi-Fi | The simulated temperature sensor of the SDK: Level 1 Birth/Health/Close/Will; Level 2 Data "Temperature" (writable Setpoint) and Config "Sensor"; Level 3 ReferenceDesignation and NewDataSetWriterId. Serial monitor: `stop` ends the device gracefully. |
| `M5Stack` | M5Stack Core2, CoreS3 | The same device with a status display (Identification Link, connection, Health, Value/Setpoint); button A or a tap cycles the Health. Needs [M5Unified](https://github.com/m5stack/M5Unified) (MIT). |

## Provisioning

Each example reads its settings from `arduino_secrets.h` next to the sketch:
Wi-Fi SSID and password, the broker URI `mqtts://host:port` (the `SecurePort`
of `broker.json`), the PEM of the CA that signed the broker certificate, and
username + password or a client certificate + key (the certificate wins). The
messaging level is `LEVEL` in the sketch. Replace the placeholders in your copy
of the sketch and never commit real values. The persisted records of Level 3
live in the NVS namespace `oecm`; the Arduino-ESP32 core initializes NVS.

The device needs the wall clock before it connects (the MessageId carries the
time): the examples call `configTime` and wait for SNTP.

## Use in an own sketch

```cpp
#include <OECMicro.h>

oecm_esp_init(&port_config, &port, &pal);       // TLS, credentials, NVS
oecm_init(&dev, &cfg, &pal, tx, sizeof tx);     // MAM, level, definitions
oecm_start(&dev);

void loop() {
  uint32_t wait = oecm_poll_interval_ms(&dev);
  oecm_esp_dispatch(port, &dev, wait > 200 ? 200 : wait);
  oecm_poll(&dev);
}
```

Every `oecm_` function runs on the loop task; the port hands the MQTT events
of the esp-mqtt task over in `oecm_esp_dispatch`. The API is documented in
`oecm.h` and `oecm_esp.h`.

## Licenses

OECMicro is MIT (`LICENSE`). A sketch built with it also links the
Arduino-ESP32 core (LGPL-2.1) and ESP-IDF with esp-mqtt and mbedTLS
(Apache-2.0); the M5Stack example links M5Unified and M5GFX (MIT).
