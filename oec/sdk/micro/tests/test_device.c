/* Lifecycle of Messaging Level 1 with a recording PAL (ADR-0030 §5): the
 * same order as oec-core lifecycle.rs. The payloads are validated by D7 in
 * check.sh (tests/emit.c); here the order, topics and timers are checked. */
#include <stdio.h>
#include <string.h>

#include "fake_pal.h"
#include "oecm_internal.h"
#include "unity.h"

static fake_pal fp;
static oecm_pal pal;
static oecm_device dev;
static uint8_t tx[OECM_TX_MIN];
static oecm_config cfg;

/* base64url of "provider.com/Sensor/S-1/4711" */
#define APP "cHJvdmlkZXIuY29tL1NlbnNvci9TLTEvNDcxMQ"
#define TOPIC_MAM "Oi4v2/OTConnector/" APP "/Pub/MAM/" APP
#define TOPIC_HEALTH "Oi4v2/OTConnector/" APP "/Pub/Health/" APP

void setUp(void) {
    fake_pal_reset(&fp);
    pal = fake_pal_make(&fp);
    memset(&cfg, 0, sizeof cfg);
    cfg.level = OECM_LEVEL_1;
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
}

void tearDown(void) {}

static void start_connected(void) {
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_start(&dev));
    oecm_on_connected(&dev);
}

static bool contains(const char *s, const char *part) { return strstr(s, part) != NULL; }

static void test_birth_then_health_after_connect(void) {
    start_connected();
    TEST_ASSERT_EQUAL_size_t(3, fp.count);
    const fake_event *c = &fp.events[0];
    TEST_ASSERT_EQUAL(FAKE_CONNECT, c->kind);
    TEST_ASSERT_EQUAL_STRING(TOPIC_HEALTH, c->topic);
    TEST_ASSERT_TRUE(contains(c->payload, "\"Health\":\"FAILURE_1\",\"HealthScore\":0}"));
    TEST_ASSERT_EQUAL_UINT16(60, c->keep_alive_s);
    TEST_ASSERT_EQUAL_STRING(TOPIC_MAM, fp.events[1].topic);
    TEST_ASSERT_TRUE(contains(fp.events[1].payload, "\"ProductInstanceUri\":\"provider.com/Sensor/S-1/4711\""));
    TEST_ASSERT_TRUE(contains(fp.events[1].payload, "\"HardwareRevision\":\"undefined\""));
    TEST_ASSERT_TRUE(contains(fp.events[1].payload, "\"RevisionCounter\":-1"));
    TEST_ASSERT_FALSE(contains(fp.events[1].payload, "Description"));
    TEST_ASSERT_EQUAL_STRING(TOPIC_HEALTH, fp.events[2].topic);
    TEST_ASSERT_TRUE(contains(fp.events[2].payload, "\"Health\":\"NORMAL_0\",\"HealthScore\":100}"));
    TEST_ASSERT_EQUAL_STRING("provider.com/Sensor/S-1/4711", oecm_identification_link(&dev));
}

static void test_message_id_counter_while_clock_stands(void) {
    start_connected();
    /* The Will, the Birth and the Health at the same ms. */
    char id[64];
    snprintf(id, sizeof id, "\"MessageId\":\"%lld-OTConnector/", (long long)fp.unix_ms);
    TEST_ASSERT_TRUE(contains(fp.events[0].payload, id));
    snprintf(id, sizeof id, "\"MessageId\":\"%lld1-OTConnector/", (long long)fp.unix_ms);
    TEST_ASSERT_TRUE(contains(fp.events[1].payload, id));
    snprintf(id, sizeof id, "\"MessageId\":\"%lld2-OTConnector/", (long long)fp.unix_ms);
    TEST_ASSERT_TRUE(contains(fp.events[2].payload, id));
    fp.unix_ms += 1;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_set_health(&dev, OECM_HEALTH_OFF_SPEC, 40));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    snprintf(id, sizeof id, "\"MessageId\":\"%lld-OTConnector/", (long long)fp.unix_ms);
    TEST_ASSERT_TRUE(contains(fake_last(&fp, FAKE_PUBLISH)->payload, id));
}

static void test_heartbeat_every_60_s(void) {
    start_connected();
    size_t n = fp.count;
    fp.mono_ms += 59999;
    TEST_ASSERT_EQUAL_UINT32(1, oecm_poll_interval_ms(&dev));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n, fp.count);
    fp.mono_ms += 1;
    TEST_ASSERT_EQUAL_UINT32(0, oecm_poll_interval_ms(&dev));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n + 1, fp.count);
    TEST_ASSERT_EQUAL_STRING(TOPIC_HEALTH, fake_last(&fp, FAKE_PUBLISH)->topic);
    TEST_ASSERT_EQUAL_UINT32(1000, oecm_poll_interval_ms(&dev));
}

