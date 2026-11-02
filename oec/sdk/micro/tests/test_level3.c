/* Messaging Level 3 with a recording PAL (ADR-0030, C-095): the
 * ReferenceDesignation (Get with Query, Set, Del, persisted) and the common
 * service NewDataSetWriterId. The payloads are validated by D7 in check.sh
 * (tests/emit.c) and against D7 `test --level 3` by the host runner. */
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
static char refdes[1024];

/* base64url of "provider.com/Sensor/S-1/4711" */
#define APP "cHJvdmlkZXIuY29tL1NlbnNvci9TLTEvNDcxMQ"
#define PRE "Oi4v2/OTConnector/" APP
/* base64url of "other.com/X/Y/1" */
#define OTHER "b3RoZXIuY29tL1gvWS8x"
#define REPLY "1793000000000-Registry/other.com/X/Y/1"
#define CALL PRE "/Call/NewDataSetWriterId"
#define RD_GET PRE "/Get/ReferenceDesignation"
#define RD_SET PRE "/Set/ReferenceDesignation/" APP
#define RD_DEL PRE "/Del/ReferenceDesignation/" APP

static double pv;
static const oecm_field fields[] = {
    {.name = "Pv", .type = OECM_TYPE_DOUBLE, .value = &pv, .decimals = 2},
};
static oecm_data data[] = {
    {.tag = "Temperature", .fields = fields, .field_count = 1},
};

/* The DataSetWriterId of the ReferenceDesignation: FIRST (18) + one Data. */
#define RD_WRITER "\"DataSetWriterId\":19"
#define FIRST_WID 20u

