/*
 * Device lifecycle of Messaging Level 1 (MG 6.3, MG 9.1.1, MG 9.1.2): Will
 * per connect, Birth first after every connect and after a recovery, Health
 * at connect, on change and every 60 s, Close before a graceful disconnect,
 * reconnect with backoff. Same behavior as oec-core lifecycle.rs.
 */
#include <string.h>

#include "oecm_internal.h"

_Static_assert(sizeof(oecm_state) <= OECM_DEVICE_SIZE, "oecm_state does not fit OECM_DEVICE_SIZE");
_Static_assert(_Alignof(oecm_state) <= _Alignof(oecm_device), "oecm_device alignment too small");

#define HEARTBEAT_MS 60000u
#define BACKOFF_MIN_MS 1000u
#define BACKOFF_MAX_MS 30000u
#define CONNECT_TIMEOUT_MS 30000u
#define POLL_MAX_MS 1000u
/* 2020-01-01T00:00:00Z: an earlier wall clock is not set (SNTP). */
#define PLAUSIBLE_UNIX_MS 1577836800000

enum { PH_IDLE = 1, PH_CONNECTING, PH_CONNECTED, PH_BACKOFF, PH_STOPPED };

static oecm_state *state(oecm_device *dev) { return (oecm_state *)(void *)dev->opaque; }
static const oecm_state *cstate(const oecm_device *dev) {
    return (const oecm_state *)(const void *)dev->opaque;
}

static bool ready(const oecm_device *dev) { return dev != NULL && cstate(dev)->phase != 0; }

const char *oecm_err_str(oecm_err err) {
    switch (err) {
    case OECM_OK:
        return "ok";
    case OECM_E_ARG:
        return "invalid argument";
    case OECM_E_NOSPACE:
        return "buffer or device state too small";
    case OECM_E_STATE:
        return "call not allowed in this state";
    case OECM_E_PAL:
        return "platform (PAL) function failed";
    case OECM_E_LEVEL:
        return "not available at the configured level";
    }
    return "unknown error";
}

const char *oecm_service_type_name(oecm_service_type st) {
    switch (st) {
    case OECM_ST_REGISTRY:
        return "Registry";
    case OECM_ST_OT_CONNECTOR:
        return "OTConnector";
    case OECM_ST_UTILITY:
        return "Utility";
    case OECM_ST_PERSISTENCE:
        return "Persistence";
    case OECM_ST_AGGREGATION:
        return "Aggregation";
    case OECM_ST_OOC_CONNECTOR:
        return "OOCConnector";
    case OECM_ST_IT_CONNECTOR:
        return "ITConnector";
    }
    return NULL;
}

const char *oecm_health_name(oecm_health h) {
    switch (h) {
    case OECM_HEALTH_NORMAL:
        return "NORMAL_0";
    case OECM_HEALTH_FAILURE:
        return "FAILURE_1";
    case OECM_HEALTH_CHECK_FUNCTION:
        return "CHECK_FUNCTION_2";
    case OECM_HEALTH_OFF_SPEC:
        return "OFF_SPEC_3";
    case OECM_HEALTH_MAINTENANCE_REQUIRED:
        return "MAINTENANCE_REQUIRED_4";
    }
    return NULL;
}

uint64_t oecm_fnv1a64(const char *s, size_t n) {
    uint64_t h = 0xcbf29ce484222325u;
    for (size_t i = 0; i < n; i++) {
        h ^= (unsigned char)s[i];
        h *= 0x00000100000001b3u;
    }
    return h;
}

/* --- Encoding ---------------------------------------------------------------- */

static void put_json_cstr(oecm_buf *b, const char *s) { oecm_put_json_str(b, s, s ? strlen(s) : 0); }

static void put_localized(oecm_buf *b, const char *text) {
    oecm_puts(b, "{\"Locale\":\"en-US\",\"Text\":");
    put_json_cstr(b, text);
    oecm_putc(b, '}');
}

/* Topic "Oi4v2/<ST>/<AppId>/Pub/<resource>/<AppId>", then a NUL. */
static void put_own_topic(oecm_buf *b, const oecm_state *s, const char *resource) {
    oecm_puts(b, "Oi4v2/");
    oecm_puts(b, oecm_service_type_name(s->cfg->service_type));
    oecm_putc(b, '/');
    oecm_put_b64url(b, (const uint8_t *)s->il, s->il_len);
    oecm_puts(b, "/Pub/");
    oecm_puts(b, resource);
    oecm_putc(b, '/');
    oecm_put_b64url(b, (const uint8_t *)s->il, s->il_len);
    oecm_putc(b, '\0');
}

