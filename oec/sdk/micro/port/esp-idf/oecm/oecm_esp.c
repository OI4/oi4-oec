/*
 * oecm_esp.c - the ESP-IDF PAL of the OEC Micro SDK (C-096). See
 * oecm_esp.h. The port may use the heap (the core may not, ADR-0022 §2):
 * esp-mqtt allocates anyway; the port allocates its state, the reassembly
 * buffer and the ring buffer once in oecm_esp_init.
 */
#include "oecm_esp.h"

#include <limits.h>
#include <stdatomic.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>

#include "esp_log.h"
#include "esp_timer.h"
#include "freertos/FreeRTOS.h"
#include "freertos/ringbuf.h"
#include "mqtt_client.h"
#include "nvs.h"

static const char *TAG = "oecm";

enum { EV_CONNECTED = 1, EV_DISCONNECTED = 2, EV_MESSAGE = 3 };

/* Header of a ring buffer item; the topic, a NUL and the payload follow. */
typedef struct item {
    uint32_t gen;
    uint32_t len;
    uint16_t topic_len;
    uint8_t kind;
} item;

struct oecm_esp {
    oecm_esp_config cfg;
    esp_mqtt_client_handle_t client;
    bool running; /* the MQTT task of the client exists */
    RingbufHandle_t rb;
    /* Connection attempt; an event of another attempt is dropped. Written
     * by the application task, read by the MQTT task. */
    _Atomic uint32_t gen;
    nvs_handle_t nvs;
    /* Reassembly of a fragmented message (MQTT task only). */
    uint8_t *rx;
    size_t rx_topic, rx_len, rx_have;
    bool rx_busy, rx_drop;
};

/* Queues an event (MQTT task, or the application task while esp-mqtt
 * aborts a connection inside a publish). */
static void push(oecm_esp *p, uint8_t kind, const char *topic, size_t tlen, const uint8_t *data,
                 size_t len) {
    size_t size = sizeof(item) + tlen + 1 + len;
    void *mem = NULL;
    TickType_t wait = pdMS_TO_TICKS(kind == EV_MESSAGE ? 100 : 1000);
    if (xRingbufferSendAcquire(p->rb, &mem, size, wait) != pdTRUE) {
        ESP_LOGE(TAG, "event queue full: event %u dropped", kind);
        return;
    }
    item h = {.gen = atomic_load(&p->gen), .len = (uint32_t)len, .topic_len = (uint16_t)tlen, .kind = kind};
    uint8_t *m = mem;
    memcpy(m, &h, sizeof h);
    if (tlen)
        memcpy(m + sizeof h, topic, tlen);
    m[sizeof h + tlen] = 0;
    if (len)
        memcpy(m + sizeof h + tlen + 1, data, len);
    xRingbufferSendComplete(p->rb, mem);
}

static void on_data(oecm_esp *p, const esp_mqtt_event_t *e) {
    size_t total = e->total_data_len > 0 ? (size_t)e->total_data_len : 0;
    size_t chunk = e->data_len > 0 ? (size_t)e->data_len : 0;
    if (e->current_data_offset == 0) {
        size_t tlen = e->topic_len > 0 ? (size_t)e->topic_len : 0;
        p->rx_busy = false;
        if (tlen == 0 || tlen > UINT16_MAX || tlen + 1 + total > p->cfg.rx_size) {
            ESP_LOGW(TAG, "received message of %u octets dropped (rx_size %u)", (unsigned)(tlen + 1 + total),
                     (unsigned)p->cfg.rx_size);
            p->rx_drop = chunk < total;
            return;
        }
        p->rx_drop = false;
        if (chunk >= total) { /* the whole message in one event */
            push(p, EV_MESSAGE, e->topic, tlen, (const uint8_t *)e->data, total);
            return;
        }
        memcpy(p->rx, e->topic, tlen);
        p->rx_topic = tlen;
        p->rx_len = total;
        p->rx_have = 0;
        p->rx_busy = true;
    } else if (p->rx_drop || !p->rx_busy) {
        return;
    }
    if (p->rx_have + chunk > p->rx_len) {
        p->rx_busy = false;
        return;
    }
    memcpy(p->rx + p->rx_topic + p->rx_have, e->data, chunk);
    p->rx_have += chunk;
    if (p->rx_have == p->rx_len) {
        p->rx_busy = false;
        push(p, EV_MESSAGE, (const char *)p->rx, p->rx_topic, p->rx + p->rx_topic, p->rx_len);
    }
}

