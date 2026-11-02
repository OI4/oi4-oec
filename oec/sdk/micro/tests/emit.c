/* Writes the messages of a device lifecycle as JSONL for
 * `oec-check validate` (ADR-0030 §6): Birth, Health, Will, Close, for a
 * built Oi4Identifier and a given Identification Link with all optional MAM
 * elements; at Level 2 (C-094) also the Profile, both lists, Config, Data
 * and Metadata, and the answers to Get and Set requests (publications with
 * ReplyTo, Status events Good and Bad); at Level 3 (C-095) the
 * ReferenceDesignation (Set, Get, Get with Query, invalid Set) and the
 * replies to NewDataSetWriterId Calls. Usage: oecm-emit OUT.jsonl */
#include <stdbool.h>
#include <stdio.h>
#include <string.h>

#include "fake_pal.h"
#include "oecm.h"

static fake_pal fp;
static uint8_t tx[8192];

static double value = 21.25, setpoint = 21.0;
static char label[33] = "Lab";
static char interval[16] = "5", mode[16] = "fast";

static const oecm_field fields[] = {
    {.name = "Value", .type = OECM_TYPE_DOUBLE, .value = &value, .decimals = 2, .unit = "\xc2\xb0" "C",
     .eu_low = "-40", .eu_high = "125"},
    {.name = "Setpoint", .description = "Target", .type = OECM_TYPE_DOUBLE, .value = &setpoint,
     .decimals = 1},
    {.name = "Label", .type = OECM_TYPE_STRING, .value = label, .cap = sizeof label},
};

static const oecm_data data[] = {{.tag = "Temperature",
                                  .description = "Temperature",
                                  .fields = fields,
                                  .field_count = 3,
                                  .interval_ms = 10000,
                                  .writable = true}};

static const char *const modes[] = {"fast", "slow"};

static const oecm_cfg_object objects[] = {
    {.key = "Interval", .name = "Interval", .description = "Seconds", .type = OECM_CFG_NUMBER,
     .value = interval, .cap = sizeof interval, .default_value = "5", .unit = "s",
     .mandatory = true, .min = "1", .max = "3600"},
    {.key = "Mode", .name = "Mode", .type = OECM_CFG_STRING, .value = mode, .cap = sizeof mode,
     .values = modes, .value_count = 2},
    {.key = "Label", .name = "Label", .type = OECM_CFG_STRING, .value = label, .cap = sizeof label,
     .length = 32, .sensitive = true},
};

static const oecm_cfg_group groups[] = {
    {.key = "General", .name = "General", .description = "Settings", .objects = objects,
     .count = 3}};

static const oecm_cfg_context contexts[] = {
    {.name = "Sensor", .title = "Sensor", .groups = groups, .count = 1}};

static void put_json(FILE *f, const char *s) {
    fputc('"', f);
    for (; *s; s++) {
        if (*s == '"' || *s == '\\')
            fputc('\\', f);
        fputc(*s, f);
    }
    fputc('"', f);
}

static void line(FILE *f, const fake_event *e, const char *role) {
    fputs("{\"topic\":", f);
    put_json(f, e->topic);
    fprintf(f, ",\"payload\":%s", e->payload);
    if (role)
        fprintf(f, ",\"role\":\"%s\"", role);
    fputs("}\n", f);
}

/* Requests of a peer at Level 2: Method/Resource, Filter, Messages. */
static const char *const requests[][3] = {
    {"/Get/Config/", "/Sensor", "[]"},
    {"/Get/Data/", "/Temperature", "[]"},
    {"/Get/Metadata/", "/Temperature", "[]"},
    {"/Get/PublicationList/", "", "[]"},
    {"/Get/SubscriptionList/", "", "[]"},
    {"/Get/Profile/", "", "[]"},
    {"/Get/MAM/", "", "[]"},
    {"/Get/Health/", "", "[]"},
    {"/Set/Config/", "/Sensor",
     "[{\"DataSetWriterId\":18,\"Payload\":{\"General\":{\"Name\":{\"Locale\":\"en-US\","
     "\"Text\":\"General\"},\"Interval\":{\"Type\":\"Number\",\"Value\":\"7\"}}}}]"},
    {"/Set/Config/", "/Sensor",
     "[{\"DataSetWriterId\":18,\"Payload\":{\"General\":{\"Name\":{\"Locale\":\"en-US\","
     "\"Text\":\"General\"},\"Interval\":{\"Type\":\"Number\",\"Value\":\"0\"}}}}]"},
    {"/Set/Data/", "/Temperature",
     "[{\"DataSetWriterId\":19,\"Payload\":{\"Value\":21.5,\"Setpoint\":22,"
     "\"Label\":\"Room\"}}]"},
};

