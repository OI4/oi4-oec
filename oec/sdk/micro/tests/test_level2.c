/* Messaging Level 2 with a recording PAL (ADR-0030, C-094): the
 * publications after a connect, Get with filters and pagination, Set of
 * Data, Config and the lists, the Status codes. The payloads are validated
 * by D7 in check.sh (tests/emit.c) and against D7 `test --level 2` by the
 * host runner. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "fake_pal.h"
#include "oecm_internal.h"
#include "unity.h"

static fake_pal fp;
static oecm_pal pal;
static oecm_device dev;
static uint8_t tx[8192];
static oecm_config cfg;

/* base64url of "provider.com/Sensor/S-1/4711" */
#define APP "cHJvdmlkZXIuY29tL1NlbnNvci9TLTEvNDcxMQ"
#define PRE "Oi4v2/OTConnector/" APP
/* base64url of "other.com/X/Y/1" */
#define OTHER "b3RoZXIuY29tL1gvWS8x"
#define REPLY "1793000000000-Registry/other.com/X/Y/1"

static double pv;
static float sv1;
static int32_t count;
static uint32_t state_;
static bool on;
static char label[16];

static const oecm_field fields[] = {
    {.name = "Pv", .type = OECM_TYPE_DOUBLE, .value = &pv, .decimals = 2, .unit = "°C",
     .eu_low = "-40", .eu_high = "85"},
    {.name = "Sv1", .type = OECM_TYPE_FLOAT, .value = &sv1, .decimals = 1},
    {.name = "Count", .type = OECM_TYPE_INT32, .value = &count},
    {.name = "State", .type = OECM_TYPE_UINT32, .value = &state_},
    {.name = "On", .type = OECM_TYPE_BOOLEAN, .value = &on},
    {.name = "Label", .type = OECM_TYPE_STRING, .value = label, .cap = sizeof label},
};

static double hum;
static const oecm_field hum_fields[] = {
    {.name = "Pv", .type = OECM_TYPE_DOUBLE, .value = &hum, .decimals = 1, .unit = "%"},
};

static oecm_data data[] = {
    {.tag = "Temperature", .description = "Air temperature", .fields = fields,
     .field_count = 6, .writable = true},
    {.tag = "Humidity", .fields = hum_fields, .field_count = 1, .interval_ms = 10000},
};

static char interval[12];
static char name_[17];
static char enabled[8];
static char mode[16];
static char secret[16];
static const char *const modes[] = {"Fast", "Slow"};

static const oecm_cfg_object objects[] = {
    {.key = "Interval", .name = "Interval", .description = "Publication interval",
     .type = OECM_CFG_NUMBER, .value = interval, .cap = sizeof interval,
     .default_value = "10", .unit = "s", .mandatory = true, .min = "1", .max = "3600"},
    {.key = "Label", .name = "Label", .type = OECM_CFG_STRING, .value = name_,
     .cap = sizeof name_, .length = 16},
    {.key = "Enabled", .name = "Enabled", .type = OECM_CFG_BOOLEAN, .value = enabled,
     .cap = sizeof enabled},
    {.key = "Mode", .name = "Mode", .type = OECM_CFG_STRING, .value = mode, .cap = sizeof mode,
     .values = modes, .value_count = 2},
    {.key = "Token", .name = "Token", .type = OECM_CFG_STRING, .value = secret,
     .cap = sizeof secret, .sensitive = true},
};

static const oecm_cfg_group groups[] = {
    {.key = "General", .name = "General", .objects = objects, .count = 5},
};

static const oecm_cfg_context contexts[] = {
    {.name = "Sensor", .title = "Sensor settings", .groups = groups, .count = 1},
};

static int config_calls;
static size_t config_last;
static int data_calls;
static size_t data_last;

static void on_config(void *ctx, size_t i) {
    (void)ctx;
    config_calls++;
    config_last = i;
}

static void on_data(void *ctx, size_t j) {
    (void)ctx;
    data_calls++;
    data_last = j;
}

