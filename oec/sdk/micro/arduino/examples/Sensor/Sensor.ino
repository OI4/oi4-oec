/*
 * Sensor - the simulated temperature sensor of the OEC Micro SDK as an OEC
 * smart device on an ESP32 with Wi-Fi (C-097): the Arduino twin of the
 * ESP-IDF example (examples/esp-idf in the SDK).
 *
 * Settings: arduino_secrets.h (Wi-Fi, broker, CA, credentials) and LEVEL
 * below. The persisted records of Level 3 live in the NVS namespace "oecm".
 * Serial monitor (115200 Bd): "stop" ends the device gracefully (Close,
 * DISCONNECT).
 */
#include <WiFi.h>
#include <esp_chip_info.h>
#include <esp_mac.h>
#include <time.h>

#include <OECMicro.h>

#include "arduino_secrets.h"
#include "oecm_sensor.h"

// Messaging level 1 ... 3; from Level 2 the sensor adds its Data and Config.
#define LEVEL 3
// The MaxPacketSize of broker.json; 0: the default of the SDK.
#define MAX_PACKET 0
// A wall clock before 2023 is not set yet.
#define TIME_PLAUSIBLE 1672531200

static oecm_device dev;
static uint8_t tx[8192];  // the largest publication: the Config with its definition
static char refdes[2048];
static oecm_config cfg;
static oecm_esp_config pc;
static oecm_esp *port;
static oecm_pal pal;
static char serial[13];
static char hw[16];
static bool running;
static uint32_t nextTick;
static String line;

static const char *nonEmpty(const char *s) { return s[0] ? s : nullptr; }

static void fail(const char *what, const char *why) {
  Serial.printf("%s: %s\n", what, why);
  running = false;
}

void setup() {
  Serial.begin(115200);
  WiFi.mode(WIFI_STA);
  WiFi.begin(SECRET_WIFI_SSID, SECRET_WIFI_PASS);
  Serial.print("Wi-Fi");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print('.');
  }
  Serial.printf("\naddress %s\n", WiFi.localIP().toString().c_str());
  configTime(0, 0, SECRET_NTP_SERVER);
  while (time(nullptr) < TIME_PLAUSIBLE) {
    Serial.println("waiting for the time");
    delay(2000);
  }

  uint8_t mac[6];
  esp_read_mac(mac, ESP_MAC_BASE);
  snprintf(serial, sizeof serial, "%02X%02X%02X%02X%02X%02X", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]);
  esp_chip_info_t chip;
  esp_chip_info(&chip);
  snprintf(hw, sizeof hw, "v%u.%u", chip.revision / 100, chip.revision % 100);

  cfg.level = (oecm_level)LEVEL;
  cfg.service_type = OECM_ST_OT_CONNECTOR;  // a directly connected smart device (SF-102)
  cfg.max_packet_size = MAX_PACKET;
  cfg.mam.manufacturer = "OI4";
  cfg.mam.manufacturer_uri = "oi4.org";
  cfg.mam.model = "OEC Micro Arduino";
  cfg.mam.product_code = "OECM-ARDUINO";
  cfg.mam.hardware_revision = hw;
  cfg.mam.software_revision = "0.1.0";
  cfg.mam.device_manual = "https://oi4.org";
  cfg.mam.device_class = "Sensor";
  cfg.mam.serial_number = serial;
  cfg.mam.revision_counter = -1;

  pc.uri = SECRET_BROKER_URI;
  pc.ca_pem = SECRET_CA_PEM;
  pc.username = nonEmpty(SECRET_USERNAME);
  pc.password = nonEmpty(SECRET_PASSWORD);
  pc.client_cert_pem = nonEmpty(SECRET_CLIENT_CERT_PEM);
  pc.client_key_pem = nonEmpty(SECRET_CLIENT_KEY_PEM);
  esp_err_t e = oecm_esp_init(&pc, &port, &pal);
  if (e != ESP_OK)
    return fail("oecm_esp_init (broker URI, CA, credentials)", esp_err_to_name(e));
  if (LEVEL >= 2)
    oecm_sensor_setup(&cfg, &dev);
  if (LEVEL >= 3) {
    cfg.refdes_buf = refdes;
    cfg.refdes_cap = sizeof refdes;
  }
  oecm_err oe = oecm_init(&dev, &cfg, &pal, tx, sizeof tx);
  if (oe == OECM_OK)
    oe = oecm_start(&dev);
  if (oe != OECM_OK)
    return fail("oecm", oecm_err_str(oe));
  Serial.printf("device %s (Level %u), broker %s\n", oecm_identification_link(&dev), LEVEL, pc.uri);
  running = true;
  nextTick = millis() + 1000u * oecm_sensor_interval_s();
}

static void readConsole() {
  while (Serial.available()) {
    char c = (char)Serial.read();
    if (c != '\n' && c != '\r') {
      if (line.length() < 32)
        line += c;
      continue;
    }
    if (line == "stop") {
      Serial.printf("oecm: stopped (%s)\n", oecm_err_str(oecm_stop(&dev)));
      running = false;
    }
    line = "";
  }
}

void loop() {
  if (!running) {
    delay(1000);
    return;
  }
  uint32_t wait = oecm_poll_interval_ms(&dev);
  oecm_esp_dispatch(port, &dev, wait > 200 ? 200 : wait);
  readConsole();
  if (!running)
    return;
  if (LEVEL >= 2 && (int32_t)(millis() - nextTick) >= 0) {
    oecm_sensor_tick();
    nextTick = millis() + 1000u * oecm_sensor_interval_s();
  }
  oecm_err oe = oecm_poll(&dev);
  if (oe != OECM_OK)
    Serial.printf("oecm_poll: %s\n", oecm_err_str(oe));
}