void setUp(void) {
    fake_pal_reset(&fp);
    pal = fake_pal_make(&fp);
    pv = 21.5;
    memset(refdes, 0, sizeof refdes);
    memset(&cfg, 0, sizeof cfg);
    cfg.level = OECM_LEVEL_3;
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
    cfg.data_count = 1;
    cfg.refdes_buf = refdes;
    cfg.refdes_cap = sizeof refdes;
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

/* A restart of the device with the persisted records of fp. */
static void restart(void) {
    fp.count = 0;
    memset(refdes, 0, sizeof refdes);
    start_connected();
    fp.count = 0;
}

static fake_record *record_mut(const char *key) {
    for (size_t i = 0; i < FAKE_RECORDS; i++)
        if (fp.records[i].used && strcmp(fp.records[i].key, key) == 0)
            return &fp.records[i];
    return NULL;
}

static bool contains(const char *s, const char *part) { return strstr(s, part) != NULL; }

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

static const char *request(const char *messages) {
    snprintf(msg, sizeof msg,
             "{\"MessageId\":\"" REPLY "\",\"MessageType\":\"ua-data\","
             "\"PublisherId\":\"Registry/other.com/X/Y/1\",\"Messages\":%s}",
             messages);
    return msg;
}

static const char *set_request(const char *payload) {
    static char m[2048];
    snprintf(m, sizeof m, "[{\"DataSetWriterId\":19,\"Payload\":%s}]", payload);
    return request(m);
}

static const fake_event *ev(size_t i) {
    TEST_ASSERT_TRUE(i < fp.count);
    return &fp.events[i];
}

static const char *status(void) {
    const fake_event *e = fake_last(&fp, FAKE_PUBLISH);
    TEST_ASSERT_NOT_NULL(e);
    TEST_ASSERT_TRUE(contains(e->topic, "/Pub/Event/"));
    return e->payload;
}

#define RD_A "{\"Function\":{\"Value\":\"=F1\",\"Parent\":{\"Value\":\"=P\",\"IdentificationLink\":\"other.com/X/Y/1\"}}}"

static void set_refdes(const char *payload) {
    size_t first = send(RD_SET, set_request(payload));
    TEST_ASSERT_TRUE(fp.count > first);
}

/* --- PublicationList, Profile, subscriptions ---------------------------------- */

static void test_connect_lists_level3(void) {
    start_connected();
    TEST_ASSERT_EQUAL_size_t(4, fake_count(&fp, FAKE_SUBSCRIBE));
    bool call = false;
    for (size_t i = 0; i < fp.count; i++)
        call = call || (fp.events[i].kind == FAKE_SUBSCRIBE &&
                        strcmp(fp.events[i].topic, PRE "/Call/#") == 0);
    TEST_ASSERT_TRUE(call);
    bool profile = false, publist = false;
    for (size_t i = 0; i < fp.count; i++) {
        const fake_event *e = &fp.events[i];
        if (e->kind != FAKE_PUBLISH)
            continue;
        if (contains(e->topic, "/Pub/Profile/")) {
            profile = true;
            TEST_ASSERT_TRUE(contains(e->payload, "\"Metadata\",\"ReferenceDesignation\""));
            TEST_ASSERT_TRUE(contains(e->payload, "\"NewDataSetWriterId\"]"));
        }
        if (contains(e->topic, "/Pub/PublicationList")) {
            publist = true;
            TEST_ASSERT_TRUE(contains(e->payload, "\"Resource\":\"ReferenceDesignation\""));
            TEST_ASSERT_TRUE(contains(e->payload, RD_WRITER));
        }
        /* nothing stored: no ReferenceDesignation publication */
        TEST_ASSERT_FALSE(contains(e->topic, "/Pub/ReferenceDesignation/"));
    }
    TEST_ASSERT_TRUE(profile && publist);
}

static void test_level2_lists_unchanged(void) {
    cfg.level = OECM_LEVEL_2;
    start_connected();
    for (size_t i = 0; i < fp.count; i++) {
        const fake_event *e = &fp.events[i];
        TEST_ASSERT_FALSE(contains(e->payload, "ReferenceDesignation"));
        TEST_ASSERT_FALSE(contains(e->payload, "NewDataSetWriterId"));
        TEST_ASSERT_FALSE(contains(e->topic, "/Call/"));
    }
    fp.count = 0;
    send(RD_GET, request("[]"));
    TEST_ASSERT_EQUAL_size_t(0, fp.count);
}

/* --- init ------------------------------------------------------------------- */

static void test_init_arguments(void) {
    cfg.refdes_buf = NULL;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    cfg.refdes_buf = refdes;
    cfg.refdes_cap = OECM_REFDES_MIN - 1;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    cfg.refdes_cap = sizeof refdes;
    pal.persist_save = NULL;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    pal = fake_pal_make(&fp);
    pal.persist_load = NULL;
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    pal = fake_pal_make(&fp);
    fp.load_result = OECM_E_PAL;
    TEST_ASSERT_EQUAL(OECM_E_PAL, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
    fp.load_result = OECM_OK;
    /* a buffer too small for the minimum */
    static uint8_t small[512];
    TEST_ASSERT_EQUAL(OECM_E_NOSPACE, oecm_init(&dev, &cfg, &pal, small, sizeof small));
    TEST_ASSERT_EQUAL(OECM_OK, oecm_init(&dev, &cfg, &pal, tx, sizeof tx));
}

static void test_init_ignores_invalid_stored_refdes(void) {
    start_connected();
    set_refdes(RD_A);
    fake_record *r = record_mut("oecm.refdes");
    TEST_ASSERT_NOT_NULL(r);
    /* not canonical (other member order): ignored */
    const char *bad = "{\"Function\":{\"Parent\":{\"Value\":\"=P\"},\"Value\":\"=F1\"}}";
    memcpy(r->data, bad, strlen(bad));
    r->len = strlen(bad);
    restart();
    send(RD_GET, request("[]"));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Messages\":[]"));
}

/* --- Set, Get, Del ------------------------------------------------------------ */