void setUp(void) {
    fake_pal_reset(&fp);
    pal = fake_pal_make(&fp);
    pv = 21.5;
    sv1 = 1.5f;
    count = -3;
    state_ = 7;
    on = true;
    strcpy(label, "lab");
    hum = 40.0;
    strcpy(interval, "10");
    strcpy(name_, "Room");
    strcpy(enabled, "true");
    strcpy(mode, "Fast");
    strcpy(secret, "abc");
    config_calls = data_calls = 0;
    memset(&cfg, 0, sizeof cfg);
    cfg.level = OECM_LEVEL_2;
    cfg.service_type = OECM_ST_OT_CONNECTOR;
    cfg.mam = (oecm_mam){
        .manufacturer = "Provider",
        .manufacturer_uri = "provider.com",
        .model = "Sensor",
        .product_code = "S-1",
        .software_revision = "1.0.0",
        .device_revision = "1",
        .device_manual = "https://provider.com/manual",
        .device_class = "Sensor",
        .serial_number = "4711",
        .revision_counter = -1,
    };
    cfg.data = data;
    cfg.data_count = 2;
    cfg.config = contexts;
    cfg.config_count = 1;
    cfg.on_config = on_config;
    cfg.on_data = on_data;
}

void tearDown(void) {}

static void dump(const char *what) {
    if (!getenv("OECM_DUMP"))
        return;
    printf("--- %s\n", what);
    for (size_t i = 0; i < fp.count; i++)
        printf("%d %s\n  %s\n", fp.events[i].kind, fp.events[i].topic, fp.events[i].payload);
}

static void start_connected(void) {
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_start(&dev));
    oecm_on_connected(&dev);
    dump("connect");
}

static bool contains(const char *s, const char *part) { return strstr(s, part) != NULL; }

/* Sends a request and returns the index of the first event it caused. */
static size_t send(const char *topic, const char *payload) {
    size_t first = fp.count;
    oecm_on_message(&dev, topic, (const uint8_t *)payload, strlen(payload));
    if (getenv("OECM_DUMP")) {
        printf("--- %s %s\n", topic, payload);
        for (size_t i = first; i < fp.count; i++)
            printf("%d %s\n  %s\n", fp.events[i].kind, fp.events[i].topic,
                   fp.events[i].payload);
    }
    return first;
}

static char msg[8192];

/* A NetworkMessage of a request with the given Messages array. */
static const char *request(const char *messages) {
    snprintf(msg, sizeof msg,
             "{\"MessageId\":\"" REPLY "\",\"MessageType\":\"ua-data\","
             "\"PublisherId\":\"Registry/other.com/X/Y/1\",\"Messages\":%s}",
             messages);
    return msg;
}

static const fake_event *ev(size_t i) {
    TEST_ASSERT_TRUE(i < fp.count);
    return &fp.events[i];
}

/* The Status event after a Set/Del: the last publication. */
static const char *status(void) {
    const fake_event *e = fake_last(&fp, FAKE_PUBLISH);
    TEST_ASSERT_NOT_NULL(e);
    TEST_ASSERT_TRUE(contains(e->topic, "/Pub/Event/"));
    return e->payload;
}

/* --- connect ------------------------------------------------------------- */

static void test_connect_subscribes_and_publishes_lists(void) {
    start_connected();
    TEST_ASSERT_EQUAL_size_t(3, fake_count(&fp, FAKE_SUBSCRIBE));
    TEST_ASSERT_EQUAL_STRING(PRE "/Get/#", ev(1)->topic);
    TEST_ASSERT_EQUAL_STRING(PRE "/Set/#", ev(2)->topic);
    TEST_ASSERT_EQUAL_STRING(PRE "/Del/#", ev(3)->topic);
    /* Birth first after the subscriptions (REQ-LIFE-001) */
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/MAM/" APP, ev(4)->topic);
    /* then the order of oec-core provider.rs on_connected */
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Health/" APP, ev(5)->topic);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Profile/" APP, ev(6)->topic);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/PublicationList", ev(7)->topic);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/SubscriptionList", ev(8)->topic);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Config/" APP "/Sensor", ev(9)->topic);
    bool config = false, publist = false, meta = false, temp = false, sublist = false;
    for (size_t i = 5; i < fp.count; i++) {
        const fake_event *e = ev(i);
        if (strcmp(e->topic, PRE "/Pub/Config/" APP "/Sensor") == 0) {
            config = true;
            TEST_ASSERT_TRUE(contains(e->payload, "\"Interval\":{\"Type\":\"Number\""));
            TEST_ASSERT_TRUE(contains(e->payload, "\"Value\":\"10\""));
            /* sensitive values are masked (REQ-RES-CONFIG-024) */
            TEST_ASSERT_FALSE(contains(e->payload, "abc"));
        }
        publist |= strcmp(e->topic, PRE "/Pub/PublicationList") == 0;
        sublist |= strcmp(e->topic, PRE "/Pub/SubscriptionList") == 0;
        meta |= strcmp(e->topic, PRE "/Pub/Metadata/" APP "/Temperature") == 0;
        if (strcmp(e->topic, PRE "/Pub/Data/" APP "/Temperature") == 0) {
            temp = true;
            TEST_ASSERT_TRUE(contains(e->payload, "\"Pv\":21.50"));
            TEST_ASSERT_TRUE(contains(e->payload, "\"Label\":\"lab\""));
        }
    }
    TEST_ASSERT_TRUE(config && publist && meta && temp && sublist);
}