static void on_event(void *arg, esp_event_base_t base, int32_t id, void *data) {
    (void)base;
    oecm_esp *p = arg;
    esp_mqtt_event_handle_t e = data;
    switch ((esp_mqtt_event_id_t)id) {
    case MQTT_EVENT_CONNECTED:
        push(p, EV_CONNECTED, NULL, 0, NULL, 0);
        break;
    case MQTT_EVENT_DISCONNECTED:
        p->rx_busy = false;
        push(p, EV_DISCONNECTED, NULL, 0, NULL, 0);
        break;
    case MQTT_EVENT_DATA:
        on_data(p, e);
        break;
    case MQTT_EVENT_ERROR:
        if (e->error_handle->error_type == MQTT_ERROR_TYPE_TCP_TRANSPORT)
            ESP_LOGW(TAG, "transport error: esp-tls 0x%x, TLS stack 0x%x, socket errno %d",
                     e->error_handle->esp_tls_last_esp_err, e->error_handle->esp_tls_stack_err,
                     e->error_handle->esp_transport_sock_errno);
        else if (e->error_handle->error_type == MQTT_ERROR_TYPE_CONNECTION_REFUSED)
            ESP_LOGW(TAG, "connection refused: return code %d", e->error_handle->connect_return_code);
        break;
    default:
        break;
    }
}

static oecm_err p_connect(void *ctx, const oecm_connect *o) {
    oecm_esp *p = ctx;
    if (o->will_len > INT_MAX)
        return OECM_E_ARG;
    esp_mqtt_client_config_t c = {0};
    c.broker.address.uri = p->cfg.uri;
    c.broker.verification.certificate = p->cfg.ca_pem;
    c.credentials.client_id = o->client_id;
    if (p->cfg.client_cert_pem) {
        c.credentials.authentication.certificate = p->cfg.client_cert_pem;
        c.credentials.authentication.key = p->cfg.client_key_pem;
    } else {
        c.credentials.username = p->cfg.username;
        c.credentials.authentication.password = p->cfg.password;
    }
    /* A new Will for every connect, QoS 0, not retained (REQ-LIFE-008,
     * REQ-MQTT-015); clean session (REQ-MQTT-018). */
    c.session.last_will.topic = o->will_topic;
    c.session.last_will.msg = (const char *)o->will_payload;
    c.session.last_will.msg_len = (int)o->will_len;
    c.session.keepalive = o->keep_alive_s;
    c.session.protocol_ver = MQTT_PROTOCOL_V_3_1_1;
    c.network.disable_auto_reconnect = true;
    if (!p->client) {
        p->client = esp_mqtt_client_init(&c);
        if (!p->client)
            return OECM_E_PAL;
        if (esp_mqtt_client_register_event(p->client, ESP_EVENT_ANY_ID, on_event, p) != ESP_OK)
            return OECM_E_PAL;
    } else if (esp_mqtt_set_config(p->client, &c) != ESP_OK) {
        return OECM_E_PAL;
    }
    atomic_fetch_add(&p->gen, 1);
    /* After a lost or failed connection the client waits for a reconnect;
     * otherwise (first connect, after mqtt_disconnect) its task is started. */
    if (p->running && esp_mqtt_client_reconnect(p->client) == ESP_OK)
        return OECM_OK;
    if (p->running) {
        (void)esp_mqtt_client_stop(p->client);
        atomic_fetch_add(&p->gen, 1);
    }
    p->running = esp_mqtt_client_start(p->client) == ESP_OK;
    return p->running ? OECM_OK : OECM_E_PAL;
}

static oecm_err p_publish(void *ctx, const char *topic, const uint8_t *payload, size_t len) {
    oecm_esp *p = ctx;
    if (!p->client)
        return OECM_E_PAL;
    if (len > INT_MAX)
        return OECM_E_ARG;
    return esp_mqtt_client_publish(p->client, topic, (const char *)payload, (int)len, 0, 0) < 0 ? OECM_E_PAL
                                                                                                : OECM_OK;
}

static oecm_err p_subscribe(void *ctx, const char *filter) {
    oecm_esp *p = ctx;
    if (!p->client)
        return OECM_E_PAL;
    return esp_mqtt_client_subscribe_single(p->client, filter, 0) < 0 ? OECM_E_PAL : OECM_OK;
}

/* Sends DISCONNECT if connected (no Will) and ends the MQTT task; later
 * events of the connection are dropped. */
static oecm_err p_disconnect(void *ctx) {
    oecm_esp *p = ctx;
    if (p->client && p->running) {
        (void)esp_mqtt_client_stop(p->client);
        p->running = false;
    }
    atomic_fetch_add(&p->gen, 1);
    return OECM_OK;
}

static int64_t p_unix_ms(void *ctx) {
    (void)ctx;
    struct timeval tv;
    gettimeofday(&tv, NULL);
    return (int64_t)tv.tv_sec * 1000 + tv.tv_usec / 1000;
}