static void test_set_publishes_answers_and_persists(void) {
    start_connected();
    fp.count = 0;
    send(RD_SET, set_request("{\"Location\":{\"Value\":\"+L\",\"Local\":null,\"X\":1},"
                             "\"Function\":{\"Local\":\"f\",\"Value\":\"=F1\"},\"Product\":null}"));
    /* the answer, the Status event, then the publication on change */
    TEST_ASSERT_EQUAL_size_t(2, fake_count(&fp, FAKE_PUBLISH));
    const char *canon =
        "{\"Function\":{\"Value\":\"=F1\",\"Local\":\"f\"},\"Location\":{\"Value\":\"+L\"}}";
    TEST_ASSERT_TRUE(contains(ev(0)->topic, "/Pub/ReferenceDesignation/" APP));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, REPLY));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, canon));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, RD_WRITER));
    TEST_ASSERT_TRUE(contains(ev(1)->topic, "/Pub/Event/" APP "/Status/Good"));
    const fake_record *r = fake_record_of(&fp, "oecm.refdes");
    TEST_ASSERT_NOT_NULL(r);
    TEST_ASSERT_EQUAL_size_t(strlen(canon), r->len);
    TEST_ASSERT_EQUAL_MEMORY(canon, r->data, r->len);
    fp.count = 0;
    oecm_poll(&dev);
    TEST_ASSERT_EQUAL_size_t(1, fake_count(&fp, FAKE_PUBLISH));
    TEST_ASSERT_TRUE(contains(ev(0)->topic, "/Pub/ReferenceDesignation/" APP));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"ReplyTo\":\"\""));
    /* unchanged: answered, not published again, not saved again */
    size_t saves = fp.saves;
    fp.count = 0;
    send(RD_SET, set_request(canon));
    oecm_poll(&dev);
    TEST_ASSERT_EQUAL_size_t(2, fake_count(&fp, FAKE_PUBLISH));
    TEST_ASSERT_EQUAL_size_t(saves, fp.saves);
    /* persisted across a restart; published only on change (as Rust) */
    fp.count = 0;
    memset(refdes, 0, sizeof refdes);
    start_connected();
    oecm_poll(&dev);
    for (size_t i = 0; i < fp.count; i++)
        TEST_ASSERT_FALSE(contains(fp.events[i].topic, "/Pub/ReferenceDesignation/"));
    fp.count = 0;
    send(RD_GET, request("[]"));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, canon));
}

static void test_set_errors(void) {
    start_connected();
    fp.count = 0;
    send(RD_SET, set_request("[1]"));
    TEST_ASSERT_TRUE(contains(status(), "BadDecoding"));
    send(RD_SET, set_request("{\"Function\":{\"Local\":\"x\"}}"));
    TEST_ASSERT_TRUE(contains(status(), "BadDecoding"));
    send(RD_SET, set_request("{\"Function\":{\"Value\":null}}"));
    TEST_ASSERT_TRUE(contains(status(), "BadDecoding"));
    send(RD_SET, set_request("{\"Function\":{\"Value\":\"=F\",\"Parent\":{\"Local\":\"x\"}}}"));
    TEST_ASSERT_TRUE(contains(status(), "BadDecoding"));
    send(RD_SET, set_request("{\"Function\":{\"Value\":1}}"));
    TEST_ASSERT_TRUE(contains(status(), "BadDecoding"));
    TEST_ASSERT_NULL(fake_record_of(&fp, "oecm.refdes"));
    /* larger than the buffer of the device */
    static char big[1400];
    static char obj[1500];
    memset(big, 'a', sizeof big - 1);
    snprintf(obj, sizeof obj, "{\"Function\":{\"Value\":\"%s\"}}", big);
    send(RD_SET, set_request(obj));
    TEST_ASSERT_TRUE(contains(status(), "BadOutOfRange"));
    /* persisting fails: nothing changes */
    fp.save_result = OECM_E_PAL;
    send(RD_SET, set_request(RD_A));
    TEST_ASSERT_TRUE(contains(status(), "BadInternalError"));
    fp.save_result = OECM_OK;
    fp.count = 0;
    send(RD_GET, request("[]"));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Messages\":[]"));
    /* foreign Source */
    send(PRE "/Set/ReferenceDesignation/" OTHER, set_request(RD_A));
    TEST_ASSERT_TRUE(contains(status(), "BadNotFound"));
}