/* The next MessageId (REQ-MSG-007, REQ-MSG-008): the Unix time in ms, a
 * counter appended while the clock does not advance. */
static void next_message_id(oecm_state *s, int64_t unix_ms) {
    uint64_t ms = unix_ms > 0 ? (uint64_t)unix_ms : 0u;
    if (ms > s->id_ms) {
        s->id_ms = ms;
        s->id_counter = 0;
    } else if (s->id_counter < UINT32_MAX) {
        s->id_counter++;
    } else {
        s->id_ms++;
        s->id_counter = 0;
    }
}

static void put_publisher(oecm_buf *b, const oecm_state *s) {
    oecm_puts(b, oecm_service_type_name(s->cfg->service_type));
    oecm_putc(b, '/');
    oecm_put(b, s->il, s->il_len);
}

/* NetworkMessage up to the Payload of its single DataSetMessage. */
static void put_head(oecm_buf *b, oecm_state *s, int64_t unix_ms, const char *class_id,
                     unsigned writer) {
    next_message_id(s, unix_ms);
    oecm_puts(b, "{\"MessageId\":\"");
    oecm_put_u64(b, s->id_ms);
    if (s->id_counter != 0)
        oecm_put_u64(b, s->id_counter);
    oecm_putc(b, '-');
    put_publisher(b, s);
    oecm_puts(b, "\",\"MessageType\":\"ua-data\",\"PublisherId\":\"");
    put_publisher(b, s);
    oecm_puts(b, "\",\"DataSetClassId\":\"");
    oecm_puts(b, class_id);
    oecm_puts(b, "\",\"ReplyTo\":\"\",\"Messages\":[{\"DataSetWriterId\":");
    oecm_put_u64(b, writer);
    oecm_puts(b, ",\"WriterGroupName\":\"\",\"DataSetWriterName\":");
    oecm_put_json_str(b, s->il, s->il_len);
    oecm_puts(b, ",\"Payload\":");
}

static size_t finish(oecm_buf *b, size_t payload_at, size_t *payload_len) {
    oecm_puts(b, "}]}");
    if (b->full) {
        *payload_len = 0;
        return 0;
    }
    *payload_len = b->len - payload_at;
    return payload_at;
}

size_t oecm_encode_birth(oecm_state *s, int64_t unix_ms, size_t *payload_len) {
    const oecm_mam *m = &s->cfg->mam;
    oecm_buf b;
    oecm_buf_init(&b, s->tx, s->tx_cap);
    put_own_topic(&b, s, "MAM");
    size_t at = b.len;
    put_head(&b, s, unix_ms, OECM_CLASS_MAM, OECM_WRITER_MAM);
    oecm_puts(&b, "{\"Manufacturer\":");
    put_localized(&b, m->manufacturer);
    oecm_puts(&b, ",\"ManufacturerUri\":");
    put_json_cstr(&b, m->manufacturer_uri);
    oecm_puts(&b, ",\"Model\":");
    put_localized(&b, m->model);
    oecm_puts(&b, ",\"ProductCode\":");
    put_json_cstr(&b, m->product_code);
    oecm_puts(&b, ",\"HardwareRevision\":");
    put_json_cstr(&b, m->hardware_revision ? m->hardware_revision : "undefined");
    oecm_puts(&b, ",\"SoftwareRevision\":");
    put_json_cstr(&b, m->software_revision ? m->software_revision : "undefined");
    oecm_puts(&b, ",\"DeviceRevision\":");
    put_json_cstr(&b, m->device_revision);
    oecm_puts(&b, ",\"DeviceManual\":");
    put_json_cstr(&b, m->device_manual);
    oecm_puts(&b, ",\"DeviceClass\":");
    put_json_cstr(&b, m->device_class);
    oecm_puts(&b, ",\"SerialNumber\":");
    put_json_cstr(&b, m->serial_number);
    oecm_puts(&b, ",\"ProductInstanceUri\":");
    oecm_put_json_str(&b, s->il, s->il_len);
    oecm_puts(&b, ",\"RevisionCounter\":");
    oecm_put_i64(&b, m->revision_counter);
    if (m->description) {
        oecm_puts(&b, ",\"Description\":");
        put_localized(&b, m->description);
    }
    if (m->global_asset_id) {
        oecm_puts(&b, ",\"GlobalAssetId\":");
        put_json_cstr(&b, m->global_asset_id);
    }
    oecm_putc(&b, '}');
    return finish(&b, at, payload_len);
}