static uint32_t p_mono_ms(void *ctx) {
    (void)ctx;
    return (uint32_t)(esp_timer_get_time() / 1000);
}

static oecm_err p_load(void *ctx, const char *key, uint8_t *buf, size_t cap, size_t *len) {
    oecm_esp *p = ctx;
    size_t n = cap;
    esp_err_t e = nvs_get_blob(p->nvs, key, buf, &n);
    if (e == ESP_ERR_NVS_NOT_FOUND) {
        *len = 0;
        return OECM_OK;
    }
    if (e != ESP_OK) {
        ESP_LOGE(TAG, "nvs_get_blob %s: %s", key, esp_err_to_name(e));
        return OECM_E_PAL;
    }
    *len = n;
    return OECM_OK;
}

static oecm_err p_save(void *ctx, const char *key, const uint8_t *buf, size_t len) {
    oecm_esp *p = ctx;
    esp_err_t e = len == 0 ? nvs_erase_key(p->nvs, key) : nvs_set_blob(p->nvs, key, buf, len);
    if (e == ESP_ERR_NVS_NOT_FOUND && len == 0)
        e = ESP_OK;
    if (e == ESP_OK)
        e = nvs_commit(p->nvs);
    if (e != ESP_OK) {
        ESP_LOGE(TAG, "nvs save %s: %s", key, esp_err_to_name(e));
        return OECM_E_PAL;
    }
    return OECM_OK;
}

esp_err_t oecm_esp_init(const oecm_esp_config *cfg, oecm_esp **port, oecm_pal *pal) {
    if (!cfg || !port || !pal || !cfg->uri || strncmp(cfg->uri, "mqtts://", 8) != 0 || !cfg->ca_pem ||
        (cfg->client_cert_pem == NULL) != (cfg->client_key_pem == NULL) ||
        (!cfg->client_cert_pem && !cfg->username))
        return ESP_ERR_INVALID_ARG;
    oecm_esp *p = calloc(1, sizeof *p);
    if (!p)
        return ESP_ERR_NO_MEM;
    p->cfg = *cfg;
    if (p->cfg.rx_size == 0)
        p->cfg.rx_size = 8192;
    if (!p->cfg.nvs_namespace)
        p->cfg.nvs_namespace = "oecm";
    p->rx = malloc(p->cfg.rx_size);
    p->rb = xRingbufferCreate(2 * (p->cfg.rx_size + sizeof(item) + 8), RINGBUF_TYPE_NOSPLIT);
    if (!p->rx || !p->rb) {
        if (p->rb)
            vRingbufferDelete(p->rb);
        free(p->rx);
        free(p);
        return ESP_ERR_NO_MEM;
    }
    /* Without NVS the persistence stays NULL: Level 3 then fails in
     * oecm_init with OECM_E_ARG. */
    esp_err_t e = nvs_open(p->cfg.nvs_namespace, NVS_READWRITE, &p->nvs);
    if (e != ESP_OK)
        ESP_LOGW(TAG, "nvs_open %s: %s (no persistence)", p->cfg.nvs_namespace, esp_err_to_name(e));
    *pal = (oecm_pal){
        .ctx = p,
        .mqtt_connect = p_connect,
        .mqtt_publish = p_publish,
        .mqtt_subscribe = p_subscribe,
        .mqtt_disconnect = p_disconnect,
        .unix_time_ms = p_unix_ms,
        .monotonic_ms = p_mono_ms,
        .persist_load = e == ESP_OK ? p_load : NULL,
        .persist_save = e == ESP_OK ? p_save : NULL,
    };
    *port = p;
    return ESP_OK;
}

void oecm_esp_dispatch(oecm_esp *p, oecm_device *dev, uint32_t wait_ms) {
    TickType_t wait = (wait_ms + portTICK_PERIOD_MS - 1) / portTICK_PERIOD_MS;
    for (;;) {
        size_t size = 0;
        uint8_t *m = xRingbufferReceive(p->rb, &size, wait);
        if (!m)
            return;
        wait = 0;
        item h;
        memcpy(&h, m, sizeof h);
        if (h.gen == atomic_load(&p->gen)) {
            switch (h.kind) {
            case EV_CONNECTED:
                oecm_on_connected(dev);
                break;
            case EV_DISCONNECTED:
                oecm_on_disconnected(dev);
                break;
            case EV_MESSAGE:
                oecm_on_message(dev, (const char *)m + sizeof h, m + sizeof h + h.topic_len + 1, h.len);
                break;
            default:
                break;
            }
        }
        vRingbufferReturnItem(p->rb, m);
    }
}
