/* Shape of the logger demo (D4, Messaging Level 1, Q-024) on the C ABI: a
 * Utility that subscribes to the Events and the Health of all participants
 * and prints one line per DataSetMessage until SIGINT/SIGTERM. OEC_ROOT
 * (optional) is the root of the container storage, "/" in a container
 * (REQ-CNT-004). */
#include <stdio.h>
#include <stdlib.h>

#include "oec.h"

static const char *resource_name(oec_resource r) {
    switch (r) {
    case OEC_RES_EVENT:
        return "Event";
    case OEC_RES_HEALTH:
        return "Health";
    default:
        return "?";
    }
}

/* Runs on the thread of oec_app_run, one call at a time. */
static void print_msg(void *user_data, const oec_received *msg) {
    FILE *out = user_data;
    fprintf(out, "%s %s %s %s %s\n", resource_name(msg->resource), msg->app,
            msg->source ? msg->source : "-", msg->filter ? msg->filter : "-", msg->payload_json);
    fflush(out);
}

static int fail(const char *what) {
    fprintf(stderr, "%s: %s\n", what, oec_last_error());
    return 1;
}

int main(void) {
    oec_product *product = NULL;
    oec_builder *builder = NULL;
    oec_app *app = NULL;
    oec_subscription *events = NULL, *health = NULL;
    const char *root = getenv("OEC_ROOT");

    if (oec_product_new("example.com", "Logger", "LOGGER-1", &product) != OEC_OK)
        return fail("product");
    if (oec_product_set(product, "SoftwareRevision", "1.0.0") != OEC_OK) {
        oec_product_free(product);
        return fail("product");
    }
    oec_status st = oec_builder_new(OEC_LEVEL_1, OEC_ST_UTILITY, product, &builder);
    oec_product_free(product);
    if (st != OEC_OK)
        return fail("builder");
    if (root && oec_builder_container_storage(builder, root) != OEC_OK) {
        oec_builder_free(builder);
        return fail("root");
    }
    if (oec_builder_build(builder, &app) != OEC_OK)
        return fail("build");

    /* Every ServiceType, AppId, Source and Filter (REQ-RES-SUBLIST-001). */
    if (oec_subscribe(app, OEC_RES_EVENT, OEC_ST_ANY, NULL, NULL, NULL, print_msg, stdout,
                      &events) != OEC_OK ||
        oec_subscribe(app, OEC_RES_HEALTH, OEC_ST_ANY, NULL, NULL, NULL, print_msg, stdout,
                      &health) != OEC_OK) {
        fail("subscribe");
        oec_subscription_free(events);
        oec_app_free(app);
        return 1;
    }

    printf("logging\n");
    fflush(stdout);
    st = oec_app_run(app);
    if (st != OEC_OK)
        fail("run");
    oec_subscription_free(events);
    oec_subscription_free(health);
    oec_app_free(app);
    return st == OEC_OK ? 0 : 1;
}
