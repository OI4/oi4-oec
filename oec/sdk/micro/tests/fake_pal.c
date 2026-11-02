#include "fake_pal.h"

#include <string.h>

static fake_event *next(fake_pal *fp, fake_kind kind) {
    if (fp->count >= FAKE_MAX_EVENTS)
        return NULL;
    fake_event *e = &fp->events[fp->count++];
    memset(e, 0, sizeof *e);
    e->kind = kind;
    return e;
}

static void copy(char *dst, size_t cap, const void *src, size_t n) {
    if (n >= cap)
        n = cap - 1;
    memcpy(dst, src, n);
    dst[n] = '\0';
}

static oecm_err pal_connect(void *ctx, const oecm_connect *o) {
    fake_pal *fp = ctx;
    fake_event *e = next(fp, FAKE_CONNECT);
    if (e) {
        copy(e->topic, sizeof e->topic, o->will_topic, strlen(o->will_topic));
        copy(e->payload, sizeof e->payload, o->will_payload, o->will_len);
        e->len = o->will_len;
        copy(e->client_id, sizeof e->client_id, o->client_id, strlen(o->client_id));
        e->keep_alive_s = o->keep_alive_s;
    }
    return fp->connect_result;
}

static oecm_err pal_publish(void *ctx, const char *topic, const uint8_t *payload, size_t len) {
    fake_pal *fp = ctx;
    if (fp->publish_result != OECM_OK)
        return fp->publish_result;
    fake_event *e = next(fp, FAKE_PUBLISH);
    if (e) {
        copy(e->topic, sizeof e->topic, topic, strlen(topic));
        copy(e->payload, sizeof e->payload, payload, len);
        e->len = len;
    }
    return OECM_OK;
}

static oecm_err pal_subscribe(void *ctx, const char *filter) {
    fake_event *e = next(ctx, FAKE_SUBSCRIBE);
    if (e)
        copy(e->topic, sizeof e->topic, filter, strlen(filter));
    return OECM_OK;
}

static oecm_err pal_disconnect(void *ctx) {
    (void)next(ctx, FAKE_DISCONNECT);
    return OECM_OK;
}

static fake_record *record(fake_pal *fp, const char *key, bool create) {
    for (size_t i = 0; i < FAKE_RECORDS; i++)
        if (fp->records[i].used && strcmp(fp->records[i].key, key) == 0)
            return &fp->records[i];
    if (!create)
        return NULL;
    for (size_t i = 0; i < FAKE_RECORDS; i++)
        if (!fp->records[i].used) {
            fake_record *r = &fp->records[i];
            r->used = true;
            copy(r->key, sizeof r->key, key, strlen(key));
            return r;
        }
    return NULL;
}

static oecm_err pal_load(void *ctx, const char *key, uint8_t *buf, size_t cap, size_t *len) {
    fake_pal *fp = ctx;
    if (fp->load_result != OECM_OK)
        return fp->load_result;
    const fake_record *r = record(fp, key, false);
    *len = 0;
    if (r == NULL)
        return OECM_OK;
    if (r->len > cap)
        return OECM_E_NOSPACE;
    memcpy(buf, r->data, r->len);
    *len = r->len;
    return OECM_OK;
}

static oecm_err pal_save(void *ctx, const char *key, const uint8_t *buf, size_t len) {
    fake_pal *fp = ctx;
    if (fp->save_result != OECM_OK)
        return fp->save_result;
    fake_record *r = record(fp, key, len != 0);
    if (len == 0) {
        if (r)
            memset(r, 0, sizeof *r);
    } else {
        if (r == NULL || len > FAKE_RECORD_MAX)
            return OECM_E_NOSPACE;
        memcpy(r->data, buf, len);
        r->len = len;
    }
    fp->saves++;
    return OECM_OK;
}

const fake_record *fake_record_of(const fake_pal *fp, const char *key) {
    for (size_t i = 0; i < FAKE_RECORDS; i++)
        if (fp->records[i].used && strcmp(fp->records[i].key, key) == 0)
            return &fp->records[i];
    return NULL;
}

static int64_t pal_unix(void *ctx) { return ((fake_pal *)ctx)->unix_ms; }
static uint32_t pal_mono(void *ctx) { return ((fake_pal *)ctx)->mono_ms; }

oecm_pal fake_pal_make(fake_pal *fp) {
    oecm_pal p = {
        .ctx = fp,
        .mqtt_connect = pal_connect,
        .mqtt_publish = pal_publish,
        .mqtt_subscribe = pal_subscribe,
        .mqtt_disconnect = pal_disconnect,
        .unix_time_ms = pal_unix,
        .monotonic_ms = pal_mono,
        .persist_load = pal_load,
        .persist_save = pal_save,
    };
    return p;
}

void fake_pal_reset(fake_pal *fp) {
    memset(fp, 0, sizeof *fp);
    fp->unix_ms = 1793000000000;
    fp->mono_ms = 5000;
}

const fake_event *fake_last(const fake_pal *fp, fake_kind kind) {
    for (size_t i = fp->count; i > 0; i--)
        if (fp->events[i - 1].kind == kind)
            return &fp->events[i - 1];
    return NULL;
}

size_t fake_count(const fake_pal *fp, fake_kind kind) {
    size_t n = 0;
    for (size_t i = 0; i < fp->count; i++)
        n += fp->events[i].kind == kind;
    return n;
}