static void test_level_2_needs_subscribe(void) {
    pal.mqtt_subscribe = NULL;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
}

static void test_invalid_definitions_are_rejected(void) {
    oecm_data bad = data[0];
    bad.tag = "a b";
    cfg.data = &bad;
    cfg.data_count = 1;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    oecm_data twice[2] = {data[1], data[1]};
    cfg.data = twice;
    cfg.data_count = 2;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
}

static void test_reserved_config_names_are_rejected(void) {
    static const char *const keys[] = {"Name", "Description", "1st", "-x"};
    for (size_t i = 0; i < 4; i++) {
        oecm_cfg_object o = objects[1];
        o.key = keys[i];
        oecm_cfg_group g = groups[0];
        g.objects = &o;
        g.count = 1;
        oecm_cfg_context x = contexts[0];
        x.groups = &g;
        cfg.config = &x;
        TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    }
}

static void test_small_buffer_is_nospace(void) {
    TEST_ASSERT_EQUAL(OECM_E_NOSPACE, oecm_init(&dev, &cfg, &pal, tx, 900));
}

/* --- Get ----------------------------------------------------------------- */

static void test_get_mam_answers_with_reply_to(void) {
    start_connected();
    size_t at = send(PRE "/Get/MAM/" APP, request("[]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/MAM/" APP, ev(at)->topic);
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"ReplyTo\":\"" REPLY "\""));
}

static void test_get_without_source_answers_every_asset(void) {
    start_connected();
    size_t at = send(PRE "/Get/Health", request("[]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Health", ev(at)->topic);
}

static void test_get_data_by_tag_and_all(void) {
    start_connected();
    size_t at = send(PRE "/Get/Data/" APP "/Humidity", request("[]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Data/" APP "/Humidity", ev(at)->topic);
    TEST_ASSERT_FALSE(contains(ev(at)->payload, "Temperature"));
    at = send(PRE "/Get/Data/" APP, request("[]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Data/" APP, ev(at)->topic);
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"Pv\":40.0"));
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"Pv\":21.50"));
}

static void test_get_unknown_tag_is_empty_list(void) {
    start_connected();
    size_t at = send(PRE "/Get/Data/" APP "/Pressure", request("[]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"Messages\":[]"));
}

static void test_get_foreign_source_is_not_found(void) {
    start_connected();
    size_t at = send(PRE "/Get/MAM/" OTHER, request("[]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Event/" APP "/Status/Bad", ev(at)->topic);
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "BadNotFound"));
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"ReplyTo\":\"" REPLY "\""));
}

static void test_get_metadata_by_tag(void) {
    start_connected();
    size_t at = send(PRE "/Get/Metadata/" APP "/Temperature", request("[]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Metadata/" APP "/Temperature", ev(at)->topic);
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"MessageType\":\"ua-metadata\""));
    at = send(PRE "/Get/Metadata/" APP "/Nope", request("[]"));
    TEST_ASSERT_EQUAL_size_t(at, fp.count);
}

static void test_get_config_and_publication_list(void) {
    start_connected();
    size_t at = send(PRE "/Get/Config/" APP "/Sensor", request("[]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Config/" APP "/Sensor", ev(at)->topic);
    at = send(PRE "/Get/PublicationList/" APP "/Data", request("[]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/PublicationList/" APP "/Data", ev(at)->topic);
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"Filter\":\"Temperature\""));
    TEST_ASSERT_FALSE(contains(ev(at)->payload, "\"Resource\":\"MAM\""));
}

