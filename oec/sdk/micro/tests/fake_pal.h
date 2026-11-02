/* A recording PAL for the host tests and the emitter (ADR-0030 §5). */
#ifndef OECM_FAKE_PAL_H
#define OECM_FAKE_PAL_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include "oecm.h"

#define FAKE_MAX_EVENTS 128
#define FAKE_MAX_TEXT 16384

typedef enum fake_kind { FAKE_CONNECT, FAKE_PUBLISH, FAKE_SUBSCRIBE, FAKE_DISCONNECT } fake_kind;

typedef struct fake_event {
    fake_kind kind;
    char topic[1024];          /* publish topic, Will topic or topic filter */
    char payload[FAKE_MAX_TEXT]; /* NUL-terminated copy */
    size_t len;
    char client_id[64];
    uint16_t keep_alive_s;
} fake_event;

#define FAKE_RECORDS 4
#define FAKE_RECORD_MAX 4096

/* A persisted record (persist_load / persist_save). */
typedef struct fake_record {
    char key[32];
    uint8_t data[FAKE_RECORD_MAX];
    size_t len;
    bool used;
} fake_record;

typedef struct fake_pal {
    int64_t unix_ms;
    uint32_t mono_ms;
    oecm_err connect_result;
    oecm_err publish_result;
    oecm_err load_result; /* not OECM_OK: persist_load fails */
    oecm_err save_result; /* not OECM_OK: persist_save fails */
    size_t saves;         /* successful persist_save calls */
    fake_record records[FAKE_RECORDS];
    size_t count;
    fake_event events[FAKE_MAX_EVENTS];
} fake_pal;

/* A PAL whose ctx is fp. */
oecm_pal fake_pal_make(fake_pal *fp);
void fake_pal_reset(fake_pal *fp);
/* The last event of a kind, NULL if none. */
const fake_event *fake_last(const fake_pal *fp, fake_kind kind);
/* The number of events of a kind. */
size_t fake_count(const fake_pal *fp, fake_kind kind);
/* The persisted record of a key, NULL if none. */
const fake_record *fake_record_of(const fake_pal *fp, const char *key);

#endif
