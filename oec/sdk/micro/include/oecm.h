/*
 * oecm.h - API of the OEC Micro SDK (D9) for microcontroller-class smart
 * devices: C17, portable core + platform abstraction layer (PAL), static
 * memory (ADR-0016, ADR-0017, ADR-0022, ADR-0030).
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
 *  - Messaging Level 1 (Birth/Will/Close, Pub MAM + Health); Level 2/3
 *    (Get, Set/Del, Data, Config, lists, NewDataSetWriterId) are additions.
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
 * core (ADR-0022 §3). Level 1 uses 368 octets on a 64-bit host (C-092);
 * the rest is reserved for Level 2/3. */
#define OECM_DEVICE_SIZE 1024u
/* Minimum transmit buffer. The buffer holds the topic, a NUL and the
 * payload of one message. 2048 octets hold every Health, Will and Close,
 * also for an Identification Link of 255 octets (1804 octets). The Birth
 * (MAM) grows with the MAM texts: oecm_init encodes it once and fails with
 * OECM_E_NOSPACE if the buffer is too small for the Identification Link and
 * the MAM; a typical Birth needs about 1000 octets. */
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

/* MAM of the device (REQ-RES-MAM-001); borrowed, UTF-8. ProductInstanceUri
 * is the Identification Link (REQ-RES-MAM-016). Other NULL strings are
 * sent as the empty string. */
typedef struct oecm_mam {
    const char *manufacturer;      /* LocalizedText, locale en-US */
    const char *manufacturer_uri;  /* REQ-RES-MAM-004 */
    const char *model;             /* LocalizedText, locale en-US (REQ-RES-MAM-024) */
    const char *product_code;
    const char *hardware_revision; /* NULL: "undefined" (REQ-RES-MAM-008) */
    const char *software_revision; /* firmware version; NULL: "undefined" (REQ-RES-MAM-010) */
    const char *device_revision;
    const char *device_manual;     /* URL (REQ-RES-MAM-012) */
    const char *device_class;      /* class of the product, not Oi4.<ServiceType> */
    const char *serial_number;     /* e.g. from the MAC address (REQ-RES-MAM-015) */
    const char *description;       /* optional, NULL: omitted */
    int32_t revision_counter;      /* -1 without counter (REQ-RES-MAM-018) */
    const char *global_asset_id;   /* optional AAS globalAssetId, NULL: omitted (REQ-RES-MAM-023) */
} oecm_mam;

typedef struct oecm_config {
    oecm_level level;
    /* A directly connected smart device uses OECM_ST_OT_CONNECTOR
     * (REQ-TOP-028, MG 7.2, SF-102; ADR-0022). */
    oecm_service_type service_type;
    /* NULL: the Oi4Identifier is built from the MAM (REQ-IL-007,
     * REQ-RES-MAM-006); else an existing Identification Link (REQ-IL-006). */
    const char *identification_link;
    oecm_mam mam;
    /* MQTT ClientId; NULL: "oec" + 16 hex digits of the FNV-1a hash of the
     * Identification Link, stable across reconnects (REQ-MQTT-013), as the
     * Rust SDK derives it. */
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

/* Drives timers: Health heartbeat every 60 s (REQ-TOP-017), reconnect
 * with backoff (1 s doubling to 30 s), connect timeout (30 s). OECM_E_PAL
 * if a publication failed; it is retried on the next call. */
oecm_err oecm_poll(oecm_device *dev);
/* Milliseconds until oecm_poll has work, at most 1000. */
uint32_t oecm_poll_interval_ms(const oecm_device *dev);

/* PAL → core: connection established. The core publishes the Birth as
 * first message (REQ-LIFE-001, REQ-LIFE-015) and the Health. */
void oecm_on_connected(oecm_device *dev);
/* PAL → core: connection lost or closed. */
void oecm_on_disconnected(oecm_device *dev);
/* PAL → core: received message (Level 2+; ignored at Level 1). */
void oecm_on_message(oecm_device *dev, const char *topic, const uint8_t *payload, size_t len);

/* score < 0: no HealthScore; 0 … 100 otherwise (REQ-RES-HEALTH-001).
 * NORMAL_0 with score 0 is reserved for the Close (REQ-LIFE-014):
 * OECM_E_ARG. The next oecm_poll publishes a change (REQ-TOP-017). A
 * recovery from FAILURE_1/0 publishes the Birth again (REQ-LIFE-017). */
oecm_err oecm_set_health(oecm_device *dev, oecm_health health, int score);

/* The Identification Link in use (unencoded, REQ-IL-005); borrowed. */
const char *oecm_identification_link(const oecm_device *dev);

#ifdef __cplusplus
}
#endif

#endif /* OECM_H */