static void test_get_pagination(void) {
    start_connected();
    size_t at = send(PRE "/Get/PublicationList/" APP,
                     request("[{\"DataSetWriterId\":1,\"Payload\":{\"PerPage\":2,\"Page\":2}}]"));
    TEST_ASSERT_EQUAL_size_t(at + 1, fp.count);
    const char *p = ev(at)->payload;
    TEST_ASSERT_TRUE(contains(p, "\"DataSetWriterId\":2"));
    TEST_ASSERT_TRUE(contains(p, "\"Page\":2"));
    TEST_ASSERT_TRUE(contains(p, "\"HasNext\":true"));
}

static void test_requests_that_are_ignored(void) {
    start_connected();
    size_t at = fp.count;
    send(PRE "/Get/MAM/" APP, "{\"MessageId\":\"x\",\"Messages\":[]}");
    send(PRE "/Get/MAM/" APP, "not json");
    send(PRE "/Get/Nope/" APP, request("[]"));
    send(PRE "/Pub/MAM/" APP, request("[]"));
    send("Oi4v2/OTConnector/" OTHER "/Get/MAM/" OTHER, request("[]"));
    send(PRE "/Get/MAM/" APP "/x", request("[]"));
    send(PRE "/Set/Data", request("[]"));
    TEST_ASSERT_EQUAL_size_t(at, fp.count);
}

/* --- Set Data ------------------------------------------------------------ */

static const char *set_temp(const char *payload) {
    static char m[1024];
    snprintf(m, sizeof m, "[{\"DataSetWriterId\":19,\"Payload\":%s}]", payload);
    return request(m);
}

/* A complete Temperature payload (every field, oec-core write.rs). */
static const char *temp(const char *pv_, const char *sv1_, const char *count_,
                        const char *state__, const char *on_, const char *label_) {
    static char p[512];
    snprintf(p, sizeof p,
             "{\"Pv\":%s,\"Sv1\":%s,\"Count\":%s,\"State\":%s,\"On\":%s,\"Label\":%s}", pv_,
             sv1_, count_, state__, on_, label_);
    return set_temp(p);
}

static void test_set_data_writes_and_answers(void) {
    start_connected();
    size_t at = send(PRE "/Set/Data/" APP "/Temperature",
                     temp("23.25", "1.5", "5", "7", "false", "\"x\""));
    TEST_ASSERT_EQUAL_DOUBLE(23.25, pv);
    TEST_ASSERT_EQUAL_INT32(5, count);
    TEST_ASSERT_EQUAL_STRING("x", label);
    TEST_ASSERT_FALSE(on);
    TEST_ASSERT_EQUAL(1, data_calls);
    TEST_ASSERT_EQUAL_size_t(0, data_last);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Data/" APP "/Temperature", ev(at)->topic);
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"ReplyTo\":\"" REPLY "\""));
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"Pv\":23.25"));
    TEST_ASSERT_TRUE(contains(status(), "\"Good\""));
    /* the change is published once more without ReplyTo (REQ-MQTT-019) */
    size_t before = fp.count;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(before + 1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(before)->payload, "\"ReplyTo\":\"\""));
}

static void test_set_data_rejects_whole_set(void) {
    start_connected();
    send(PRE "/Set/Data/" APP "/Temperature", temp("30", "1", "\"x\"", "7", "true", "\"\""));
    TEST_ASSERT_TRUE(contains(status(), "BadTypeMismatch"));
    TEST_ASSERT_EQUAL_DOUBLE(21.5, pv);
    send(PRE "/Set/Data/" APP "/Temperature", set_temp("{\"Pv\":30}"));
    TEST_ASSERT_TRUE(contains(status(), "BadInvalidArgument"));
    send(PRE "/Set/Data/" APP "/Temperature",
         temp("30", "1", "3000000000", "7", "true", "\"\""));
    TEST_ASSERT_TRUE(contains(status(), "BadTypeMismatch"));
    send(PRE "/Set/Data/" APP "/Temperature", temp("30", "1", "1", "-1", "true", "\"\""));
    TEST_ASSERT_TRUE(contains(status(), "BadTypeMismatch"));
    send(PRE "/Set/Data/" APP "/Temperature",
         temp("30", "1", "1", "1", "true", "\"0123456789abcdef\""));
    TEST_ASSERT_TRUE(contains(status(), "BadOutOfRange"));
    send(PRE "/Set/Data/" APP "/Temperature", temp("30", "1e300", "1", "1", "true", "\"\""));
    TEST_ASSERT_TRUE(contains(status(), "BadTypeMismatch"));
    send(PRE "/Set/Data/" APP "/Temperature", temp("30", "1", "1", "1", "1", "\"\""));
    TEST_ASSERT_TRUE(contains(status(), "BadTypeMismatch"));
    send(PRE "/Set/Data/" APP "/Temperature", request("[]"));
    TEST_ASSERT_TRUE(contains(status(), "BadInvalidArgument"));
    TEST_ASSERT_EQUAL(0, data_calls);
    TEST_ASSERT_EQUAL_DOUBLE(21.5, pv);
    TEST_ASSERT_EQUAL_FLOAT(1.5f, sv1);
    TEST_ASSERT_EQUAL_STRING("lab", label);
}

