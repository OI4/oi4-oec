/* A recording PAL for the host tests and the emitter (ADR-0030 §5). */
#ifndef OECM_FAKE_PAL_H
#define OECM_FAKE_PAL_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include "oecm.h"

#define FAKE_MAX_EVENTS 64
#define FAKE_MAX_TEXT 4096

typedef enum fake_kind { FAKE_CONNECT, FAKE_PUBLISH, FAKE_DISCONNECT } fake_kind;

typedef struct fake_event {
    fake_kind kind;
    char topic[1024];          /* publish topic or Will topic */
    char payload[FAKE_MAX_TEXT]; /* NUL-terminated copy */
    size_t len;
    char client_id[64];
    uint16_t keep_alive_s;
} fake_event;

typedef struct fake_pal {
    int64_t unix_ms;
    uint32_t mono_ms;
    oecm_err connect_result;
    oecm_err publish_result;
    size_t count;
    fake_event events[FAKE_MAX_EVENTS];
} fake_pal;

/* A PAL whose ctx is fp. */
oecm_pal fake_pal_make(fake_pal *fp);
void fake_pal_reset(fake_pal *fp);
/* The last event of a kind, NULL if none. */
const fake_event *fake_last(const fake_pal *fp, fake_kind kind);

#endif