static void test_get_with_query(void) {
    start_connected();
    set_refdes(RD_A);
    fp.count = 0;
    /* matching Parent, any aspect */
    send(RD_GET, request("[{\"DataSetWriterId\":9,\"Payload\":{\"Parent\":\"other.com/X/Y/1\"}}]"));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "=F1"));
    /* matching aspect */
    fp.count = 0;
    send(RD_GET, request("[{\"DataSetWriterId\":9,\"Payload\":{\"Parent\":\"other.com/X/Y/1\","
                         "\"Aspect\":\"Function\"}}]"));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "=F1"));
    /* other aspect, other Parent: an empty answer */
    fp.count = 0;
    send(RD_GET, request("[{\"DataSetWriterId\":9,\"Payload\":{\"Parent\":\"other.com/X/Y/1\","
                         "\"Aspect\":\"Location\"}}]"));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Messages\":[]"));
    fp.count = 0;
    send(RD_GET, request("[{\"DataSetWriterId\":9,\"Payload\":{\"Parent\":\"x\"}}]"));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Messages\":[]"));
    /* an invalid Query: no answer */
    fp.count = 0;
    send(RD_GET, request("[{\"DataSetWriterId\":9,\"Payload\":{\"Aspect\":\"Function\"}}]"));
    send(RD_GET, request("[{\"DataSetWriterId\":9,\"Payload\":{\"Parent\":\"x\",\"Aspect\":\"F\"}}]"));
    send(RD_GET, request("[{\"DataSetWriterId\":9,\"Payload\":{\"Parent\":1}}]"));
    TEST_ASSERT_EQUAL_size_t(0, fp.count);
    /* a Payload that is not an object: no Query */
    send(RD_GET, request("[{\"DataSetWriterId\":9,\"Payload\":null}]"));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "=F1"));
    /* with the own Source the Query is ignored */
    fp.count = 0;
    send(RD_GET "/" APP, request("[{\"DataSetWriterId\":9,\"Payload\":{\"Parent\":\"x\"}}]"));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "=F1"));
    /* a Filter: ignored */
    fp.count = 0;
    send(RD_GET "/" APP "/x", request("[]"));
    TEST_ASSERT_EQUAL_size_t(0, fp.count);
}

static void test_del(void) {
    start_connected();
    fp.count = 0;
    send(RD_DEL, request("[]"));
    TEST_ASSERT_TRUE(contains(status(), "BadNotFound"));
    set_refdes(RD_A);
    fp.save_result = OECM_E_PAL;
    send(RD_DEL, request("[]"));
    TEST_ASSERT_TRUE(contains(status(), "BadInternalError"));
    fp.save_result = OECM_OK;
    send(RD_DEL, request("[]"));
    TEST_ASSERT_FALSE(contains(status(), "Bad"));
    TEST_ASSERT_NULL(fake_record_of(&fp, "oecm.refdes"));
    fp.count = 0;
    send(RD_GET, request("[]"));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Messages\":[]"));
    restart();
    send(RD_GET, request("[]"));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Messages\":[]"));
}

/* --- NewDataSetWriterId -------------------------------------------------------- */

static char call_msg[32768];

/* A ua-action-request; extra is inserted into the NetworkMessage. */
static const char *call_req(const char *reply_topic, const char *extra, const char *messages) {
    snprintf(call_msg, sizeof call_msg,
             "{\"MessageId\":\"" REPLY "\",\"MessageType\":\"ua-action-request\","
             "\"PublisherId\":\"OTConnector/provider.com/Sensor/S-1/4711\","
             "\"Timestamp\":\"2026-11-11T10:00:00.000Z\",\"ResponseAddress\":\"%s\","
             "\"RequestorId\":\"Registry/other.com/X/Y/1\",\"TimeoutHint\":5000%s,"
             "\"Messages\":%s}",
             reply_topic, extra, messages);
    return call_msg;
}

#define ITEM(rid, res)                                                                   \
    "{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":" #rid                    \
    ",\"ActionState\":1,\"Payload\":{\"Resource\":\"" res "\"}}"
#define ITEM_FMT                                                                          \
    "{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":%u,\"ActionState\":1,"     \
    "\"Payload\":{\"Resource\":\"Data\"}}"
#define REPLY_TOPIC PRE "/Reply/NewDataSetWriterId"

static size_t simple_call(void) {
    return send(CALL, call_req(REPLY_TOPIC, "", "[" ITEM(1, "Data") "]"));
}

