/* The PAL of the host runner: trampolines from the core's function
 * pointers to the Go functions exported in pal.go. ctx is unused – the
 * runner drives exactly one device. */
#include "oecm.h"
#include "_cgo_export.h"

static oecm_err t_connect(void *ctx, const oecm_connect *o) {
    (void)ctx;
    return oecmHostConnect((oecm_connect *)o);
}

static oecm_err t_publish(void *ctx, const char *topic, const uint8_t *payload, size_t len) {
    (void)ctx;
    return oecmHostPublish((char *)topic, (uint8_t *)payload, len);
}

static oecm_err t_subscribe(void *ctx, const char *filter) {
    (void)ctx;
    return oecmHostSubscribe((char *)filter);
}

static oecm_err t_disconnect(void *ctx) {
    (void)ctx;
    return oecmHostDisconnect();
}

static int64_t t_unix_ms(void *ctx) {
    (void)ctx;
    return oecmHostUnixMs();
}

static uint32_t t_mono_ms(void *ctx) {
    (void)ctx;
    return oecmHostMonoMs();
}

const oecm_pal oecm_host_pal = {
    .mqtt_connect = t_connect,
    .mqtt_publish = t_publish,
    .mqtt_subscribe = t_subscribe,
    .mqtt_disconnect = t_disconnect,
    .unix_time_ms = t_unix_ms,
    .monotonic_ms = t_mono_ms,
};