static void test_health_on_change_only(void) {
    start_connected();
    size_t n = fp.count;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_set_health(&dev, OECM_HEALTH_NORMAL, 100));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n, fp.count);
    TEST_ASSERT_EQUAL(OECM_OK, oecm_set_health(&dev, OECM_HEALTH_MAINTENANCE_REQUIRED, -5));
    TEST_ASSERT_EQUAL_UINT32(0, oecm_poll_interval_ms(&dev));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n + 1, fp.count);
    const char *p = fake_last(&fp, FAKE_PUBLISH)->payload;
    TEST_ASSERT_TRUE(contains(p, "\"Payload\":{\"Health\":\"MAINTENANCE_REQUIRED_4\"}"));
}

static void test_set_health_arguments(void) {
    memset(&dev, 0, sizeof dev);
    TEST_ASSERT_EQUAL(OECM_E_STATE, oecm_set_health(&dev, OECM_HEALTH_NORMAL, 50));
    start_connected();
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_set_health(&dev, OECM_HEALTH_NORMAL, 0));
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_set_health(&dev, OECM_HEALTH_FAILURE, 101));
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_set_health(&dev, (oecm_health)9, 1));
}

static void test_birth_again_after_failure_recovery(void) {
    start_connected();
    TEST_ASSERT_EQUAL(OECM_OK, oecm_set_health(&dev, OECM_HEALTH_FAILURE, 0));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    size_t n = fp.count;
    TEST_ASSERT_TRUE(contains(fp.events[n - 1].payload, "\"FAILURE_1\",\"HealthScore\":0}"));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_set_health(&dev, OECM_HEALTH_NORMAL, 90));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n + 2, fp.count);
    TEST_ASSERT_EQUAL_STRING(TOPIC_MAM, fp.events[n].topic);
    TEST_ASSERT_EQUAL_STRING(TOPIC_HEALTH, fp.events[n + 1].topic);
    /* A failure with a score is not unavailable: no Birth afterwards. */
    TEST_ASSERT_EQUAL(OECM_OK, oecm_set_health(&dev, OECM_HEALTH_FAILURE, 10));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_set_health(&dev, OECM_HEALTH_NORMAL, 90));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n + 4, fp.count);
    TEST_ASSERT_EQUAL_STRING(TOPIC_HEALTH, fp.events[n + 3].topic);
}

static void test_close_then_disconnect_on_stop(void) {
    start_connected();
    size_t n = fp.count;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_stop(&dev));
    TEST_ASSERT_EQUAL_size_t(n + 2, fp.count);
    TEST_ASSERT_EQUAL_STRING(TOPIC_HEALTH, fp.events[n].topic);
    TEST_ASSERT_TRUE(contains(fp.events[n].payload, "\"Health\":\"NORMAL_0\",\"HealthScore\":0}"));
    TEST_ASSERT_EQUAL(FAKE_DISCONNECT, fp.events[n + 1].kind);
    TEST_ASSERT_EQUAL(OECM_E_STATE, oecm_stop(&dev));
    /* Restart after a stop. */
    TEST_ASSERT_EQUAL(OECM_OK, oecm_start(&dev));
    TEST_ASSERT_EQUAL(FAKE_CONNECT, fp.events[fp.count - 1].kind);
}

static void test_connected_state(void) {
    memset(&dev, 0, sizeof dev);
    TEST_ASSERT_FALSE(oecm_connected(NULL));
    TEST_ASSERT_FALSE(oecm_connected(&dev));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    TEST_ASSERT_FALSE(oecm_connected(&dev));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_start(&dev));
    TEST_ASSERT_FALSE(oecm_connected(&dev));
    oecm_on_connected(&dev);
    TEST_ASSERT_TRUE(oecm_connected(&dev));
    oecm_on_disconnected(&dev);
    TEST_ASSERT_FALSE(oecm_connected(&dev));
    fp.mono_ms += 1000;
    fp.unix_ms += 1000;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_FALSE(oecm_connected(&dev)); /* connecting again */
    oecm_on_connected(&dev);
    TEST_ASSERT_TRUE(oecm_connected(&dev));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_stop(&dev));
    TEST_ASSERT_FALSE(oecm_connected(&dev));
}