static void test_call_answers_new_ids(void) {
    start_connected();
    fp.count = 0;
    send(CALL, call_req(REPLY_TOPIC, ",\"CorrelationData\":\"QUJD\"",
                        "[" ITEM(7, "Data") "," ITEM(8, "Config") "]"));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    const fake_event *e = ev(0);
    TEST_ASSERT_EQUAL_STRING(REPLY_TOPIC, e->topic);
    TEST_ASSERT_TRUE(contains(e->payload, "\"MessageType\":\"ua-action-response\""));
    TEST_ASSERT_TRUE(contains(e->payload, "\"RequestorId\":\"Registry/other.com/X/Y/1\""));
    TEST_ASSERT_TRUE(contains(e->payload, "\"CorrelationData\":\"QUJD\""));
    TEST_ASSERT_TRUE(contains(e->payload, "\"RequestId\":7,\"Status\":{},\"ActionState\":2,"
                                          "\"Payload\":{\"DataSetWriterId\":20,\"Ttl\":60}"));
    TEST_ASSERT_TRUE(contains(e->payload, "\"RequestId\":8,\"Status\":{},\"ActionState\":2,"
                                          "\"Payload\":{\"DataSetWriterId\":21,\"Ttl\":60}"));
    /* the limit is persisted before an id is given */
    const fake_record *r = fake_record_of(&fp, "oecm.wid");
    TEST_ASSERT_NOT_NULL(r);
    TEST_ASSERT_EQUAL_size_t(4, r->len);
    TEST_ASSERT_EQUAL_UINT8(FIRST_WID + 256u, r->data[0] | r->data[1] << 8);
    /* the own Source; the Reply topic with the Source */
    fp.count = 0;
    send(CALL "/" APP, call_req(REPLY_TOPIC "/" APP, "", "[" ITEM(9, "ReferenceDesignation") "]"));
    TEST_ASSERT_EQUAL_STRING(REPLY_TOPIC "/" APP, ev(0)->topic);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"DataSetWriterId\":22,"));
    TEST_ASSERT_FALSE(contains(ev(0)->payload, "CorrelationData"));
}

static void test_call_ids_not_reused_after_restart(void) {
    start_connected();
    fp.count = 0;
    simple_call();
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"DataSetWriterId\":20,\"Ttl\""));
    restart();
    simple_call();
    /* the next block: the persisted limit */
    char want[64];
    snprintf(want, sizeof want, "\"DataSetWriterId\":%u,\"Ttl\"", FIRST_WID + 256u);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, want));
    /* an invalid persisted limit: from the first id */
    fake_record *r = record_mut("oecm.wid");
    r->data[0] = 1;
    r->data[1] = 0;
    restart();
    simple_call();
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"DataSetWriterId\":20,\"Ttl\""));
}

static void test_call_errors(void) {
    start_connected();
    fp.count = 0;
    /* an unknown Source */
    send(CALL "/" OTHER, call_req(REPLY_TOPIC "/" OTHER, "", "[" ITEM(1, "Data") "]"));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Status\":{\"Code\":2151546880},\"ActionState\":2,\"Payload\":{}"));
    /* an unknown Resource, no Resource */
    fp.count = 0;
    send(CALL, call_req(REPLY_TOPIC, "",
                        "[" ITEM(1, "Foo") ",{\"DataSetWriterId\":8,\"ActionTargetId\":1,"
                        "\"RequestId\":2,\"ActionState\":1,\"Payload\":null}]"));
    TEST_ASSERT_EQUAL_size_t(1, fp.count);
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"RequestId\":1,\"Status\":{\"Code\":2158690304}"));
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"RequestId\":2,\"Status\":{\"Code\":2158690304}"));
    /* persisting fails */
    fp.save_result = OECM_E_PAL;
    fp.count = 0;
    simple_call();
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Status\":{\"Code\":2147614720}"));
    fp.save_result = OECM_OK;
}

static void test_call_rate_limit(void) {
    start_connected();
    for (unsigned i = 0; i < 256; i++) {
        fp.count = 0;
        simple_call();
        TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Status\":{}"));
    }
    fp.count = 0;
    simple_call();
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Status\":{\"Code\":2147745792}"));
    fp.mono_ms += 60000;
    fp.count = 0;
    simple_call();
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"Status\":{}"));
    /* 257 ids: two blocks persisted */
    TEST_ASSERT_EQUAL_UINT32(FIRST_WID + 512u,
                             fake_record_of(&fp, "oecm.wid")->data[0] |
                                 (uint32_t)fake_record_of(&fp, "oecm.wid")->data[1] << 8);
}

