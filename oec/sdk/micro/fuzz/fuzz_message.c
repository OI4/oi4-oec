/* Fuzz target: a received message (oecm_on_message) at a connected Level 3
 * device with writable Data, a Config context and an in-memory store (reset
 * for every input). Input: the topic after
 * "Oi4v2/OTConnector/<AppId>/" up to the first '\n', then the payload. The
 * oracle: every publication has an Oi4v2 topic and a valid JSON payload. */
#include <stddef.h>
#include <stdint.h>
#include <string.h>

#include "oecm.h"
#include "oecm_internal.h"

int LLVMFuzzerTestOneInput(const uint8_t *data, size_t size);

static int64_t unix_ms = 1793000000000;
static char will[256];
static oecm_span scratch;

static oecm_err p_connect(void *ctx, const oecm_connect *o) {
    (void)ctx;
    size_t n = strlen(o->will_topic);
    if (n >= sizeof will)
        __builtin_trap();
    memcpy(will, o->will_topic, n + 1);
    return OECM_OK;
}

static oecm_err p_publish(void *ctx, const char *topic, const uint8_t *payload, size_t len) {
    (void)ctx;
    if (strncmp(topic, "Oi4v2/", 6) != 0 || !oecm_json_parse((const char *)payload, len, &scratch))
        __builtin_trap();
    return OECM_OK;
}

static oecm_err p_subscribe(void *ctx, const char *filter) {
    (void)ctx;
    (void)filter;
    return OECM_OK;
}

static oecm_err p_disconnect(void *ctx) {
    (void)ctx;
    return OECM_OK;
}

static int64_t p_unix(void *ctx) {
    (void)ctx;
    return unix_ms;
}

static uint32_t p_mono(void *ctx) {
    (void)ctx;
    return 1000;
}

/* The store: one record per key ("oecm.wid", "oecm.refdes"). */
static struct {
    uint8_t buf[1024];
    size_t len;
} store[2];

static size_t slot(const char *key) {
    return strcmp(key, "oecm.wid") == 0 ? 0 : 1;
}

static oecm_err p_load(void *ctx, const char *key, uint8_t *buf, size_t cap, size_t *len) {
    (void)ctx;
    size_t i = slot(key);
    if (store[i].len > cap)
        return OECM_E_NOSPACE;
    memcpy(buf, store[i].buf, store[i].len);
    *len = store[i].len;
    return OECM_OK;
}

static oecm_err p_save(void *ctx, const char *key, const uint8_t *buf, size_t len) {
    (void)ctx;
    size_t i = slot(key);
    if (len > sizeof store[i].buf)
        return OECM_E_NOSPACE;
    memcpy(store[i].buf, buf, len);
    store[i].len = len;
    return OECM_OK;
}

static const oecm_pal pal = {.mqtt_connect = p_connect,
                             .mqtt_publish = p_publish,
                             .mqtt_subscribe = p_subscribe,
                             .mqtt_disconnect = p_disconnect,
                             .unix_time_ms = p_unix,
                             .monotonic_ms = p_mono,
                             .persist_load = p_load,
                             .persist_save = p_save};

static double pv;
static float sv;
static int32_t count;
static uint32_t state;
static bool on;
static char label[16];

static const oecm_field fields[] = {
    {.name = "Pv", .type = OECM_TYPE_DOUBLE, .value = &pv, .decimals = 2, .unit = "Cel",
     .eu_low = "-40", .eu_high = "85"},
    {.name = "Sv1", .type = OECM_TYPE_FLOAT, .value = &sv, .decimals = 1},
    {.name = "Count", .type = OECM_TYPE_INT32, .value = &count},
    {.name = "State", .type = OECM_TYPE_UINT32, .value = &state},
    {.name = "On", .type = OECM_TYPE_BOOLEAN, .value = &on},
    {.name = "Label", .type = OECM_TYPE_STRING, .value = label, .cap = sizeof label},
};

static const oecm_data datasets[] = {
    {.tag = "Temperature", .fields = fields, .field_count = 6, .writable = true},
    {.tag = "Humidity", .fields = fields, .field_count = 1, .interval_ms = 10000},
};

static char interval[12], name[17], enabled[8], mode[16], token[16];
static const char *const modes[] = {"Fast", "Slow"};

static const oecm_cfg_object objects[] = {
    {.key = "Interval", .name = "Interval", .type = OECM_CFG_NUMBER, .value = interval,
     .cap = sizeof interval, .mandatory = true, .min = "1", .max = "3600"},
    {.key = "Label", .name = "Label", .type = OECM_CFG_STRING, .value = name, .cap = sizeof name,
     .length = 16},
    {.key = "Enabled", .name = "Enabled", .type = OECM_CFG_BOOLEAN, .value = enabled,
     .cap = sizeof enabled},
    {.key = "Mode", .name = "Mode", .type = OECM_CFG_STRING, .value = mode, .cap = sizeof mode,
     .values = modes, .value_count = 2},
    {.key = "Token", .name = "Token", .type = OECM_CFG_STRING, .value = token,
     .cap = sizeof token, .sensitive = true},
};

static const oecm_cfg_group groups[] = {
    {.key = "General", .name = "General", .objects = objects, .count = 5}};
static const oecm_cfg_context contexts[] = {
    {.name = "Sensor", .title = "Sensor", .groups = groups, .count = 1}};

static char refdes[1024];

static const oecm_config cfg = {
    .level = OECM_LEVEL_3,
    .service_type = OECM_ST_OT_CONNECTOR,
    .mam = {.manufacturer = "Provider", .manufacturer_uri = "provider.com", .model = "Sensor",
            .product_code = "S-1", .serial_number = "4711", .revision_counter = -1},
    .data = datasets,
    .data_count = 2,
    .config = contexts,
    .config_count = 1,
    .refdes_buf = refdes,
    .refdes_cap = sizeof refdes,
};

static oecm_device dev;
static uint8_t tx[8192];
static char topic[1024];

int LLVMFuzzerTestOneInput(const uint8_t *data, size_t size) {
    pv = 21.5;
    sv = 1.5f;
    count = -3;
    state = 7;
    on = true;
    memcpy(label, "lab", 4);
    memcpy(interval, "10", 3);
    memcpy(name, "S", 2);
    memcpy(enabled, "true", 5);
    memcpy(mode, "Fast", 5);
    memcpy(token, "secret", 7);
    memset(store, 0, sizeof store);
    if (oecm_init(&dev, &cfg, &pal, tx, sizeof tx) != OECM_OK || oecm_start(&dev) != OECM_OK)
        __builtin_trap();
    oecm_on_connected(&dev);
    const char *health = strstr(will, "/Pub/Health/");
    if (health == NULL)
        __builtin_trap();
    size_t pre = (size_t)(health - will) + 1;
    const uint8_t *nl = memchr(data, '\n', size);
    size_t tn = nl ? (size_t)(nl - data) : size;
    if (pre + tn >= sizeof topic)
        return 0;
    memcpy(topic, will, pre);
    memcpy(topic + pre, data, tn);
    topic[pre + tn] = '\0';
    const uint8_t *payload = nl ? nl + 1 : data + size;
    oecm_on_message(&dev, topic, payload, (size_t)(data + size - payload));
    (void)oecm_poll(&dev);
    return 0;
}
