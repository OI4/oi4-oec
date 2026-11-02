/* Level 1 smart device on the micro SDK (ADR-0022, ADR-0030): the shape of
 * the ESP32 example firmware, with a stub PAL that prints instead of
 * ESP-IDF. Built and run by micro/check.sh. */
#include <stdio.h>

#include "oecm.h"

/* --- stub PAL (the ESP-IDF port maps these to esp-mqtt/esp-tls/SNTP) --- */
static oecm_err pal_connect(void *ctx, const oecm_connect *o) {
    (void)ctx;
    printf("connect %s, will %s\n", o->client_id, o->will_topic);
    return OECM_OK;
}
static oecm_err pal_publish(void *ctx, const char *topic, const uint8_t *p, size_t n) {
    (void)ctx;
    printf("pub %s %.*s\n", topic, (int)n, (const char *)p);
    return OECM_OK;
}
static oecm_err pal_disconnect(void *ctx) {
    (void)ctx;
    return OECM_OK;
}
static int64_t pal_unix_ms(void *ctx) {
    (void)ctx;
    return 1793000000000;
}
static uint32_t pal_mono_ms(void *ctx) {
    (void)ctx;
    return 0;
}

/* --- static memory ----------------------------------------------------- */
static oecm_device device;
static uint8_t tx_buf[OECM_TX_MIN];

static const oecm_pal pal = {
    .mqtt_connect = pal_connect,
    .mqtt_publish = pal_publish,
    .mqtt_disconnect = pal_disconnect,
    .unix_time_ms = pal_unix_ms,
    .monotonic_ms = pal_mono_ms,
};

static const oecm_config config = {
    .level = OECM_LEVEL_1,
    .service_type = OECM_ST_OT_CONNECTOR,
    .identification_link = NULL, /* → Oi4Identifier from the MAM */
    .mam = {
        .manufacturer = "Example Inc.",
        .manufacturer_uri = "example.com",
        .model = "Climate Sensor",
        .product_code = "CS-32",
        .hardware_revision = "1",
        .software_revision = "0.1.0",
        .device_revision = "",
        .device_manual = "https://example.com/cs32/manual.pdf",
        .device_class = "Sensor",
        .serial_number = "24A160112233", /* from the MAC address */
        .revision_counter = -1,
    },
};

int main(void) {
    oecm_err err = oecm_init(&device, &config, &pal, tx_buf, sizeof tx_buf);
    if (err != OECM_OK) {
        printf("init: %s\n", oecm_err_str(err));
        return 1;
    }
    printf("IL %s\n", oecm_identification_link(&device));
    if ((err = oecm_start(&device)) != OECM_OK) {
        printf("start: %s\n", oecm_err_str(err));
        return 1;
    }
    oecm_on_connected(&device); /* normally queued from the MQTT event task */
    oecm_set_health(&device, OECM_HEALTH_OFF_SPEC, 70);
    for (int i = 0; i < 3; i++) /* firmware: for (;;) with a task delay */
        oecm_poll(&device);
    return oecm_stop(&device) == OECM_OK ? 0 : 1;
}
