/*
 * oecm.h - API of the OEC Micro SDK (D9) for microcontroller-class smart
 * devices: C17, portable core + platform abstraction layer (PAL), static
 * memory (ADR-0016, ADR-0017).
 *
 * P3 SKETCH (C-042, ADR-0022) - NOT PRODUCT CODE. Compile-checked by
 * oec/check.sh with -std=c17 -Wall -Wextra -Wpedantic -Werror.
 *
 * Contract (ADR-0022):
 *  - Prefix oecm_ / OECM_. Same names as oec.h (D2 C ABI) where the concept
 *    is the same (level, health, service type, resource); deliberately no
 *    shared code (ADR-0017).
 *  - No heap: the caller provides the device state (oecm_device, opaque,
 *    fixed size) and the transmit buffer; the core never calls malloc.
 *  - Borrowed configuration: all strings in oecm_config must stay valid
 *    and unchanged as long as the device is used (typically string
 *    literals or static buffers); the core does not copy them.
 *  - Single-threaded: every oecm_ function, including the oecm_on_*
 *    entry points called by the PAL, runs on the same task/loop. A port
 *    whose MQTT client delivers events on another task (e.g. esp-mqtt)
 *    queues them and calls oecm_on_* from the application task.
 *  - Event loop: oecm_poll() drives timers (Health heartbeat, reconnect
 *    backoff); call it at least every oecm_poll_interval_ms().
 *  - TLS is the duty of the PAL: its MQTT connection is always TLS >= 1.2
 *    with broker certificate verification and user/password or client
 *    certificate (REQ-MQTT-005, REQ-MQTT-008, REQ-MQTT-009). The core has
 *    no API for a plain connection.
 *  - Increment 1 implements Messaging Level 1 (Birth/Will/Close, Pub MAM +
 *    Health); the API is designed so that Level 2/3 (Get, Set/Del, Data,
 *    Config, lists, NewDataSetWriterId) are additions, not changes.
 */
#ifndef OECM_H
#define OECM_H

#include <stdalign.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define OECM_API_VERSION 1u

/* Size of the opaque device state; checked by a static assertion in the
 * core. Value to be measured in P4b. */
#define OECM_DEVICE_SIZE 1024u
/* Minimum transmit buffer: the largest message of Level 1 is the Birth
 * (MAM) with a 255-character Identification Link. */
#define OECM_TX_MIN 2048u

typedef enum oecm_err {
    OECM_OK = 0,
    OECM_E_ARG = 1,      /* invalid argument, e.g. Identification Link (REQ-IL-008) */
    OECM_E_NOSPACE = 2,  /* transmit buffer or device state too small */
    OECM_E_STATE = 3,    /* call not allowed in this state */
    OECM_E_PAL = 4,      /* a PAL function failed */
    OECM_E_LEVEL = 5     /* feature not available at the configured level */
} oecm_err;

const char *oecm_err_str(oecm_err err);

typedef enum oecm_level { OECM_LEVEL_1 = 1, OECM_LEVEL_2 = 2, OECM_LEVEL_3 = 3 } oecm_level;

typedef enum oecm_service_type { /* REQ-TOP-005, same order as oec.h */
    OECM_ST_REGISTRY,
    OECM_ST_OT_CONNECTOR,
    OECM_ST_UTILITY,
    OECM_ST_PERSISTENCE,
    OECM_ST_AGGREGATION,
    OECM_ST_OOC_CONNECTOR,
    OECM_ST_IT_CONNECTOR
} oecm_service_type;

typedef enum oecm_health { /* MG 8.3.2, same order as oec.h */
    OECM_HEALTH_NORMAL,
    OECM_HEALTH_FAILURE,
    OECM_HEALTH_CHECK_FUNCTION,
    OECM_HEALTH_OFF_SPEC,
    OECM_HEALTH_MAINTENANCE_REQUIRED
} oecm_health;

/* MAM of the device (REQ-RES-MAM-001); borrowed. ProductInstanceUri is the
 * Identification Link (REQ-RES-MAM-016). */
typedef struct oecm_mam {
    const char *manufacturer;      /* LocalizedText, locale en-US */
    const char *manufacturer_uri;  /* REQ-RES-MAM-004 */
    const char *model;             /* LocalizedText, locale en-US (REQ-RES-MAM-024) */
    const char *product_code;
    const char *hardware_revision; /* "undefined" if unknown (REQ-RES-MAM-008) */
    const char *software_revision; /* firmware version (REQ-RES-MAM-010) */
    const char *device_revision;
    const char *device_manual;     /* URL (REQ-RES-MAM-012) */
    const char *device_class;      /* class of the product, not Oi4.<ServiceType> */
    const char *serial_number;     /* e.g. from the MAC address (REQ-RES-MAM-015) */
    const char *description;       /* optional, NULL */
    int32_t revision_counter;      /* -1 without counter (REQ-RES-MAM-018) */
} oecm_mam;

