/* Shape of the logger demo (D4, Messaging L1) on the C ABI: subscribes to
 * Events and Health of all participants and prints them; a writable Data
 * DataSet shows the Set callback. */
#include <stdio.h>
#include <string.h>

#include "oec.h"

static void print_msg(void *user_data, const oec_received *msg) {
    FILE *out = user_data;
    fprintf(out, "[%d] %s %s %s: %s\n", (int)msg->resource, msg->app,
            msg->source ? msg->source : "-", msg->filter ? msg->filter : "-", msg->payload_json);
}

/* Accepts only {"Enabled": true|false}; the SDK did the type check. */
static oec_status_code on_set(void *user_data, const char *values_json) {
    (void)user_data;
    return strstr(values_json, "\"Enabled\"") ? OEC_GOOD : OEC_BAD_OUT_OF_RANGE;
}

int main(void) {
    oec_product *product = NULL;
    oec_builder *builder = NULL;
    oec_app *app = NULL;
    oec_dataset *switch_ds = NULL;
    oec_subscription *events = NULL, *health = NULL;

    if (oec_product_new("example.com", "Logger", "LOGGER-1", &product) != OEC_OK) return 1;
    oec_product_set(product, "SoftwareRevision", "1.0.0");
    if (oec_builder_new(OEC_LEVEL_1, OEC_ST_UTILITY, product, &builder) != OEC_OK) return 1;
    oec_product_free(product);
    if (oec_builder_build(builder, &app) != OEC_OK) {
        fprintf(stderr, "build: %s\n", oec_last_error());
        return 1;
    }

    oec_subscribe(app, OEC_RES_EVENT, NULL, NULL, NULL, print_msg, stdout, &events);
    oec_subscribe(app, OEC_RES_HEALTH, NULL, NULL, NULL, print_msg, stdout, &health);

    oec_asset_add_data(oec_app_this(app), "logging", "[{\"Name\":\"Enabled\",\"Type\":\"Boolean\"}]",
                       OEC_MODE_ON_REQUEST | OEC_MODE_FILTER, 0, OEC_WRITABLE_NOTHING, &switch_ds);
    oec_dataset_on_set(switch_ds, on_set, NULL);
    oec_dataset_update(switch_ds, "{\"Enabled\":true}");

    oec_status st = oec_app_run(app);
    oec_subscription_free(events);
    oec_subscription_free(health);
    oec_app_free(app);
    return st == OEC_OK ? 0 : 1;
}
