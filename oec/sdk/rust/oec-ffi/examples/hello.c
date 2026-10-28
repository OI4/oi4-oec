/* Hello World in C via the C ABI of the Rust SDK (ADR-0021): a Level 1
 * Utility that publishes its MAM and Health until SIGINT/SIGTERM.
 * OEC_ROOT (optional) is the root of the container storage, "/" in a
 * container (REQ-CNT-004). */
#include <stdio.h>
#include <stdlib.h>

#include "oec.h"

static int fail(const char *what) {
    fprintf(stderr, "%s: %s\n", what, oec_last_error());
    return 1;
}

int main(void) {
    oec_product *product = NULL;
    oec_builder *builder = NULL;
    oec_app *app = NULL;
    const char *root = getenv("OEC_ROOT");

    if (oec_product_new("example.com", "Hello OEC C", "HELLO-C-1", &product) != OEC_OK)
        return fail("product");
    if (oec_product_set(product, "Manufacturer", "Example Inc.") != OEC_OK ||
        oec_product_set(product, "SoftwareRevision", "1.0.0") != OEC_OK ||
        oec_product_set(product, "DeviceManual", "https://example.com/hello/manual.pdf") != OEC_OK) {
        oec_product_free(product);
        return fail("product");
    }

    oec_status st = oec_builder_new(OEC_LEVEL_1, OEC_ST_UTILITY, product, &builder);
    oec_product_free(product); /* the builder copied it */
    if (st != OEC_OK)
        return fail("builder");
    if (root && oec_builder_container_storage(builder, root) != OEC_OK) {
        oec_builder_free(builder);
        return fail("root");
    }
    if (oec_builder_build(builder, &app) != OEC_OK) /* consumes the builder */
        return fail("build");

    if (oec_asset_set_health(oec_app_this(app), OEC_HEALTH_NORMAL, 100) != OEC_OK) {
        oec_app_free(app);
        return fail("health");
    }
    printf("running %s\n", oec_version());
    fflush(stdout);
    st = oec_app_run(app); /* blocks until SIGINT/SIGTERM or oec_app_shutdown */
    if (st != OEC_OK)
        fail("run");
    oec_app_free(app);
    return st == OEC_OK ? 0 : 1;
}
