/*
 * Bounded output buffer and the JSON text primitives of the core. Numbers
 * are formatted here, not by the C library (ADR-0030 §2).
 */
#include <string.h>

#include "oecm_internal.h"

void oecm_buf_init(oecm_buf *b, uint8_t *p, size_t cap) {
    b->p = p;
    b->cap = cap;
    b->len = 0;
    b->full = false;
}

void oecm_buf_count(oecm_buf *b) { oecm_buf_init(b, NULL, SIZE_MAX); }

void oecm_put(oecm_buf *b, const void *s, size_t n) {
    if (b->full || n > b->cap - b->len) {
        b->full = true;
        return;
    }
    if (n > 0 && b->p != NULL)
        memcpy(b->p + b->len, s, n);
    b->len += n;
}

void oecm_puts(oecm_buf *b, const char *s) { oecm_put(b, s, strlen(s)); }

void oecm_putc(oecm_buf *b, char c) { oecm_put(b, &c, 1); }

void oecm_put_u64(oecm_buf *b, uint64_t v) {
    char d[20];
    size_t i = sizeof d;
    do {
        d[--i] = (char)('0' + v % 10u);
        v /= 10u;
    } while (v != 0);
    oecm_put(b, d + i, sizeof d - i);
}

void oecm_put_i64(oecm_buf *b, int64_t v) {
    if (v < 0) {
        oecm_putc(b, '-');
        oecm_put_u64(b, (uint64_t)(-(v + 1)) + 1u);
    } else {
        oecm_put_u64(b, (uint64_t)v);
    }
}

void oecm_put_json_chars(oecm_buf *b, const char *s, size_t n) {
    static const char hex[] = "0123456789abcdef";
    size_t run = 0; /* start of the pending unescaped run */
    for (size_t i = 0; i < n; i++) {
        unsigned char c = (unsigned char)s[i];
        if (c >= 0x20 && c != '"' && c != '\\')
            continue;
        oecm_put(b, s + run, i - run);
        run = i + 1;
        switch (c) {
        case '"':
            oecm_put(b, "\\\"", 2);
            break;
        case '\\':
            oecm_put(b, "\\\\", 2);
            break;
        case '\n':
            oecm_put(b, "\\n", 2);
            break;
        case '\r':
            oecm_put(b, "\\r", 2);
            break;
        case '\t':
            oecm_put(b, "\\t", 2);
            break;
        default: {
            char u[6] = {'\\', 'u', '0', '0', hex[c >> 4], hex[c & 15u]};
            oecm_put(b, u, sizeof u);
        }
        }
    }
    oecm_put(b, s + run, n - run);
}

void oecm_put_json_str(oecm_buf *b, const char *s, size_t n) {
    oecm_putc(b, '"');
    oecm_put_json_chars(b, s, n);
    oecm_putc(b, '"');
}