static void test_set_data_special_floats(void) {
    start_connected();
    send(PRE "/Set/Data/" APP "/Temperature",
         temp("\"NaN\"", "\"-Infinity\"", "-3", "7", "true", "\"lab\""));
    TEST_ASSERT_TRUE(contains(status(), "\"Good\""));
    TEST_ASSERT_TRUE(pv != pv);
    TEST_ASSERT_TRUE(sv1 < -3.4e38f);
}

static void test_set_data_not_writable(void) {
    start_connected();
    send(PRE "/Set/Data/" APP "/Humidity",
         request("[{\"DataSetWriterId\":20,\"Payload\":{\"Pv\":1}}]"));
    TEST_ASSERT_TRUE(contains(status(), "BadNotWritable"));
    TEST_ASSERT_EQUAL_DOUBLE(40.0, hum);
    send(PRE "/Set/Data/" APP "/Pressure",
         request("[{\"DataSetWriterId\":21,\"Payload\":{\"Pv\":1}}]"));
    TEST_ASSERT_TRUE(contains(status(), "BadNotFound"));
}

static void test_set_unchanged_data_calls_nothing(void) {
    start_connected();
    send(PRE "/Set/Data/" APP "/Temperature", temp("21.5", "1.5", "-3", "7", "true", "\"lab\""));
    TEST_ASSERT_TRUE(contains(status(), "\"Good\""));
    TEST_ASSERT_EQUAL(0, data_calls);
}

static void test_set_foreign_source_is_not_found(void) {
    start_connected();
    send(PRE "/Set/Data/" OTHER "/Temperature", temp("1", "1", "1", "1", "true", "\"\""));
    TEST_ASSERT_TRUE(contains(status(), "BadNotFound"));
    TEST_ASSERT_EQUAL_DOUBLE(21.5, pv);
}

static void test_del_data_is_not_found(void) {
    start_connected();
    send(PRE "/Del/Data/" APP "/Temperature", request("[]"));
    TEST_ASSERT_TRUE(contains(status(), "Bad"));
}

/* --- Set Config ---------------------------------------------------------- */

static const char *set_config(const char *general) {
    static char m[2048];
    snprintf(m, sizeof m,
             "[{\"DataSetWriterId\":%d,\"Payload\":{\"Context\":{\"Name\":{\"Locale\":\"en-US\","
             "\"Text\":\"Sensor settings\"}},\"General\":{\"Name\":{\"Locale\":\"en-US\","
             "\"Text\":\"General\"},%s}}}]",
             18, general);
    return request(m);
}

/* The writer id of the Config DataSet from the Birth publications. */
static int config_writer(void) {
    for (size_t i = 0; i < fp.count; i++) {
        const fake_event *e = &fp.events[i];
        if (strcmp(e->topic, PRE "/Pub/Config/" APP "/Sensor") == 0) {
            const char *w = strstr(e->payload, "\"DataSetWriterId\":");
            TEST_ASSERT_NOT_NULL(w);
            return atoi(w + 18);
        }
    }
    TEST_FAIL_MESSAGE("no Config");
    return 0;
}