static void test_call_ignored(void) {
    start_connected();
    fp.count = 0;
    const char *item = "[" ITEM(1, "Data") "]";
    /* the wrong PublisherId, ResponseAddress, Timestamp, CorrelationData */
    send(CALL, call_req(PRE "/Reply/Other", "", item));
    send(CALL, call_req(REPLY_TOPIC "/" APP, "", item));
    send(CALL, call_req(REPLY_TOPIC, ",\"CorrelationData\":\"QUJ\"", item));
    send(CALL, call_req(REPLY_TOPIC, ",\"CorrelationData\":\"QU=D\"", item));
    send(CALL, call_req(REPLY_TOPIC, ",\"CorrelationData\":1", item));
    /* a duplicate field */
    send(CALL, call_req(REPLY_TOPIC, ",\"TimeoutHint\":5", item));
    send(CALL, call_req(REPLY_TOPIC, ",\"CorrelationData\":\"QUJD\",\"CorrelationData\":null", item));
    /* no request for the service, ActionState 2, an invalid item */
    send(CALL, call_req(REPLY_TOPIC, "", "[]"));
    send(CALL, call_req(REPLY_TOPIC, "",
                        "[{\"DataSetWriterId\":8,\"ActionTargetId\":2,\"RequestId\":1,"
                        "\"ActionState\":1,\"Payload\":{}}]"));
    send(CALL, call_req(REPLY_TOPIC, "",
                        "[{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":1,"
                        "\"ActionState\":2,\"Payload\":{}}]"));
    send(CALL, call_req(REPLY_TOPIC, "",
                        "[{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":70000,"
                        "\"ActionState\":1,\"Payload\":{}}]"));
    send(CALL, call_req(REPLY_TOPIC, "",
                        "[{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":1,"
                        "\"ActionState\":1}]"));
    send(CALL, call_req(REPLY_TOPIC, "",
                        "[{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":1,"
                        "\"ActionState\":1,\"Payload\":{},\"Timestamp\":\"2026-02-30T00:00:00Z\"}]"));
    /* the wrong service, a Filter */
    send(PRE "/Call/Other", call_req(PRE "/Reply/Other", "", item));
    send(CALL "/" APP "/x", call_req(REPLY_TOPIC "/" APP "/x", "", item));
    send(CALL, "not json");
    TEST_ASSERT_EQUAL_size_t(0, fp.count);
    /* the PublisherId of another application */
    snprintf(call_msg, sizeof call_msg,
             "{\"MessageId\":\"" REPLY "\",\"MessageType\":\"ua-action-request\","
             "\"PublisherId\":\"OTConnector/other.com/X/Y/1\","
             "\"Timestamp\":\"2026-11-11T10:00:00Z\",\"ResponseAddress\":\"" REPLY_TOPIC "\","
             "\"RequestorId\":\"Registry/other.com/X/Y/1\",\"TimeoutHint\":5000,"
             "\"Messages\":%s}",
             item);
    send(CALL, call_msg);
    /* the wrong MessageType, no TimeoutHint, a zero TimeoutHint */
    snprintf(call_msg, sizeof call_msg,
             "{\"MessageId\":\"" REPLY "\",\"MessageType\":\"ua-data\","
             "\"PublisherId\":\"OTConnector/provider.com/Sensor/S-1/4711\","
             "\"Timestamp\":\"2026-11-11T10:00:00Z\",\"ResponseAddress\":\"" REPLY_TOPIC "\","
             "\"RequestorId\":\"Registry/other.com/X/Y/1\",\"TimeoutHint\":5000,"
             "\"Messages\":%s}",
             item);
    send(CALL, call_msg);
    snprintf(call_msg, sizeof call_msg,
             "{\"MessageId\":\"" REPLY "\",\"MessageType\":\"ua-action-request\","
             "\"PublisherId\":\"OTConnector/provider.com/Sensor/S-1/4711\","
             "\"Timestamp\":\"2026-11-11T10:00:00Z\",\"ResponseAddress\":\"" REPLY_TOPIC "\","
             "\"RequestorId\":\"Registry/other.com/X/Y/1\",\"TimeoutHint\":0,"
             "\"Messages\":%s}",
             item);
    send(CALL, call_msg);
    snprintf(call_msg, sizeof call_msg,
             "{\"MessageId\":\"" REPLY "\",\"MessageType\":\"ua-action-request\","
             "\"PublisherId\":\"OTConnector/provider.com/Sensor/S-1/4711\","
             "\"Timestamp\":\"2026-11-11T10:00:00Z\",\"ResponseAddress\":\"" REPLY_TOPIC "\","
             "\"RequestorId\":\"Nobody/other.com/X/Y/1\",\"TimeoutHint\":1,"
             "\"Messages\":%s}",
             item);
    send(CALL, call_msg);
    TEST_ASSERT_EQUAL_size_t(0, fp.count);
    /* no id was taken */
    simple_call();
    TEST_ASSERT_TRUE(contains(ev(0)->payload, "\"DataSetWriterId\":20,\"Ttl\""));
}

