/* Test program of the C ABI (ADR-0021): argument and state errors, a
 * Level 2 provider and a Level 2 consumer in one process on the test
 * broker (Get, Set, Del, NewDataSetWriterId, Set handlers, subscriptions),
 * the callback rules and a stress phase with several threads. Run by
 * oec-ffi/tests/c.rs, also under AddressSanitizer and UBSan.
 *
 * Usage: abi <port> <ca.pem> <work dir> */
#define _POSIX_C_SOURCE 200809L
#include <pthread.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "oec.h"

#define CHECK(cond)                                                                        \
    do {                                                                                   \
        if (!(cond)) {                                                                     \
            fprintf(stderr, "%s:%d: CHECK(%s) failed; last error: %s\n", __FILE__,         \
                    __LINE__, #cond, oec_last_error());                                    \
            exit(1);                                                                       \
        }                                                                                  \
    } while (0)
#define OK(call) CHECK((call) == OEC_OK)
#define FAILS(call, status) CHECK((call) == (status))

/* The Oi4Identifier of the provider: product and serial number. */
#define PROVIDER "example.com/Abi/ABI-1/provider"
#define WAIT_MS 15000

static const char *port_arg, *ca_arg, *work_arg;

static void sleep_ms(long ms) {
    struct timespec t = {ms / 1000, (ms % 1000) * 1000000L};
    nanosleep(&t, NULL);
}

static long now_ms(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return (long)t.tv_sec * 1000 + t.tv_nsec / 1000000;
}

static oec_product *product(void) {
    oec_product *p = NULL;
    OK(oec_product_new("example.com", "Abi", "ABI-1", &p));
    OK(oec_product_set(p, "SoftwareRevision", "1.0.0"));
    return p;
}

/* An app on the test broker with an explicit broker and its own store. */
static oec_app *app(oec_level level, const char *serial) {
    oec_product *p = product();
    oec_builder *b = NULL;
    oec_app *a = NULL;
    char store[1024];
    OK(oec_builder_new(level, OEC_ST_UTILITY, p, &b));
    oec_product_free(p);
    OK(oec_builder_broker(b, "localhost", (uint16_t)atoi(port_arg), ca_arg));
    OK(oec_builder_credentials(b, "user", "pa:ss"));
    OK(oec_builder_serial_number(b, serial));
    snprintf(store, sizeof store, "%s/store-%s", work_arg, serial);
    OK(oec_builder_store_dir(b, store));
    OK(oec_builder_syslog_level(b, OEC_SYSLOG_DEBUG));
    OK(oec_builder_build(b, &a));
    return a;
}

/* ---- errors without a connection -------------------------------------- */

static void ignore(void *user_data, const oec_received *msg) {
    (void)user_data;
    (void)msg;
}

static void argument_errors(void) {
    oec_product *p = NULL;
    oec_builder *b = NULL;
    oec_app *a = NULL;

    CHECK(oec_abi_version() == OEC_ABI_VERSION);
    CHECK(strlen(oec_version()) > 0);
    FAILS(oec_product_new(NULL, "M", "C", &p), OEC_ERR_INVALID_ARGUMENT);
    CHECK(strstr(oec_last_error(), "is NULL") != NULL);
    CHECK(p == NULL);
    FAILS(oec_product_new("example.com", "M", "C", NULL), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_product_new("example.com", "\xff", "C", &p), OEC_ERR_INVALID_ARGUMENT);
    CHECK(strstr(oec_last_error(), "UTF-8") != NULL);
    p = product();
    FAILS(oec_product_set(p, "Model", "x"), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_product_set(NULL, "Manufacturer", "x"), OEC_ERR_INVALID_ARGUMENT);

    FAILS(oec_builder_new(0, OEC_ST_UTILITY, p, &b), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_builder_new(OEC_LEVEL_1, OEC_ST_ANY, p, &b), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_builder_new(OEC_LEVEL_1, 7, p, &b), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_builder_new(OEC_LEVEL_1, OEC_ST_UTILITY, NULL, &b), OEC_ERR_INVALID_ARGUMENT);
    OK(oec_builder_new(OEC_LEVEL_1, OEC_ST_UTILITY, p, &b));
    FAILS(oec_builder_credentials(b, "u", "p"), OEC_ERR_INVALID_STATE);
    FAILS(oec_builder_client_cert(b, "c", "k"), OEC_ERR_INVALID_STATE);
    FAILS(oec_builder_syslog_level(b, 8), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_builder_identification_link(b, "no IL"), OEC_ERR_INVALID_ARGUMENT);
    oec_builder_free(b);
    oec_builder_free(NULL);
    FAILS(oec_builder_build(NULL, &a), OEC_ERR_INVALID_ARGUMENT);

    /* a missing container storage: the builder is consumed anyway */
    OK(oec_builder_new(OEC_LEVEL_1, OEC_ST_UTILITY, p, &b));
    OK(oec_builder_container_storage(b, "/nonexistent/oec-root"));
    FAILS(oec_builder_build(b, &a), OEC_ERR_BOOTSTRAP);
    CHECK(a == NULL);
    oec_product_free(p);

    oec_product_free(NULL);
    oec_app_free(NULL);
    oec_app_shutdown(NULL);
    oec_subscription_free(NULL);
    oec_string_free(NULL);
    CHECK(oec_app_this(NULL) == NULL);
    CHECK(strstr(oec_last_error(), "app is NULL") != NULL);
}

static void state_errors(void) {
    oec_app *a = app(OEC_LEVEL_1, "level-1");
    oec_asset *this = oec_app_this(a), *asset = NULL;
    oec_dataset *ds = NULL;
    oec_subscription *sub = NULL;
    oec_product *p = product();
    oec_status_code code = 7;

    FAILS(oec_asset_set_health(this, OEC_HEALTH_NORMAL, 101), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_asset_set_health(this, 5, 0), OEC_ERR_INVALID_ARGUMENT);
    OK(oec_asset_set_health(this, OEC_HEALTH_OFF_SPEC, -1));
    FAILS(oec_asset_remove(this), OEC_ERR_INVALID_ARGUMENT);

    OK(oec_app_add_asset(a, NULL, p, "0815", &asset));
    FAILS(oec_app_add_asset(a, "no IL", p, "0816", &asset), OEC_ERR_INVALID_ARGUMENT);
    CHECK(asset == NULL);
    OK(oec_app_add_asset(a, "vendor.com/Pump/P-1/0817", p, "0817", &asset));
    OK(oec_asset_emit_event(asset, "Maintenance", 7, "filter", "{\"Hours\":12}"));
    OK(oec_asset_emit_event(asset, "Maintenance", 8, NULL, NULL));
    FAILS(oec_asset_emit_event(asset, "Maintenance", 9, NULL, "[1]"), OEC_ERR_INVALID_ARGUMENT);
    OK(oec_asset_remove(asset));
    FAILS(oec_asset_remove(asset), OEC_ERR_INVALID_ARGUMENT); /* still a valid handle */
    oec_product_free(p);

    FAILS(oec_asset_add_data(this, "t", "[{\"Name\":\"A\",\"Type\":\"Real\"}]", 0, 0,
                             OEC_WRITABLE_NOTHING, &ds),
          OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_asset_add_data(this, "t", "[]", 16, 0, OEC_WRITABLE_NOTHING, &ds),
          OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_asset_add_data(this, "t", "[]", 0, 0, 4, &ds), OEC_ERR_INVALID_ARGUMENT);
    OK(oec_asset_add_data(this, "t", "[{\"Name\":\"A\",\"Type\":\"Int32\"}]",
                          OEC_MODE_ON_REQUEST, 0, OEC_WRITABLE_NOTHING, &ds));
    FAILS(oec_dataset_update(ds, "{\"A\":\"x\"}"), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_dataset_update(ds, "{\"B\":1}"), OEC_ERR_INVALID_ARGUMENT);
    OK(oec_dataset_update(ds, "{\"A\":1}"));
    FAILS(oec_dataset_on_set(ds, NULL, NULL), OEC_ERR_INVALID_ARGUMENT);
    OK(oec_dataset_remove(ds));
    FAILS(oec_dataset_update(ds, "{\"A\":2}"), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_app_syslog(a, 8, "x"), OEC_ERR_INVALID_ARGUMENT);
    OK(oec_app_syslog(a, OEC_SYSLOG_NOTICE, "offline: dropped"));

    FAILS(oec_subscribe(a, 10, OEC_ST_ANY, NULL, NULL, NULL, ignore, NULL, &sub),
          OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_subscribe(a, OEC_RES_DATA, OEC_ST_ANY, NULL, NULL, NULL, NULL, NULL, &sub),
          OEC_ERR_INVALID_ARGUMENT);
    CHECK(sub == NULL);
    FAILS(oec_get(a, OEC_RES_MAM, OEC_ST_UTILITY, PROVIDER, NULL, NULL, 100, ignore, NULL),
          OEC_ERR_NOT_CONNECTED);
    FAILS(oec_set(a, OEC_RES_DATA, OEC_ST_UTILITY, PROVIDER, PROVIDER, "t", "{}", 100, &code),
          OEC_ERR_NOT_CONNECTED);
    CHECK(code == 7);

    /* shut down before the run: the run returns at once; once per app */
    oec_app_shutdown(a);
    OK(oec_app_run(a));
    FAILS(oec_app_run(a), OEC_ERR_INVALID_STATE);
    FAILS(oec_app_start(a), OEC_ERR_INVALID_STATE);
    oec_app_free(a);
}

/* ---- provider ---------------------------------------------------------- */

static atomic_int data_sets, config_sets, blocked_in_handler;
static char last_config[512];

/* Accepts Count <= 100; tries a blocking call (refused in a callback). */
static oec_status_code on_set_data(void *user_data, const char *json) {
    (void)user_data;
    atomic_fetch_add(&data_sets, 1);
    if (oec_get(NULL, OEC_RES_MAM, OEC_ST_UTILITY, PROVIDER, NULL, NULL, 10, ignore, NULL) ==
        OEC_ERR_INVALID_STATE)
        atomic_fetch_add(&blocked_in_handler, 1);
    const char *count = strstr(json, "\"Count\":");
    if (count == NULL)
        return OEC_BAD;
    return strtoul(count + 8, NULL, 10) <= 100 ? OEC_GOOD : OEC_BAD_OUT_OF_RANGE;
}

static oec_status_code on_set_config(void *user_data, const char *json) {
    (void)user_data;
    snprintf(last_config, sizeof last_config, "%s", json);
    atomic_fetch_add(&config_sets, 1);
    return strstr(json, "\"Nowhere\"") ? OEC_BAD_OUT_OF_RANGE : OEC_GOOD;
}

struct provider {
    oec_app *app;
    oec_dataset *oee;
    oec_config *cfg;
};

static struct provider provider(void) {
    struct provider p = {app(OEC_LEVEL_2, "provider"), NULL, NULL};
    oec_asset *this = oec_app_this(p.app);
    char *value = NULL;

    OK(oec_asset_add_data(this, "oee",
                          "[{\"Name\":\"Availability\",\"Type\":\"Double\",\"Unit\":\"%\","
                          "\"Range\":{\"Low\":0,\"High\":100},\"Precision\":0.5},"
                          "{\"Name\":\"Count\",\"Type\":\"UInt32\",\"Description\":\"parts\"}]",
                          OEC_MODE_ON_REQUEST | OEC_MODE_FILTER, 0, OEC_WRITABLE_MODE_AND_INTERVAL,
                          &p.oee));
    OK(oec_dataset_update(p.oee, "{\"Availability\":97.5,\"Count\":1}"));
    OK(oec_dataset_on_set(p.oee, on_set_data, NULL));

    FAILS(oec_asset_add_config(this, "settings", "{\"common\": 1}", &p.cfg),
          OEC_ERR_INVALID_ARGUMENT);
    OK(oec_asset_add_config(this, "settings",
                            "{\"common\":{\"Name\":{\"Locale\":\"en-US\",\"Text\":\"Common\"},"
                            "\"city\":{\"Type\":\"String\",\"Value\":\"Berlin\","
                            "\"Name\":{\"Locale\":\"en-US\",\"Text\":\"City\"}}}}",
                            &p.cfg));
    OK(oec_config_get(p.cfg, "common", "city", &value));
    CHECK(value != NULL && strcmp(value, "Berlin") == 0);
    oec_string_free(value);
    OK(oec_config_get(p.cfg, "common", "unknown", &value));
    CHECK(value == NULL);
    FAILS(oec_config_set(p.cfg, "common", "unknown", "x"), OEC_ERR_INVALID_ARGUMENT);
    OK(oec_config_set(p.cfg, "common", "city", "Hamburg"));
    OK(oec_config_get(p.cfg, "common", "city", &value));
    CHECK(value != NULL && strcmp(value, "Hamburg") == 0);
    oec_string_free(value);
    OK(oec_config_on_set(p.cfg, on_set_config, NULL));
    return p;
}

/* ---- consumer ---------------------------------------------------------- */

#define MAX_MSGS 8

struct got {
    int n;
    oec_resource resource[MAX_MSGS];
    char service_type[MAX_MSGS][32];
    char app[MAX_MSGS][128];
    char source[MAX_MSGS][128];
    char filter[MAX_MSGS][64];
    uint16_t writer[MAX_MSGS];
    char payload[MAX_MSGS][2048];
};

static void collect(void *user_data, const oec_received *msg) {
    struct got *g = user_data;
    if (g->n >= MAX_MSGS)
        return;
    g->resource[g->n] = msg->resource;
    snprintf(g->service_type[g->n], 32, "%s", msg->service_type);
    snprintf(g->app[g->n], 128, "%s", msg->app);
    snprintf(g->source[g->n], 128, "%s", msg->source ? msg->source : "");
    snprintf(g->filter[g->n], 64, "%s", msg->filter ? msg->filter : "");
    g->writer[g->n] = msg->data_set_writer_id;
    snprintf(g->payload[g->n], 2048, "%s", msg->payload_json);
    g->n++;
}

static struct got *get(oec_app *c, oec_resource r, const char *filter) {
    static _Thread_local struct got g;
    memset(&g, 0, sizeof g);
    OK(oec_get(c, r, OEC_ST_UTILITY, PROVIDER, filter ? PROVIDER : NULL, filter, 0, collect, &g));
    return &g;
}

/* Both apps online: a Get of the provider's MAM is answered. */
static void wait_online(oec_app *c) {
    long start = now_ms();
    struct got *g = calloc(1, sizeof *g);
    CHECK(g != NULL);
    for (;;) {
        memset(g, 0, sizeof *g);
        oec_status st =
            oec_get(c, OEC_RES_MAM, OEC_ST_UTILITY, PROVIDER, NULL, NULL, 200, collect, g);
        if (st == OEC_OK && g->n == 1)
            break;
        CHECK(st == OEC_ERR_NOT_CONNECTED || st == OEC_ERR_TIMEOUT);
        CHECK(now_ms() - start < WAIT_MS);
        sleep_ms(5);
    }
    free(g);
}

static atomic_int published, freed_in_callback, free_refused;
static oec_app *consumer_app;

static void count_data(void *user_data, const oec_received *msg) {
    (void)user_data;
    if (msg->resource == OEC_RES_DATA)
        atomic_fetch_add(&published, 1);
    /* refused inside a callback: nothing is released */
    oec_app_free(consumer_app);
    if (strstr(oec_last_error(), "callback") != NULL)
        atomic_store(&free_refused, 1);
}

static void free_myself(void *user_data, const oec_received *msg) {
    oec_subscription **self = user_data;
    (void)msg;
    if (*self != NULL) {
        oec_subscription_free(*self);
        *self = NULL;
        atomic_fetch_add(&freed_in_callback, 1);
    }
}

static void wait_for(atomic_int *v, const char *what) {
    long start = now_ms();
    while (atomic_load(v) == 0) {
        if (now_ms() - start >= WAIT_MS) {
            fprintf(stderr, "not reached: %s\n", what);
            exit(1);
        }
        sleep_ms(5);
    }
}

static void data_requests(oec_app *c) {
    struct got *g;
    oec_status_code code = 7;

    g = get(c, OEC_RES_DATA, "oee");
    CHECK(g->n == 1 && g->resource[0] == OEC_RES_DATA);
    CHECK(strcmp(g->service_type[0], "Utility") == 0);
    CHECK(strcmp(g->app[0], PROVIDER) == 0 && strcmp(g->source[0], PROVIDER) == 0);
    CHECK(strcmp(g->filter[0], "oee") == 0 && g->writer[0] >= 10);
    CHECK(strstr(g->payload[0], "\"Count\":1") != NULL);

    OK(oec_set(c, OEC_RES_DATA, OEC_ST_UTILITY, PROVIDER, PROVIDER, "oee",
               "{\"Availability\":50.0,\"Count\":7}", 0, &code));
    CHECK(code == OEC_GOOD);
    g = get(c, OEC_RES_DATA, "oee");
    CHECK(g->n == 1 && strstr(g->payload[0], "\"Count\":7") != NULL);
    FAILS(oec_set(c, OEC_RES_DATA, OEC_ST_UTILITY, PROVIDER, PROVIDER, "oee",
                  "{\"Availability\":50.0,\"Count\":101}", 0, &code),
          OEC_ERR_STATUS);
    CHECK(code == OEC_BAD_OUT_OF_RANGE && oec_last_status_code() == OEC_BAD_OUT_OF_RANGE);
    CHECK(atomic_load(&data_sets) == 2 && atomic_load(&blocked_in_handler) == 2);
    FAILS(oec_set(c, OEC_RES_DATA, OEC_ST_ANY, PROVIDER, PROVIDER, "oee", "{}", 0, NULL),
          OEC_ERR_INVALID_ARGUMENT);

    /* Del/Data is no allowed combination (REQ-TOP-013): refused before sending */
    FAILS(oec_del(c, OEC_RES_DATA, OEC_ST_UTILITY, PROVIDER, PROVIDER, "oee", NULL, 0, &code),
          OEC_ERR_INVALID_ARGUMENT);
    /* the provider has no ReferenceDesignation: a Status event Bad */
    code = 7;
    FAILS(oec_del(c, OEC_RES_REFERENCE_DESIGNATION, OEC_ST_UTILITY, PROVIDER, PROVIDER, NULL,
                  NULL, 0, &code),
          OEC_ERR_STATUS);
    CHECK(code == OEC_BAD_NOT_FOUND && oec_last_status_code() == OEC_BAD_NOT_FOUND);
}

static void config_requests(oec_app *c) {
    struct got *g;
    oec_status_code code = 7;

    g = get(c, OEC_RES_CONFIG, "settings");
    CHECK(g->n == 1 && strstr(g->payload[0], "Hamburg") != NULL);
    OK(oec_set(c, OEC_RES_CONFIG, OEC_ST_UTILITY, PROVIDER, PROVIDER, "settings",
               "{\"common\":{\"city\":{\"Value\":\"Munich\"}}}", 0, &code));
    CHECK(code == OEC_GOOD);
    CHECK(strcmp(last_config, "{\"common\":{\"city\":\"Munich\"}}") == 0);
    FAILS(oec_set(c, OEC_RES_CONFIG, OEC_ST_UTILITY, PROVIDER, PROVIDER, "settings",
                  "{\"common\":{\"city\":{\"Value\":\"Nowhere\"}}}", 0, NULL),
          OEC_ERR_STATUS);
    CHECK(oec_last_status_code() == OEC_BAD_OUT_OF_RANGE && atomic_load(&config_sets) == 2);
    g = get(c, OEC_RES_CONFIG, "settings");
    CHECK(g->n == 1 && strstr(g->payload[0], "Munich") != NULL);
}

static void action_requests(oec_app *c) {
    uint16_t id = 0;
    uint32_t ttl = 0;

    OK(oec_new_data_set_writer_id(c, OEC_RES_DATA, OEC_ST_UTILITY, PROVIDER, NULL, 0, &id, &ttl));
    CHECK(id >= 10 && ttl == 60);
    FAILS(oec_new_data_set_writer_id(c, OEC_RES_DATA, OEC_ST_UTILITY, PROVIDER,
                                     "vendor.com/Pump/P-1/none", 0, &id, &ttl),
          OEC_ERR_STATUS);
    CHECK(id == 0 && ttl == 0 && oec_last_status_code() == OEC_BAD_NOT_FOUND);
    FAILS(oec_new_data_set_writer_id(c, OEC_RES_DATA, OEC_ST_UTILITY, PROVIDER, NULL, 0, NULL,
                                     &ttl),
          OEC_ERR_INVALID_ARGUMENT);
}

/* ---- lifecycle: state, wait, presence (amendment C-076) ---------------- */

#define SUBSCRIBER "example.com/Abi/ABI-1/presence-sub"
#define PEER "example.com/Abi/ABI-1/presence-peer"

static atomic_int mam_own, mam_peer;

static void count_mam(void *user_data, const oec_received *msg) {
    (void)user_data;
    if (msg->resource != OEC_RES_MAM)
        return;
    if (strcmp(msg->app, SUBSCRIBER) == 0)
        atomic_fetch_add(&mam_own, 1);
    if (strcmp(msg->app, PEER) == 0)
        atomic_fetch_add(&mam_peer, 1);
}

static void wait_state(oec_app *a, oec_connection_state want) {
    long start = now_ms();
    oec_connection_state st = -1;
    for (;;) {
        OK(oec_app_state(a, &st));
        if (st == want)
            return;
        CHECK(now_ms() - start < WAIT_MS);
        sleep_ms(5);
    }
}

static void wait_count(atomic_int *n, int want, const char *what) {
    long start = now_ms();
    while (atomic_load(n) < want) {
        if (now_ms() - start >= WAIT_MS) {
            fprintf(stderr, "timeout: %s\n", what);
            exit(1);
        }
        sleep_ms(5);
    }
}

/* A start error ends the SDK thread of oec_app_start; oec_app_wait
 * reports it (here: a CA file that holds no certificate). */
static void start_error(void) {
    char ca[1024];
    snprintf(ca, sizeof ca, "%s/no-ca.pem", work_arg);
    FILE *f = fopen(ca, "w");
    CHECK(f != NULL);
    fputs("no certificate\n", f);
    fclose(f);
    oec_product *p = product();
    oec_builder *b = NULL;
    oec_app *a = NULL;
    OK(oec_builder_new(OEC_LEVEL_1, OEC_ST_UTILITY, p, &b));
    oec_product_free(p);
    OK(oec_builder_broker(b, "localhost", (uint16_t)atoi(port_arg), ca));
    OK(oec_builder_credentials(b, "user", "pa:ss"));
    OK(oec_builder_serial_number(b, "start-error"));
    OK(oec_builder_build(b, &a));
    OK(oec_app_start(a));
    FAILS(oec_app_wait(a, OEC_WAIT_FOREVER), OEC_ERR_BOOTSTRAP);
    CHECK(strstr(oec_last_error(), "no-ca.pem") != NULL);
    /* the result stays */
    FAILS(oec_app_wait(a, 0), OEC_ERR_BOOTSTRAP);
    oec_connection_state st = -1;
    OK(oec_app_state(a, &st));
    CHECK(st == OEC_STATE_CLOSED);
    oec_app_free(a);
}

/* The spike finding of C-075: a MAM subscription receives the Birth of a
 * peer that connects later, not the own Birth (the SUBSCRIBE follows it,
 * REQ-LIFE-015), and the own MAM as answer of a Get. */
static void lifecycle(void) {
    oec_app *s = app(OEC_LEVEL_2, "presence-sub");
    oec_subscription *sub = NULL;
    oec_connection_state st = -1;

    FAILS(oec_app_state(NULL, &st), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_app_state(s, NULL), OEC_ERR_INVALID_ARGUMENT);
    OK(oec_app_state(s, &st));
    CHECK(st == OEC_STATE_CLOSED); /* before the run */
    FAILS(oec_app_wait(NULL, 0), OEC_ERR_INVALID_ARGUMENT);
    FAILS(oec_app_wait(s, 0), OEC_ERR_INVALID_STATE); /* not started */
    CHECK(strstr(oec_last_error(), "oec_app_start") != NULL);

    OK(oec_subscribe(s, OEC_RES_MAM, OEC_ST_ANY, NULL, NULL, NULL, count_mam, NULL, &sub));
    OK(oec_app_start(s));
    wait_state(s, OEC_STATE_ONLINE);
    FAILS(oec_app_wait(s, 0), OEC_ERR_TIMEOUT);
    FAILS(oec_app_wait(s, 20), OEC_ERR_TIMEOUT);

    /* the Get follows the MAM SUBSCRIBE: afterwards it is active */
    struct got *g = calloc(1, sizeof *g);
    CHECK(g != NULL);
    OK(oec_get(s, OEC_RES_MAM, OEC_ST_UTILITY, SUBSCRIBER, NULL, NULL, 0, collect, g));
    CHECK(g->n == 1);
    free(g);
    wait_count(&mam_own, 1, "own MAM answer received by the subscription");
    sleep_ms(100);
    CHECK(atomic_load(&mam_own) == 1); /* the answer only, not the Birth */

    oec_app *peer = app(OEC_LEVEL_1, "presence-peer");
    OK(oec_app_start(peer));
    wait_count(&mam_peer, 1, "Birth MAM of the peer");

    oec_app_shutdown(peer);
    OK(oec_app_wait(peer, OEC_WAIT_FOREVER));
    OK(oec_app_wait(peer, 0));
    OK(oec_app_state(peer, &st));
    CHECK(st == OEC_STATE_CLOSED);
    oec_app_free(peer);

    oec_app_shutdown(s);
    OK(oec_app_wait(s, WAIT_MS));
    oec_app_free(s);
    oec_subscription_free(sub);

    start_error();
}

/* ---- stress ------------------------------------------------------------ */

#define THREADS 6
#define ROUNDS 150

struct worker {
    struct provider *p;
    oec_app *c;
    int n;
    atomic_int *errors;
};

static void *work(void *arg) {
    struct worker *w = arg;
    char json[96];
    for (int i = 0; i < ROUNDS; i++) {
        snprintf(json, sizeof json, "{\"Availability\":%d.5,\"Count\":%d}", i % 100, w->n * i);
        if (oec_dataset_update(w->p->oee, json) != OEC_OK)
            atomic_fetch_add(w->errors, 1);
        char *value = NULL;
        if (oec_config_get(w->p->cfg, "common", "city", &value) != OEC_OK || value == NULL)
            atomic_fetch_add(w->errors, 1);
        oec_string_free(value);
        if (oec_app_syslog(w->p->app, OEC_SYSLOG_DEBUG, "stress") != OEC_OK)
            atomic_fetch_add(w->errors, 1);
        if (i % 25 == 0 && get(w->c, OEC_RES_DATA, "oee")->n != 1)
            atomic_fetch_add(w->errors, 1);
        /* bad input from every thread: the error stays on its thread */
        if (oec_dataset_update(w->p->oee, "{\"Count\":\"x\"}") != OEC_ERR_INVALID_ARGUMENT ||
            strstr(oec_last_error(), "Count") == NULL)
            atomic_fetch_add(w->errors, 1);
    }
    return NULL;
}

static void stress(struct provider *p, oec_app *c) {
    pthread_t threads[THREADS];
    struct worker workers[THREADS];
    atomic_int errors = 0;
    for (int i = 0; i < THREADS; i++) {
        workers[i] = (struct worker){p, c, i, &errors};
        CHECK(pthread_create(&threads[i], NULL, work, &workers[i]) == 0);
    }
    for (int i = 0; i < THREADS; i++)
        CHECK(pthread_join(threads[i], NULL) == 0);
    CHECK(atomic_load(&errors) == 0);
}

int main(int argc, char **argv) {
    if (argc != 4) {
        fprintf(stderr, "usage: abi <port> <ca.pem> <work dir>\n");
        return 2;
    }
    port_arg = argv[1];
    ca_arg = argv[2];
    work_arg = argv[3];

    argument_errors();
    state_errors();
    lifecycle();

    struct provider p = provider();
    oec_app *c = app(OEC_LEVEL_2, "consumer");
    oec_subscription *data = NULL, *once = NULL;
    consumer_app = c;
    OK(oec_subscribe(c, OEC_RES_DATA, OEC_ST_UTILITY, PROVIDER, PROVIDER, "oee", count_data, NULL,
                     &data));
    OK(oec_subscribe(c, OEC_RES_DATA, OEC_ST_ANY, NULL, NULL, NULL, free_myself, &once, &once));
    OK(oec_app_start(p.app));
    OK(oec_app_start(c));
    FAILS(oec_app_start(c), OEC_ERR_INVALID_STATE);
    wait_online(c);

    data_requests(c);
    config_requests(c);
    action_requests(c);
    wait_for(&published, "Data received by the subscription");
    wait_for(&freed_in_callback, "subscription freed in its callback");
    wait_for(&free_refused, "oec_app_free refused in a callback");
    CHECK(once == NULL);

    stress(&p, c);

    oec_app_shutdown(c);
    oec_app_free(c);
    oec_app_free(p.app);
    oec_subscription_free(data); /* after the app: allowed */
    printf("abi ok\n");
    return 0;
}
