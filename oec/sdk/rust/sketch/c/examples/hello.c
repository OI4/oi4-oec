/* Hello World in C via the C ABI of the Rust SDK (ADR-0021). */
#include <stdio.h>

#include "oec.h"

static int fail(const char *what) {
    fprintf(stderr, "%s: %s\n", what, oec_last_error());
    return 1;
}

int main(void) {
    oec_product *product = NULL;
    oec_builder *builder = NULL;
    oec_app *app = NULL;

    if (oec_product_new("example.com", "Hello OEC C", "HELLO-C-1", &product) != OEC_OK)
        return fail("product");
    oec_product_set(product, "Manufacturer", "Example Inc.");
    oec_product_set(product, "SoftwareRevision", "1.0.0");
    oec_product_set(product, "DeviceManual", "https://example.com/hello/manual.pdf");

    if (oec_builder_new(OEC_LEVEL_1, OEC_ST_UTILITY, product, &builder) != OEC_OK)
        return fail("builder");
    oec_product_free(product); /* the builder copied it */
    if (oec_builder_build(builder, &app) != OEC_OK) /* consumes the builder */
        return fail("build");

    oec_asset_set_health(oec_app_this(app), OEC_HEALTH_NORMAL, 100);
    oec_status st = oec_app_run(app); /* blocks until SIGTERM / shutdown */
    oec_app_free(app);
    return st == OEC_OK ? 0 : fail("run");
}
