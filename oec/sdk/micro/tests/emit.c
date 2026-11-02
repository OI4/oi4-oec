/* Writes the messages of a device lifecycle as JSONL for
 * `oec-check validate` (ADR-0030 §6): Birth, Health, Will, Close, for a
 * built Oi4Identifier and a given Identification Link with all optional MAM
 * elements. Usage: oecm-emit OUT.jsonl */
#include <stdio.h>
#include <string.h>

#include "fake_pal.h"
#include "oecm.h"

static fake_pal fp;
static uint8_t tx[OECM_TX_MIN];

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

static int run(FILE *f, const oecm_config *cfg) {
    fake_pal_reset(&fp);
    oecm_pal pal = fake_pal_make(&fp);
    oecm_device dev;
    if (oecm_init(&dev, cfg, &pal, tx, sizeof tx) != OECM_OK || oecm_start(&dev) != OECM_OK)
        return 1;
    oecm_on_connected(&dev);
    fp.unix_ms += 1500;
    fp.mono_ms += 1500;
    if (oecm_set_health(&dev, OECM_HEALTH_OFF_SPEC, -1) != OECM_OK || oecm_poll(&dev) != OECM_OK)
        return 1;
    fp.unix_ms += 1500;
    if (oecm_stop(&dev) != OECM_OK)
        return 1;
    for (size_t i = 0; i < fp.count; i++) {
        const fake_event *e = &fp.events[i];
        if (e->kind == FAKE_CONNECT)
            line(f, e, "will");
        else if (e->kind == FAKE_PUBLISH)
            line(f, e, i + 2 == fp.count ? "close" : i == 1 ? "birth" : NULL);
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
    if (fclose(f) != 0)
        rc = 1;
    if (rc)
        fprintf(stderr, "oecm-emit: lifecycle failed\n");
    return rc;
}