typedef struct oecm_config {
    oecm_level level;
    /* A directly connected device uses OECM_ST_OT_CONNECTOR until SF-102
     * is decided (ADR-0022). */
    oecm_service_type service_type;
    /* NULL: the Oi4Identifier is built from the MAM (REQ-IL-007,
     * REQ-RES-MAM-006); else an existing Identification Link (REQ-IL-006). */
    const char *identification_link;
    oecm_mam mam;
    /* MQTT ClientId; NULL: derived from the Identification Link, stable
     * across reconnects (REQ-MQTT-013). */
    const char *client_id;
    uint16_t keep_alive_s;    /* 0 → 60 (REQ-MQTT-014) */
    uint32_t max_packet_size; /* 0 → 262144 */
} oecm_config;

/* Connect options handed to the PAL. The Will is created anew for every
 * connect (REQ-LIFE-008, REQ-LIFE-016): a port must not reuse an old Will
 * on an automatic reconnect of its MQTT client. Clean session true, Will
 * QoS 0 and retain false are fixed (REQ-MQTT-015, REQ-MQTT-018). */
typedef struct oecm_connect {
    const char *client_id;
    uint16_t keep_alive_s;
    const char *will_topic;
    const uint8_t *will_payload;
    size_t will_len;
} oecm_connect;

/* Platform abstraction layer. ctx is passed to every function. Optional
 * functions may be NULL. */
typedef struct oecm_pal {
    void *ctx;
    /* Starts a TLS MQTT 3.1.1 connection; completion is reported with
     * oecm_on_connected / oecm_on_disconnected. */
    oecm_err (*mqtt_connect)(void *ctx, const oecm_connect *opts);
    /* QoS 0, retain false (REQ-MQTT-017, REQ-MQTT-018); payload borrowed
     * for the call. */
    oecm_err (*mqtt_publish)(void *ctx, const char *topic, const uint8_t *payload, size_t len);
    /* Level 2+ only; may be NULL at Level 1. */
    oecm_err (*mqtt_subscribe)(void *ctx, const char *topic_filter);
    oecm_err (*mqtt_disconnect)(void *ctx);
    /* Wall clock in Unix ms (MessageId, Timestamp, REQ-MSG-007); a device
     * needs a time source (e.g. SNTP) before oecm_start. */
    int64_t (*unix_time_ms)(void *ctx);
    /* Monotonic ms for timers. */
    uint32_t (*monotonic_ms)(void *ctx);
    /* Optional persistence (Level 3: ReferenceDesignation, Config). */
    oecm_err (*persist_load)(void *ctx, const char *key, uint8_t *buf, size_t cap, size_t *len);
    oecm_err (*persist_save)(void *ctx, const char *key, const uint8_t *buf, size_t len);
} oecm_pal;

/* Opaque device state, provided by the caller (static or on a task stack). */
typedef struct oecm_device {
    alignas(max_align_t) unsigned char opaque[OECM_DEVICE_SIZE];
} oecm_device;

/* Validates the configuration (Identification Link, REQ-IL-008/016/019;
 * level; buffer sizes). cfg and pal are borrowed for the device lifetime.
 * tx_buf (>= OECM_TX_MIN) is used to encode every outgoing message. */
oecm_err oecm_init(oecm_device *dev, const oecm_config *cfg, const oecm_pal *pal,
                   uint8_t *tx_buf, size_t tx_cap);

/* Connects via the PAL; reconnects with backoff after a disconnect.
 * OECM_E_STATE while unix_time_ms has no plausible wall clock yet (the
 * MessageId carries the time, REQ-MSG-007). */
oecm_err oecm_start(oecm_device *dev);

/* Publishes Close (REQ-LIFE-005) and disconnects. */
oecm_err oecm_stop(oecm_device *dev);

/* Drives timers: Health heartbeat every 60 s (REQ-TOP-017), reconnect. */
oecm_err oecm_poll(oecm_device *dev);
uint32_t oecm_poll_interval_ms(const oecm_device *dev);

/* PAL → core: connection established. The core publishes the Birth as
 * first message (REQ-LIFE-001, REQ-LIFE-015) and the Health. */
void oecm_on_connected(oecm_device *dev);
/* PAL → core: connection lost or closed. */
void oecm_on_disconnected(oecm_device *dev);
/* PAL → core: received message (Level 2+; ignored at Level 1). */
void oecm_on_message(oecm_device *dev, const char *topic, const uint8_t *payload, size_t len);

/* score < 0: no HealthScore. Publishes on change (REQ-TOP-017). A recovery
 * from FAILURE_1/0 publishes the Birth again (REQ-LIFE-017). */
oecm_err oecm_set_health(oecm_device *dev, oecm_health health, int score);

/* The Identification Link in use (unencoded, REQ-IL-005); borrowed. */
const char *oecm_identification_link(const oecm_device *dev);

#ifdef __cplusplus
}
#endif

#endif /* OECM_H */