size_t oecm_encode_health(oecm_state *s, int64_t unix_ms, oecm_health h, int score,
                          size_t *payload_len) {
    oecm_buf b;
    oecm_buf_init(&b, s->tx, s->tx_cap);
    put_own_topic(&b, s, "Health");
    size_t at = b.len;
    put_head(&b, s, unix_ms, OECM_CLASS_HEALTH, OECM_WRITER_HEALTH);
    oecm_puts(&b, "{\"Health\":\"");
    oecm_puts(&b, oecm_health_name(h));
    oecm_putc(&b, '"');
    if (score >= 0) {
        oecm_puts(&b, ",\"HealthScore\":");
        oecm_put_u64(&b, (uint64_t)score);
    }
    oecm_putc(&b, '}');
    return finish(&b, at, payload_len);
}

/* --- Lifecycle --------------------------------------------------------------- */

static uint32_t mono(const oecm_state *s) { return s->pal->monotonic_ms(s->pal->ctx); }
static int64_t wall(const oecm_state *s) { return s->pal->unix_time_ms(s->pal->ctx); }

static oecm_err publish(oecm_state *s, size_t at, size_t len) {
    if (at == 0)
        return OECM_E_NOSPACE;
    if (s->pal->mqtt_publish(s->pal->ctx, (const char *)s->tx, s->tx + at, len) != OECM_OK)
        return OECM_E_PAL;
    return OECM_OK;
}

static void enter(oecm_state *s, uint8_t phase) {
    s->phase = phase;
    s->phase_since_ms = mono(s);
}

/* A new Will for every connect (REQ-LIFE-008, REQ-LIFE-016). */
static oecm_err connect(oecm_state *s) {
    size_t len;
    size_t at = oecm_encode_health(s, wall(s), OECM_HEALTH_FAILURE, 0, &len);
    if (at == 0)
        return OECM_E_NOSPACE;
    const oecm_config *c = s->cfg;
    oecm_connect opts = {
        .client_id = s->client_id,
        .keep_alive_s = c->keep_alive_s ? c->keep_alive_s : 60u,
        .will_topic = (const char *)s->tx,
        .will_payload = s->tx + at,
        .will_len = len,
    };
    enter(s, PH_CONNECTING);
    oecm_err err = s->pal->mqtt_connect(s->pal->ctx, &opts);
    if (err != OECM_OK) {
        enter(s, PH_BACKOFF);
        return OECM_E_PAL;
    }
    return OECM_OK;
}

static bool is_failure(oecm_health h, int score) { return h == OECM_HEALTH_FAILURE && score == 0; }

/* The Birth when due, then the Health on change or heartbeat (REQ-LIFE-001,
 * REQ-LIFE-017, REQ-TOP-017). A failed publication stays due. */
static oecm_err publish_due(oecm_state *s) {
    uint32_t now = mono(s);
    bool beat = s->health_due || now - s->last_health_ms >= HEARTBEAT_MS;
    bool failure = is_failure((oecm_health)s->health, s->score);
    bool birth = s->birth_due || (beat && s->unavailable && !failure);
    size_t len;
    if (birth) {
        oecm_err err = publish(s, oecm_encode_birth(s, wall(s), &len), len);
        if (err != OECM_OK)
            return err;
        s->birth_due = false;
    }
    if (!beat)
        return OECM_OK;
    oecm_err err =
        publish(s, oecm_encode_health(s, wall(s), (oecm_health)s->health, s->score, &len), len);
    if (err != OECM_OK) {
        s->health_due = true;
        return err;
    }
    s->unavailable = failure;
    s->health_due = false;
    s->last_health_ms = now;
    return OECM_OK;
}