static void test_reconnect_with_backoff_and_new_will(void) {
    start_connected();
    const fake_event *first = fake_last(&fp, FAKE_CONNECT);
    char will[FAKE_MAX_TEXT];
    snprintf(will, sizeof will, "%s", first->payload);
    oecm_on_disconnected(&dev);
    size_t n = fp.count;
    TEST_ASSERT_EQUAL_UINT32(1000, oecm_poll_interval_ms(&dev));
    fp.mono_ms += 999;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n, fp.count);
    fp.mono_ms += 1;
    fp.unix_ms += 1000;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n + 1, fp.count);
    const fake_event *c = fake_last(&fp, FAKE_CONNECT);
    TEST_ASSERT_EQUAL_STRING(first->client_id, c->client_id);
    TEST_ASSERT_TRUE(strcmp(will, c->payload) != 0); /* new MessageId */
    /* The connect fails: the next try after 2 s, then 4 s. */
    oecm_on_disconnected(&dev);
    fp.mono_ms += 1999;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n + 1, fp.count);
    fp.mono_ms += 1;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(n + 2, fp.count);
    oecm_on_connected(&dev);
    TEST_ASSERT_EQUAL_STRING(TOPIC_MAM, fp.events[n + 2].topic);
    TEST_ASSERT_EQUAL_STRING(TOPIC_HEALTH, fp.events[n + 3].topic);
}

static void test_backoff_is_capped_at_30_s(void) {
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    fp.connect_result = OECM_E_PAL;
    TEST_ASSERT_EQUAL(OECM_E_PAL, oecm_start(&dev));
    uint32_t waits[8];
    for (int i = 0; i < 8; i++) {
        waits[i] = 0;
        size_t n = fp.count;
        while (fp.count == n) {
            fp.mono_ms += 100;
            waits[i] += 100;
            (void)oecm_poll(&dev);
        }
    }
    const uint32_t want[8] = {1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000};
    TEST_ASSERT_EQUAL_UINT32_ARRAY(want, waits, 8);
}

static void test_connect_timeout(void) {
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_start(&dev));
    fp.mono_ms += 29999;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    fp.mono_ms += 1;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL(FAKE_DISCONNECT, fp.events[1].kind);
    oecm_on_connected(&dev); /* too late: ignored */
    TEST_ASSERT_EQUAL_size_t(2, fp.count);
    fp.mono_ms += 1000;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL(FAKE_CONNECT, fp.events[2].kind);
}

static void test_failed_publish_stays_due(void) {
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_start(&dev));
    fp.publish_result = OECM_E_PAL;
    oecm_on_connected(&dev);
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    TEST_ASSERT_EQUAL_UINT32(0, oecm_poll_interval_ms(&dev));
    fp.publish_result = OECM_OK;
    TEST_ASSERT_EQUAL(OECM_OK, oecm_poll(&dev));
    TEST_ASSERT_EQUAL_size_t(3, fp.count);
    TEST_ASSERT_EQUAL_STRING(TOPIC_MAM, fp.events[1].topic);
    TEST_ASSERT_EQUAL_STRING(TOPIC_HEALTH, fp.events[2].topic);
}

static void test_start_needs_wall_clock(void) {
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    fp.unix_ms = 1000;
    TEST_ASSERT_EQUAL(OECM_E_STATE, oecm_start(&dev));
    TEST_ASSERT_EQUAL_size_t(0, fp.count);
}

static void test_derived_and_given_client_id(void) {
    start_connected();
    const char *il = "provider.com/Sensor/S-1/4711";
    char want[24];
    snprintf(want, sizeof want, "oec%016llx", (unsigned long long)oecm_fnv1a64(il, strlen(il)));
    TEST_ASSERT_EQUAL_STRING(want, fp.events[0].client_id);
    TEST_ASSERT_EQUAL_UINT64(0xcbf29ce484222325u, oecm_fnv1a64("", 0));
    TEST_ASSERT_EQUAL_UINT64(0xaf63dc4c8601ec8cu, oecm_fnv1a64("a", 1));
    fake_pal_reset(&fp);
    cfg.client_id = "sensor-1";
    cfg.keep_alive_s = 30;
    start_connected();
    TEST_ASSERT_EQUAL_STRING("sensor-1", fp.events[0].client_id);
    TEST_ASSERT_EQUAL_UINT16(30, fp.events[0].keep_alive_s);
}