static void test_call_batches(void) {
    start_connected();
    /* 200 requests: more than fit into one message of the tx buffer */
    static char items[32768];
    size_t n = 0;
    items[n++] = '[';
    for (unsigned i = 0; i < 200; i++)
        n += (size_t)snprintf(items + n, sizeof items - n, "%s" ITEM_FMT, i ? "," : "", i + 1);
    items[n++] = ']';
    items[n] = '\0';
    fp.count = 0;
    send(CALL, call_req(REPLY_TOPIC, "", items));
    TEST_ASSERT_TRUE(fp.count >= 2);
    unsigned seen = 0;
    char last_id[96] = "";
    for (size_t i = 0; i < fp.count; i++) {
        const fake_event *e = ev(i);
        TEST_ASSERT_EQUAL_STRING(REPLY_TOPIC, e->topic);
        const char *p = e->payload;
        while ((p = strstr(p, "\"Ttl\":60")) != NULL) {
            seen++;
            p++;
        }
        /* a MessageId per message */
        const char *id = strstr(e->payload, "\"MessageId\":\"");
        TEST_ASSERT_NOT_NULL(id);
        TEST_ASSERT_TRUE(strncmp(id, last_id, 40) != 0);
        snprintf(last_id, sizeof last_id, "%.40s", id);
        TEST_ASSERT_TRUE(e->len < sizeof tx);
    }
    TEST_ASSERT_EQUAL_UINT(200, seen);
}

/* --- DateTime ------------------------------------------------------------------- */

static void test_date_time(void) {
    static const char *const good[] = {
        "2026-11-11T10:00:00Z", "2024-02-29T23:59:59.123456789Z", "2026-11-11T10:00:00+01:00",
        "2026-11-11T10:00:00.5-23:59",
    };
    static const char *const bad[] = {
        "2026-11-11T10:00:60Z", "2026-11-11t10:00:00Z", "2026-11-11T10:00:00z",
        "2025-02-29T10:00:00Z", "2026-13-01T10:00:00Z", "2026-11-11T24:00:00Z",
        "2026-11-11T10:00:00", "2026-11-11T10:00:00.Z", "2026-11-11T10:00:00+24:00",
        "2026-11-11 10:00:00Z", "2026-11-00T10:00:00Z", "2026-11-11T10:00:00+0100",
    };
    for (size_t i = 0; i < sizeof good / sizeof good[0]; i++)
        TEST_ASSERT_TRUE_MESSAGE(oecm_date_time_ok(good[i], strlen(good[i])), good[i]);
    for (size_t i = 0; i < sizeof bad / sizeof bad[0]; i++)
        TEST_ASSERT_FALSE_MESSAGE(oecm_date_time_ok(bad[i], strlen(bad[i])), bad[i]);
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_connect_lists_level3);
    RUN_TEST(test_level2_lists_unchanged);
    RUN_TEST(test_init_arguments);
    RUN_TEST(test_init_ignores_invalid_stored_refdes);
    RUN_TEST(test_set_publishes_answers_and_persists);
    RUN_TEST(test_set_errors);
    RUN_TEST(test_get_with_query);
    RUN_TEST(test_del);
    RUN_TEST(test_call_answers_new_ids);
    RUN_TEST(test_call_ids_not_reused_after_restart);
    RUN_TEST(test_call_errors);
    RUN_TEST(test_call_rate_limit);
    RUN_TEST(test_call_ignored);
    RUN_TEST(test_call_batches);
    RUN_TEST(test_date_time);
    return UNITY_END();
}