oecm_err oecm_init(oecm_device *dev, const oecm_config *cfg, const oecm_pal *pal,
                   uint8_t *tx_buf, size_t tx_cap) {
    if (dev == NULL || cfg == NULL || pal == NULL || tx_buf == NULL)
        return OECM_E_ARG;
    oecm_state *s = state(dev);
    memset(s, 0, sizeof *s);
    if (cfg->level < OECM_LEVEL_1 || cfg->level > OECM_LEVEL_3)
        return OECM_E_ARG;
    if (cfg->level != OECM_LEVEL_1)
        return OECM_E_LEVEL;
    if (oecm_service_type_name(cfg->service_type) == NULL)
        return OECM_E_ARG;
    if (!pal->mqtt_connect || !pal->mqtt_publish || !pal->mqtt_disconnect || !pal->unix_time_ms ||
        !pal->monotonic_ms)
        return OECM_E_ARG;
    if (tx_cap < OECM_TX_MIN)
        return OECM_E_NOSPACE;
    const oecm_mam *m = &cfg->mam;
    const char *texts[] = {m->manufacturer,      m->manufacturer_uri, m->model,
                           m->product_code,      m->hardware_revision, m->software_revision,
                           m->device_revision,   m->device_manual,    m->device_class,
                           m->serial_number,     m->description,      m->global_asset_id,
                           cfg->identification_link, cfg->client_id};
    for (size_t i = 0; i < sizeof texts / sizeof texts[0]; i++)
        if (texts[i] && !oecm_utf8_valid(texts[i], strlen(texts[i])))
            return OECM_E_ARG;
    if (!m->manufacturer_uri ||
        !oecm_manufacturer_uri_valid(m->manufacturer_uri, strlen(m->manufacturer_uri)))
        return OECM_E_ARG;
    if (m->revision_counter < -1)
        return OECM_E_ARG;
    if (cfg->identification_link) {
        s->il = cfg->identification_link;
        s->il_len = strlen(s->il);
        if (oecm_il_check(s->il, s->il_len) != 0)
            return OECM_E_ARG;
    } else {
        if (oecm_oi4identifier(m->manufacturer_uri, m->model, m->product_code, m->serial_number,
                               s->il_buf, sizeof s->il_buf, &s->il_len) != 0)
            return OECM_E_ARG;
        s->il = s->il_buf;
    }
    if (cfg->client_id) {
        size_t n = strlen(cfg->client_id);
        if (n == 0 || n >= sizeof s->client_id)
            return OECM_E_ARG;
        memcpy(s->client_id, cfg->client_id, n + 1);
    } else {
        static const char hex[] = "0123456789abcdef";
        uint64_t h = oecm_fnv1a64(s->il, s->il_len);
        memcpy(s->client_id, "oec", 3);
        for (int i = 0; i < 16; i++)
            s->client_id[3 + i] = hex[h >> (60 - 4 * i) & 15u];
        s->client_id[19] = '\0';
    }
    s->cfg = cfg;
    s->pal = pal;
    s->tx = tx_buf;
    s->tx_cap = tx_cap;
    /* The longest MessageId: 20 digits of time and 10 of counter. */
    s->id_ms = UINT64_MAX;
    s->id_counter = UINT32_MAX - 1u;
    size_t len;
    size_t at = oecm_encode_birth(s, 0, &len);
    s->id_ms = 0;
    s->id_counter = 0;
    if (at == 0)
        return OECM_E_NOSPACE;
    s->health = OECM_HEALTH_NORMAL;
    s->score = 100;
    s->health_due = true;
    s->backoff_ms = BACKOFF_MIN_MS;
    s->phase = PH_IDLE;
    return OECM_OK;
}

oecm_err oecm_start(oecm_device *dev) {
    if (!ready(dev))
        return OECM_E_STATE;
    oecm_state *s = state(dev);
    if (s->phase != PH_IDLE && s->phase != PH_STOPPED)
        return OECM_E_STATE;
    if (wall(s) < PLAUSIBLE_UNIX_MS)
        return OECM_E_STATE;
    s->backoff_ms = BACKOFF_MIN_MS;
    return connect(s);
}