static const char *obj(const char *key, const char *type, const char *value,
                       const char *extra) {
    static char o[5][512];
    static int k;
    k = (k + 1) % 5;
    snprintf(o[k], sizeof o[k],
             "\"%s\":{\"Name\":{\"Locale\":\"en-US\",\"Text\":\"%s\"},\"Type\":\"%s\","
             "\"Value\":\"%s\"%s}",
             key, key, type, value, extra);
    return o[k];
}

static const char *full_config(const char *iv, const char *nm, const char *mo) {
    static char g[2048];
    char a[512], b[512], c[512], d[512];
    snprintf(a, sizeof a, "%s",
             obj("Interval", "Number", iv,
                 ",\"Description\":{\"Locale\":\"en-US\",\"Text\":\"Publication interval\"},"
                 "\"DefaultValue\":\"10\",\"Unit\":\"s\",\"Mandatory\":true,"
                 "\"Validation\":{\"Min\":1,\"Max\":3600}"));
    snprintf(b, sizeof b, "%s", obj("Label", "String", nm, ",\"Validation\":{\"Length\":16}"));
    snprintf(c, sizeof c, "%s", obj("Enabled", "Boolean", "true", ""));
    snprintf(d, sizeof d, "%s",
             obj("Mode", "String", mo, ",\"Validation\":{\"Values\":[\"Fast\",\"Slow\"]}"));
    snprintf(g, sizeof g, "%s,%s,%s,%s", a, b, c, d);
    return g;
}

static void test_set_config_writes_valid_values(void) {
    start_connected();
    TEST_ASSERT_EQUAL(18, config_writer());
    size_t at = send(PRE "/Set/Config/" APP "/Sensor",
                     set_config(full_config("60", "Lab", "Slow")));
    TEST_ASSERT_TRUE(contains(status(), "\"Good\""));
    TEST_ASSERT_EQUAL_STRING("60", interval);
    TEST_ASSERT_EQUAL_STRING("Lab", name_);
    TEST_ASSERT_EQUAL_STRING("Slow", mode);
    TEST_ASSERT_EQUAL_STRING("abc", secret);
    TEST_ASSERT_EQUAL(1, config_calls);
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/Config/" APP "/Sensor", ev(at)->topic);
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"ReplyTo\":\"" REPLY "\""));
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"Value\":\"60\""));
}

static void test_set_config_violations(void) {
    start_connected();
    send(PRE "/Set/Config/" APP "/Sensor", set_config(full_config("0", "Lab", "Slow")));
    TEST_ASSERT_TRUE(contains(status(), "Bad"));
    send(PRE "/Set/Config/" APP "/Sensor", set_config(full_config("x", "Lab", "Slow")));
    TEST_ASSERT_TRUE(contains(status(), "Bad"));
    send(PRE "/Set/Config/" APP "/Sensor",
         set_config(full_config("5", "01234567890123456", "Slow")));
    TEST_ASSERT_TRUE(contains(status(), "Bad"));
    send(PRE "/Set/Config/" APP "/Sensor", set_config(full_config("5", "Lab", "Medium")));
    TEST_ASSERT_TRUE(contains(status(), "Bad"));
    TEST_ASSERT_EQUAL_STRING("10", interval);
    TEST_ASSERT_EQUAL_STRING("Room", name_);
    TEST_ASSERT_EQUAL(0, config_calls);
}

static void test_set_config_unknown_context(void) {
    start_connected();
    send(PRE "/Set/Config/" APP "/Other", set_config(full_config("5", "Lab", "Slow")));
    TEST_ASSERT_TRUE(contains(status(), "BadNotFound"));
}

/* --- Set lists ----------------------------------------------------------- */

static const char *publist_entry(const char *mode_) {
    static char m[512];
    snprintf(m, sizeof m,
             "[{\"DataSetWriterId\":13,\"Payload\":{\"Resource\":\"Data\",\"Source\":"
             "\"provider.com/Sensor/S-1/4711\",\"Filter\":\"Temperature\","
             "\"DataSetWriterId\":19,\"Mode\":\"%s\"}}]",
             mode_);
    return request(m);
}

