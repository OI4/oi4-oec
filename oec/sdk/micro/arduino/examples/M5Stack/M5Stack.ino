/*
 * M5Stack - the simulated temperature sensor of the OEC Micro SDK as an
 * OEC smart device on an M5Stack (Core2, CoreS3; M5Unified, C-097).
 *
 * The display shows the Identification Link, the level, the connection,
 * the Health and the Value/Setpoint of the sensor (a Set of the Setpoint,
 * e.g. by the OEC config service, moves the Value). Button A or a tap on
 * the screen cycles the Health: NORMAL_0, MAINTENANCE_REQUIRED_4,
 * OFF_SPEC_3, CHECK_FUNCTION_2, FAILURE_1 (the device is unavailable;
 * back to NORMAL_0 publishes the Birth again).
 *
 * Settings: arduino_secrets.h and LEVEL below. Needs the M5Unified
 * library (MIT) besides OECMicro.
 */
#include <M5Unified.h>
#include <WiFi.h>
#include <esp_chip_info.h>
#include <esp_mac.h>
#include <time.h>

#include <OECMicro.h>

#include "arduino_secrets.h"
#include "oecm_sensor.h"

#define LEVEL 3
#define MAX_PACKET 0
#define TIME_PLAUSIBLE 1672531200

static oecm_device dev;
static uint8_t tx[8192];
static char refdes[2048];
static oecm_config cfg;
static oecm_esp_config pc;
static oecm_esp *port;
static oecm_pal pal;
static char serial[13];
static char hw[16];
static bool running;
static uint32_t nextTick;
static String shown;

struct HealthStep {
  oecm_health health;
  int score;
  const char *name;
};
static const HealthStep steps[] = {
    {OECM_HEALTH_NORMAL, 100, "NORMAL_0"},
    {OECM_HEALTH_MAINTENANCE_REQUIRED, 70, "MAINTENANCE_REQUIRED_4"},
    {OECM_HEALTH_OFF_SPEC, 50, "OFF_SPEC_3"},
    {OECM_HEALTH_CHECK_FUNCTION, 80, "CHECK_FUNCTION_2"},
    {OECM_HEALTH_FAILURE, 0, "FAILURE_1"},
};
static size_t step;

static const char *nonEmpty(const char *s) { return s[0] ? s : nullptr; }

static void status(const String &text) {
  if (text == shown)
    return;
  shown = text;
  M5.Display.fillScreen(TFT_BLACK);
  M5.Display.setCursor(0, 0);
  M5.Display.print(text);
}

static void fail(const char *what, const char *why) {
  status(String("OEC Micro\n\n") + what + ":\n" + why);
  running = false;
}

void setup() {
  M5.begin(M5.config());
  M5.Display.setTextSize(2);
  M5.Display.setTextWrap(true);
  status("OEC Micro\n\nWi-Fi ...");
  WiFi.mode(WIFI_STA);
  WiFi.begin(SECRET_WIFI_SSID, SECRET_WIFI_PASS);
  while (WiFi.status() != WL_CONNECTED)
    delay(500);
  status("OEC Micro\n\n" + WiFi.localIP().toString() + "\nwaiting for the time");
  configTime(0, 0, SECRET_NTP_SERVER);
  while (time(nullptr) < TIME_PLAUSIBLE)
    delay(500);

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
  cfg.mam.model = "OEC Micro M5Stack";
  cfg.mam.product_code = "OECM-M5STACK";
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
    return fail("oecm_esp_init", esp_err_to_name(e));
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
  running = true;
  nextTick = millis() + 1000u * oecm_sensor_interval_s();
}

static bool clicked() {
  if (M5.BtnA.wasClicked())
    return true;
  return M5.Touch.getCount() > 0 && M5.Touch.getDetail(0).wasClicked();
}

void loop() {
  M5.update();
  if (!running) {
    delay(100);
    return;
  }
  uint32_t wait = oecm_poll_interval_ms(&dev);
  oecm_esp_dispatch(port, &dev, wait > 50 ? 50 : wait);  // short: buttons and touch stay responsive
  if (clicked()) {
    step = (step + 1) % (sizeof steps / sizeof steps[0]);
    oecm_err oe = oecm_set_health(&dev, steps[step].health, steps[step].score);
    if (oe != OECM_OK)
      Serial.printf("oecm_set_health: %s\n", oecm_err_str(oe));
  }
  if (LEVEL >= 2 && (int32_t)(millis() - nextTick) >= 0) {
    oecm_sensor_tick();
    nextTick = millis() + 1000u * oecm_sensor_interval_s();
  }
  oecm_err oe = oecm_poll(&dev);
  if (oe != OECM_OK)
    Serial.printf("oecm_poll: %s\n", oecm_err_str(oe));

  String text = String("OEC Micro (Level ") + LEVEL + ")\n" + oecm_identification_link(&dev) + "\n\n" +
                (oecm_connected(&dev) ? "connected" : "connecting ...") + "\nHealth " + steps[step].name + "\n";
  if (LEVEL >= 2)
    text += String("\nValue    ") + String(oecm_sensor_value(), 2) + " C\nSetpoint " +
            String(oecm_sensor_setpoint(), 1) + " C\n";
  status(text);
}