oecm_err oecm_stop(oecm_device *dev) {
    if (!ready(dev))
        return OECM_E_STATE;
    oecm_state *s = state(dev);
    oecm_err err = OECM_OK;
    switch (s->phase) {
    case PH_CONNECTED: {
        /* Close, then DISCONNECT (REQ-LIFE-005 … 007). */
        size_t len;
        err = publish(s, oecm_encode_health(s, wall(s), OECM_HEALTH_NORMAL, 0, &len), len);
        if (s->pal->mqtt_disconnect(s->pal->ctx) != OECM_OK && err == OECM_OK)
            err = OECM_E_PAL;
        break;
    }
    case PH_CONNECTING:
        if (s->pal->mqtt_disconnect(s->pal->ctx) != OECM_OK)
            err = OECM_E_PAL;
        break;
    case PH_BACKOFF:
        break;
    default:
        return OECM_E_STATE;
    }
    enter(s, PH_STOPPED);
    return err;
}

oecm_err oecm_poll(oecm_device *dev) {
    if (!ready(dev))
        return OECM_E_STATE;
    oecm_state *s = state(dev);
    uint32_t now = mono(s);
    switch (s->phase) {
    case PH_CONNECTING:
        if (now - s->phase_since_ms >= CONNECT_TIMEOUT_MS) {
            (void)s->pal->mqtt_disconnect(s->pal->ctx);
            enter(s, PH_BACKOFF);
        }
        return OECM_OK;
    case PH_BACKOFF:
        if (now - s->phase_since_ms < s->backoff_ms)
            return OECM_OK;
        s->backoff_ms = s->backoff_ms * 2u > BACKOFF_MAX_MS ? BACKOFF_MAX_MS : s->backoff_ms * 2u;
        return connect(s);
    case PH_CONNECTED:
        return publish_due(s);
    default:
        return OECM_OK;
    }
}

uint32_t oecm_poll_interval_ms(const oecm_device *dev) {
    if (!ready(dev))
        return POLL_MAX_MS;
    const oecm_state *s = cstate(dev);
    uint32_t now = s->pal->monotonic_ms(s->pal->ctx);
    uint32_t wait = POLL_MAX_MS;
    if (s->phase == PH_CONNECTED) {
        uint32_t since = now - s->last_health_ms;
        if (s->health_due || s->birth_due || since >= HEARTBEAT_MS)
            return 0;
        if (HEARTBEAT_MS - since < wait)
            wait = HEARTBEAT_MS - since;
    } else if (s->phase == PH_BACKOFF) {
        uint32_t since = now - s->phase_since_ms;
        if (since >= s->backoff_ms)
            return 0;
        if (s->backoff_ms - since < wait)
            wait = s->backoff_ms - since;
    }
    return wait;
}

void oecm_on_connected(oecm_device *dev) {
    if (!ready(dev))
        return;
    oecm_state *s = state(dev);
    if (s->phase == PH_STOPPED) {
        (void)s->pal->mqtt_disconnect(s->pal->ctx);
        return;
    }
    if (s->phase != PH_CONNECTING)
        return;
    enter(s, PH_CONNECTED);
    s->backoff_ms = BACKOFF_MIN_MS;
    /* The Birth first, then the Health (REQ-LIFE-001, REQ-LIFE-015). */
    s->birth_due = true;
    s->health_due = true;
    (void)publish_due(s);
}

void oecm_on_disconnected(oecm_device *dev) {
    if (!ready(dev))
        return;
    oecm_state *s = state(dev);
    if (s->phase == PH_CONNECTED || s->phase == PH_CONNECTING)
        enter(s, PH_BACKOFF);
}

void oecm_on_message(oecm_device *dev, const char *topic, const uint8_t *payload, size_t len) {
    (void)dev;
    (void)topic;
    (void)payload;
    (void)len;
}

oecm_err oecm_set_health(oecm_device *dev, oecm_health health, int score) {
    if (!ready(dev))
        return OECM_E_STATE;
    if (oecm_health_name(health) == NULL || score > 100)
        return OECM_E_ARG;
    if (score < 0)
        score = -1;
    if (health == OECM_HEALTH_NORMAL && score == 0)
        return OECM_E_ARG;
    oecm_state *s = state(dev);
    if (s->health != (uint8_t)health || s->score != score) {
        s->health = (uint8_t)health;
        s->score = (int8_t)score;
        s->health_due = true;
    }
    return OECM_OK;
}

const char *oecm_identification_link(const oecm_device *dev) {
    return ready(dev) ? cstate(dev)->il : NULL;
}