static void test_given_identification_link_and_optional_mam(void) {
    cfg.identification_link = "https://id.provider.com/S/4711";
    cfg.mam.description = "Temperature \"T\"";
    cfg.mam.global_asset_id = "https://provider.com/aas/4711";
    cfg.mam.hardware_revision = "B";
    cfg.mam.revision_counter = 3;
    start_connected();
    const char *p = fp.events[1].payload;
    TEST_ASSERT_TRUE(contains(p, "\"ProductInstanceUri\":\"https://id.provider.com/S/4711\""));
    TEST_ASSERT_TRUE(contains(p, "\"Description\":{\"Locale\":\"en-US\",\"Text\":\"Temperature \\\"T\\\"\"}"));
    TEST_ASSERT_TRUE(contains(p, "\"GlobalAssetId\":\"https://provider.com/aas/4711\""));
    TEST_ASSERT_TRUE(contains(p, "\"HardwareRevision\":\"B\""));
    TEST_ASSERT_TRUE(contains(p, "\"RevisionCounter\":3"));
    TEST_ASSERT_EQUAL_STRING("https://id.provider.com/S/4711", oecm_identification_link(&dev));
}

static void test_init_errors(void) {
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(NULL, &cfg, &pal, tx, sizeof tx));
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, NULL, &pal, tx, sizeof tx));
    TEST_ASSERT_EQUAL(OECM_E_NOSPACE, oecm_init(&dev, &cfg, &pal, tx, OECM_TX_MIN - 1));
    TEST_ASSERT_EQUAL(OECM_E_STATE, oecm_start(&dev)); /* failed init: not ready */
    oecm_config c = cfg;
    c.level = (oecm_level)0;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c = cfg;
    c.service_type = (oecm_service_type)7;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c = cfg;
    c.mam.manufacturer_uri = "www.provider.com";
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c = cfg;
    c.mam.manufacturer_uri = NULL;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c = cfg;
    c.identification_link = "HTTPS://provider.com/x";
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c.identification_link = "https://Provider.com/x";
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c.identification_link = "provider.com/a b";
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c = cfg;
    c.mam.model = "\xc3";
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c = cfg;
    c.client_id = "";
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c.client_id = "123456789012345678901234";
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c = cfg;
    c.mam.revision_counter = -2;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c = cfg;
    c.level = OECM_LEVEL_3; /* needs the ReferenceDesignation buffer and persist */
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    c.level = OECM_LEVEL_1; /* Data and Config need Level 2 */
    c.data_count = 1;
    TEST_ASSERT_EQUAL(OECM_E_LEVEL, oecm_init(&dev, &c, &pal, tx, sizeof tx));
    oecm_pal p = pal;
    p.monotonic_ms = NULL;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &cfg, &p, tx, sizeof tx));
}

/* The Birth must fit: a long MAM needs a larger buffer. */
static void test_init_trial_encodes_the_birth(void) {
    static char longtext[1200];
    memset(longtext, 'd', sizeof longtext - 1);
    cfg.mam.description = longtext;
    TEST_ASSERT_EQUAL(OECM_E_NOSPACE, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    static uint8_t big[OECM_TX_MIN * 2];
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &cfg, &pal, big, sizeof big));
}

static void test_level1_ignores_messages(void) {
    start_connected();
    size_t n = fp.count;
    oecm_on_message(&dev, TOPIC_MAM, (const uint8_t *)"{}", 2);
    TEST_ASSERT_EQUAL_size_t(n, fp.count);
}

static void test_err_strings(void) {
    TEST_ASSERT_EQUAL_STRING("ok", oecm_err_str(OECM_OK));
    TEST_ASSERT_EQUAL_STRING("unknown error", oecm_err_str((oecm_err)99));
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_birth_then_health_after_connect);
    RUN_TEST(test_message_id_counter_while_clock_stands);
    RUN_TEST(test_heartbeat_every_60_s);
    RUN_TEST(test_health_on_change_only);
    RUN_TEST(test_set_health_arguments);
    RUN_TEST(test_birth_again_after_failure_recovery);
    RUN_TEST(test_close_then_disconnect_on_stop);
    RUN_TEST(test_connected_state);
    RUN_TEST(test_reconnect_with_backoff_and_new_will);
    RUN_TEST(test_backoff_is_capped_at_30_s);
    RUN_TEST(test_connect_timeout);
    RUN_TEST(test_failed_publish_stays_due);
    RUN_TEST(test_start_needs_wall_clock);
    RUN_TEST(test_derived_and_given_client_id);
    RUN_TEST(test_given_identification_link_and_optional_mam);
    RUN_TEST(test_init_errors);
    RUN_TEST(test_init_trial_encodes_the_birth);
    RUN_TEST(test_level1_ignores_messages);
    RUN_TEST(test_err_strings);
    return UNITY_END();
}