/* Requests of a peer at Level 3 (DataSetWriterId of the
 * ReferenceDesignation: 20). */
static const char *const requests3[][3] = {
    {"/Set/ReferenceDesignation/", "",
     "[{\"DataSetWriterId\":20,\"Payload\":{\"Function\":{\"Value\":\"=TE1\","
     "\"Local\":\"Temperatur \\\"A\\\"\",\"Parent\":{\"Value\":\"=PLANT\","
     "\"IdentificationLink\":\"other.com/X/Y/1\"}},\"Location\":{\"Value\":\"+R1\"}}}]"},
    {"/Get/ReferenceDesignation/", "", "[]"},
    {"/Get/ReferenceDesignation", "",
     "[{\"DataSetWriterId\":9,\"Payload\":{\"Parent\":\"other.com/X/Y/1\","
     "\"Aspect\":\"Function\"}}]"},
    {"/Set/ReferenceDesignation/", "", "[{\"DataSetWriterId\":20,\"Payload\":[]}]"},
};

static char topic[512], msg[2048];

/* The PublisherId of the device, from its Will. */
static char publisher[256];

static void find_publisher(void) {
    const char *p = strstr(fp.events[0].payload, "\"PublisherId\":\"");
    publisher[0] = '\0';
    if (p) {
        p += strlen("\"PublisherId\":\"");
        const char *e = strchr(p, '"');
        if (e && (size_t)(e - p) < sizeof publisher)
            snprintf(publisher, sizeof publisher, "%.*s", (int)(e - p), p);
    }
}

/* A NewDataSetWriterId Call of a peer (MG 9.2.1); source: "" or "/<AppId>". */
static void call(oecm_device *dev, const char *prefix, size_t prefix_len, const char *source,
                 const char *items, bool correlation) {
    int n = snprintf(topic, sizeof topic, "%.*s/Call/NewDataSetWriterId%s", (int)prefix_len,
                     prefix, source);
    (void)n;
    snprintf(msg, sizeof msg,
             "{\"MessageId\":\"1793000000100-Registry/other.com/X/Y/1\","
             "\"MessageType\":\"ua-action-request\",\"PublisherId\":\"%s\","
             "\"Timestamp\":\"2026-11-11T10:00:00.000Z\","
             "\"ResponseAddress\":\"%.*s/Reply/NewDataSetWriterId%s\","
             "\"RequestorId\":\"Registry/other.com/X/Y/1\",\"TimeoutHint\":5000%s,"
             "\"Messages\":%s}",
             publisher, (int)prefix_len, prefix, source,
             correlation ? ",\"CorrelationData\":\"QUJDRA==\"" : "", items);
    oecm_on_message(dev, topic, (const uint8_t *)msg, strlen(msg));
    fp.unix_ms += 10;
}

/* Sends every request of the list to the device, as a peer would. */
static int ask(oecm_device *dev, const char *app, const oecm_config *cfg) {
    const fake_event *will = &fp.events[0];
    const char *pub = strstr(will->topic, "/Pub/");
    if (!pub)
        return 1;
    for (size_t i = 0; i < sizeof requests / sizeof requests[0]; i++) {
        int n = snprintf(topic, sizeof topic, "%.*s", (int)(pub - will->topic), will->topic);
        snprintf(topic + n, sizeof topic - (size_t)n, "%s%s%s", requests[i][0], app,
                 requests[i][1]);
        snprintf(msg, sizeof msg,
                 "{\"MessageId\":\"%llu-Registry/other.com/X/Y/1\","
                 "\"MessageType\":\"ua-data\",\"PublisherId\":\"Registry/other.com/X/Y/1\","
                 "\"Messages\":%s}",
                 1793000000000ull + i, requests[i][2]);
        oecm_on_message(dev, topic, (const uint8_t *)msg, strlen(msg));
        fp.unix_ms += 10;
    }
    if (cfg->level < OECM_LEVEL_3)
        return 0;
    for (size_t i = 0; i < sizeof requests3 / sizeof requests3[0]; i++) {
        bool with_source = requests3[i][0][strlen(requests3[i][0]) - 1] == '/';
        int n = snprintf(topic, sizeof topic, "%.*s", (int)(pub - will->topic), will->topic);
        snprintf(topic + n, sizeof topic - (size_t)n, "%s%s", requests3[i][0],
                 with_source ? app : "");
        snprintf(msg, sizeof msg,
                 "{\"MessageId\":\"%llu-Registry/other.com/X/Y/1\","
                 "\"MessageType\":\"ua-data\",\"PublisherId\":\"Registry/other.com/X/Y/1\","
                 "\"Messages\":%s}",
                 1793000000050ull + i, requests3[i][2]);
        oecm_on_message(dev, topic, (const uint8_t *)msg, strlen(msg));
        fp.unix_ms += 10;
    }
    if (oecm_poll(dev) != OECM_OK)
        return 1;
    static char source[256];
    snprintf(source, sizeof source, "/%s", app);
    find_publisher();
    size_t prefix_len = (size_t)(pub - will->topic);
    call(dev, will->topic, prefix_len, "",
         "[{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":1,\"ActionState\":1,"
         "\"Payload\":{\"Resource\":\"Data\"}},{\"DataSetWriterId\":8,\"ActionTargetId\":1,"
         "\"RequestId\":2,\"ActionState\":1,\"Payload\":{\"Resource\":\"Nope\"}}]",
         true);
    call(dev, will->topic, prefix_len, source,
         "[{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":3,\"ActionState\":1,"
         "\"Payload\":{\"Resource\":\"Config\"}}]",
         false);
    call(dev, will->topic, prefix_len, "/b3RoZXIuY29tL1gvWS8x",
         "[{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":4,\"ActionState\":1,"
         "\"Payload\":{\"Resource\":\"Data\"}}]",
         false);
    return 0;
}

