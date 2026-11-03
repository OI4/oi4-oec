/*
 * oecm_esp.h - the ESP-IDF port of the OEC Micro SDK (ADR-0022 §7,
 * ADR-0024, ADR-0030 §8, C-096): the PAL on esp-mqtt (MQTT 3.1.1 over
 * esp-tls/mbedTLS), the system time (SNTP is the application's duty) and
 * NVS for the persisted records of Level 3.
 *
 * Threading: esp-mqtt delivers its events on its own task. The port copies
 * them into a ring buffer; oecm_esp_dispatch hands them to the device on
 * the application task, the task that calls every oecm_ function
 * (ADR-0022 §2). A typical loop:
 *
 *     for (;;) {
 *         oecm_esp_dispatch(port, &dev, oecm_poll_interval_ms(&dev));
 *         oecm_poll(&dev);
 *     }
 *
 * Will per connect (REQ-LIFE-008, REQ-LIFE-016): the automatic reconnect of
 * esp-mqtt is disabled; every mqtt_connect of the core sets the new Will
 * with esp_mqtt_set_config before esp_mqtt_client_reconnect (or start).
 * Events of an earlier connection attempt are dropped (generation).
 */
#ifndef OECM_ESP_H
#define OECM_ESP_H

#include <stddef.h>

#include "esp_err.h"
#include "oecm.h"

#ifdef __cplusplus
extern "C" {
#endif

/* Connection settings. Strings are borrowed for the port lifetime. */
typedef struct oecm_esp_config {
    /* "mqtts://host:port": TLS only (REQ-MQTT-005); other schemes are
     * ESP_ERR_INVALID_ARG. */
    const char *uri;
    /* PEM of the CA that signed the broker certificate (REQ-MQTT-008); the
     * certificate bundle of ESP-IDF is never used (ADR-0024). Required. */
    const char *ca_pem;
    /* Username/password (REQ-MQTT-009) or a client certificate with its
     * key (PEM); a client certificate wins (REQ-MQTT-010). */
    const char *username;
    const char *password;
    const char *client_cert_pem;
    const char *client_key_pem;
    /* NVS namespace of the persisted records in the default NVS partition
     * (NULL: "oecm"); nvs_flash_init is the application's duty. */
    const char *nvs_namespace;
    /* The largest received message (topic + payload); larger ones are
     * dropped with a log line. 0: 8192. */
    size_t rx_size;
} oecm_esp_config;

typedef struct oecm_esp oecm_esp;

/* Allocates the port and fills pal (all functions; ctx is the port). The
 * MQTT client is created on the first mqtt_connect of the core. */
esp_err_t oecm_esp_init(const oecm_esp_config *cfg, oecm_esp **port, oecm_pal *pal);

/* Waits up to wait_ms for MQTT events and hands every queued one to dev
 * (oecm_on_connected, oecm_on_disconnected, oecm_on_message). Call it on
 * the application task only. */
void oecm_esp_dispatch(oecm_esp *port, oecm_device *dev, uint32_t wait_ms);

#ifdef __cplusplus
}
#endif

#endif /* OECM_ESP_H */