static void test_set_publication_list(void) {
    start_connected();
    /* unchanged: the entry with ReplyTo, then Good */
    size_t at = send(PRE "/Set/PublicationList/" APP "/Data/Temperature",
                     publist_entry("ON_REQUEST_FILTER_9"));
    TEST_ASSERT_EQUAL_STRING(PRE "/Pub/PublicationList/" APP "/Data/Temperature", ev(at)->topic);
    TEST_ASSERT_TRUE(contains(ev(at)->payload, "\"ReplyTo\":\"" REPLY "\""));
    TEST_ASSERT_TRUE(contains(status(), "\"Good\""));
    /* the same Mode key (FILTER) without ON_REQUEST is no change */
    send(PRE "/Set/PublicationList/" APP "/Data/Temperature", publist_entry("FILTER_4"));
    TEST_ASSERT_TRUE(contains(status(), "\"Good\""));
    /* every entry of the device is Config NONE_0: a change is not writable */
    send(PRE "/Set/PublicationList/" APP "/Data/Temperature", publist_entry("SOURCE_3"));
    TEST_ASSERT_TRUE(contains(status(), "BadNotWritable"));
    send(PRE "/Set/PublicationList/" APP "/Data/Pressure", publist_entry("FILTER_4"));
    TEST_ASSERT_TRUE(contains(status(), "BadNotFound"));
    send(PRE "/Set/PublicationList/" APP "/Data/Temperature", publist_entry("SOMETIMES"));
    TEST_ASSERT_TRUE(contains(status(), "BadDecodingError"));
}

static const char *sub_entry(const char *path) {
    static char m[512];
    snprintf(m, sizeof m,
             "[{\"DataSetWriterId\":17,\"Payload\":{\"TopicPath\":\"%s\",\"Config\":"
             "\"NONE_0\"}}]",
             path);
    return request(m);
}

static void test_set_subscription_list(void) {
    start_connected();
    const char *foreign = "Oi4v2/Registry/" OTHER "/Pub/Health/#";
    send(PRE "/Set/SubscriptionList/" APP "/Health", sub_entry(foreign));
    TEST_ASSERT_TRUE(contains(status(), "BadNotSupported"));
    send(PRE "/Del/SubscriptionList/" APP "/Health", sub_entry(foreign));
    TEST_ASSERT_TRUE(contains(status(), "BadNotFound"));
    /* the WriterGroupName of the TopicPath must be the Filter */
    send(PRE "/Set/SubscriptionList/" APP "/MAM", sub_entry(foreign));
    TEST_ASSERT_TRUE(contains(status(), "BadInvalidArgument"));
    const char *own = PRE "/Set/Data/" APP "/Temperature";
    send(PRE "/Set/SubscriptionList/" APP "/Data/Temperature", sub_entry(own));
    TEST_ASSERT_TRUE(contains(status(), "BadNotWritable"));
    send(PRE "/Del/SubscriptionList/" APP "/Data/Temperature", sub_entry(own));
    TEST_ASSERT_TRUE(contains(status(), "BadNotWritable"));
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_connect_subscribes_and_publishes_lists);
    RUN_TEST(test_level_2_needs_subscribe);
    RUN_TEST(test_invalid_definitions_are_rejected);
    RUN_TEST(test_reserved_config_names_are_rejected);
    RUN_TEST(test_small_buffer_is_nospace);
    RUN_TEST(test_get_mam_answers_with_reply_to);
    RUN_TEST(test_get_without_source_answers_every_asset);
    RUN_TEST(test_get_data_by_tag_and_all);
    RUN_TEST(test_get_unknown_tag_is_empty_list);
    RUN_TEST(test_get_foreign_source_is_not_found);
    RUN_TEST(test_get_metadata_by_tag);
    RUN_TEST(test_get_config_and_publication_list);
    RUN_TEST(test_get_pagination);
    RUN_TEST(test_requests_that_are_ignored);
    RUN_TEST(test_set_data_writes_and_answers);
    RUN_TEST(test_set_data_rejects_whole_set);
    RUN_TEST(test_set_data_special_floats);
    RUN_TEST(test_set_data_not_writable);
    RUN_TEST(test_set_unchanged_data_calls_nothing);
    RUN_TEST(test_set_foreign_source_is_not_found);
    RUN_TEST(test_del_data_is_not_found);
    RUN_TEST(test_set_config_writes_valid_values);
    RUN_TEST(test_set_config_violations);
    RUN_TEST(test_set_config_unknown_context);
    RUN_TEST(test_set_publication_list);
    RUN_TEST(test_set_subscription_list);
    return UNITY_END();
}