static int run(FILE *f, const oecm_config *cfg) {
    fake_pal_reset(&fp);
    oecm_pal pal = fake_pal_make(&fp);
    oecm_device dev;
    if (oecm_init(&dev, cfg, &pal, tx, sizeof tx) != OECM_OK || oecm_start(&dev) != OECM_OK)
        return 1;
    oecm_on_connected(&dev);
    if (cfg->level >= OECM_LEVEL_2) {
        const char *app = strstr(fp.events[0].topic, "/Pub/Health/");
        if (!app || ask(&dev, app + strlen("/Pub/Health/"), cfg))
            return 1;
    }
    fp.unix_ms += 1500;
    fp.mono_ms += 1500;
    if (oecm_set_health(&dev, OECM_HEALTH_OFF_SPEC, -1) != OECM_OK || oecm_poll(&dev) != OECM_OK)
        return 1;
    fp.unix_ms += 1500;
    if (oecm_stop(&dev) != OECM_OK)
        return 1;
    bool birth = false;
    for (size_t i = 0; i < fp.count; i++) {
        const fake_event *e = &fp.events[i];
        if (e->kind == FAKE_CONNECT) {
            line(f, e, "will");
        } else if (e->kind == FAKE_PUBLISH) {
            bool first = !birth && strstr(e->topic, "/Pub/MAM/") != NULL;
            birth |= first;
            line(f, e, i + 2 == fp.count ? "close" : first ? "birth" : NULL);
        }
    }
    return 0;
}

int main(int argc, char **argv) {
    if (argc != 2) {
        fprintf(stderr, "usage: oecm-emit OUT.jsonl\n");
        return 2;
    }
    FILE *f = fopen(argv[1], "w");
    if (!f)
        return 2;
    oecm_config cfg = {
        .level = OECM_LEVEL_1,
        .service_type = OECM_ST_OT_CONNECTOR,
        .mam =
            {
                .manufacturer = "Provider GmbH",
                .manufacturer_uri = "provider.com",
                .model = "Sensor M\xc3\xbc 1",
                .product_code = "S-1/2",
                .software_revision = "1.0.0",
                .device_revision = "1",
                .device_manual = "https://provider.com/manual",
                .device_class = "Sensor",
                .serial_number = "4711#A",
                .revision_counter = -1,
            },
    };
    int rc = run(f, &cfg);
    cfg.identification_link = "https://id.provider.com/S/4711";
    cfg.service_type = OECM_ST_UTILITY;
    cfg.mam.hardware_revision = "B";
    cfg.mam.description = "Temperature \"T\"\n";
    cfg.mam.global_asset_id = "https://provider.com/aas/4711";
    cfg.mam.revision_counter = 7;
    rc |= run(f, &cfg);
    cfg.level = OECM_LEVEL_2;
    cfg.service_type = OECM_ST_OT_CONNECTOR;
    cfg.identification_link = NULL;
    cfg.data = data;
    cfg.data_count = 1;
    cfg.config = contexts;
    cfg.config_count = 1;
    rc |= run(f, &cfg);
    static char refdes[1024];
    cfg.level = OECM_LEVEL_3;
    cfg.refdes_buf = refdes;
    cfg.refdes_cap = sizeof refdes;
    rc |= run(f, &cfg);
    if (fclose(f) != 0)
        rc = 1;
    if (rc)
        fprintf(stderr, "oecm-emit: lifecycle failed\n");
    return rc;
}
