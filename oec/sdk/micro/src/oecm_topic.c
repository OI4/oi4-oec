/*
 * Topic parsing (MG 6.3, REQ-TOP-*): the levels of a received topic as
 * spans; AppId and Source compared in their base64url form.
 */
#include <string.h>

#include "oecm_internal.h"

static oecm_span level(const char *t, size_t n, size_t *i) {
    oecm_span s = {t + *i, 0};
    while (*i < n && t[*i] != '/')
        (*i)++;
    s.n = (size_t)(t + *i - s.p);
    return s;
}

bool oecm_topic_parse(const char *t, size_t n, oecm_topic *out) {
    memset(out, 0, sizeof *out);
    if (t == NULL || n == 0)
        return false;
    for (size_t k = 0; k < n; k++)
        if (t[k] == '\0' || t[k] == '+' || t[k] == '#')
            return false;
    size_t i = 0;
    oecm_span *parts[6] = {NULL, &out->st, &out->app, &out->method, &out->resource, &out->source};
    oecm_span root = level(t, n, &i);
    if (!oecm_span_eq(root, "Oi4v2"))
        return false;
    for (size_t p = 1; p < 6; p++) {
        if (i >= n) {
            if (p < 5)
                return false;
            return true;
        }
        i++; /* '/' */
        *parts[p] = level(t, n, &i);
        if (parts[p]->n == 0)
            return false;
    }
    if (i < n) {
        i++;
        out->filter.p = t + i;
        out->filter.n = n - i;
        /* no empty level inside the filter */
        if (out->filter.n == 0 || t[i] == '/' || t[n - 1] == '/')
            return false;
        for (size_t k = i + 1; k < n; k++)
            if (t[k] == '/' && t[k - 1] == '/')
                return false;
    }
    return true;
}

bool oecm_span_eq(oecm_span a, const char *s) {
    size_t n = strlen(s);
    return a.n == n && (n == 0 || memcmp(a.p, s, n) == 0);
}

bool oecm_b64url_eq(oecm_span e, const char *s, size_t n) {
    static const char alphabet[] =
        "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
    if (e.n != oecm_b64url_len(n))
        return false;
    size_t k = 0;
    const uint8_t *u = (const uint8_t *)s;
    for (size_t i = 0; i < n; i += 3) {
        uint32_t v = (uint32_t)u[i] << 16;
        size_t rem = n - i;
        if (rem > 1)
            v |= (uint32_t)u[i + 1] << 8;
        if (rem > 2)
            v |= u[i + 2];
        size_t chars = rem >= 3 ? 4 : rem + 1;
        for (size_t c = 0; c < chars; c++)
            if (e.p[k++] != alphabet[v >> (18u - 6u * c) & 63u])
                return false;
    }
    return true;
}
