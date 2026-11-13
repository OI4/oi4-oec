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

/* --- Consumer subscriptions (C-114) --------------------------------------- */

static size_t level_end(const char *p, size_t n, size_t i) {
    while (i < n && p[i] != '/')
        i++;
    return i;
}

static bool is_wild(const char *p, size_t b, size_t e) {
    return e - b == 1 && (p[b] == '+' || p[b] == '#');
}

bool oecm_mqtt_match(const char *f, size_t fn, const char *t, size_t tn) {
    if (tn > 0 && t[0] == '$' && fn > 0 && (f[0] == '+' || f[0] == '#'))
        return false;
    size_t i = 0, j = 0;
    for (;;) {
        size_t ie = level_end(f, fn, i);
        if (ie - i == 1 && f[i] == '#')
            return true;
        size_t je = level_end(t, tn, j);
        bool plus = ie - i == 1 && f[i] == '+';
        if (!plus && (ie - i != je - j || memcmp(f + i, t + j, ie - i) != 0))
            return false;
        bool fend = ie == fn, tend = je == tn;
        if (fend && tend)
            return true;
        if (tend) /* "a/#" matches "a" */
            return fn - ie == 2 && f[ie + 1] == '#';
        if (fend)
            return false;
        i = ie + 1;
        j = je + 1;
    }
}

bool oecm_sub_filter_parse(const char *f, size_t n, oecm_sub_parts *out) {
    memset(out, 0, sizeof *out);
    if (f == NULL || n == 0 || n > OECM_SUB_FILTER_MAX || !oecm_utf8_valid(f, n))
        return false;
    for (size_t k = 0; k < n; k++)
        if (f[k] == '\0')
            return false;
    size_t level = 0, i = 0;
    bool wild_filter = false;
    for (;;) {
        size_t e = level_end(f, n, i);
        if (e == i)
            return false; /* no empty level */
        for (size_t k = i; k < e; k++)
            if ((f[k] == '+' || f[k] == '#') && e - i != 1)
                return false; /* a wildcard is a whole level */
        if (f[i] == '#' && e != n)
            return false; /* '#' is the last level */
        oecm_span v = {f + i, e - i};
        if (level == 0 && !oecm_span_eq(v, "Oi4v2"))
            return false;
        if (level == 3 && !oecm_span_eq(v, "Pub") && !oecm_span_eq(v, "+"))
            return false;
        if (level == 4) {
            if (is_wild(f, i, e))
                return false; /* a concrete Resource level */
            out->resource = v;
        }
        if (level == 5)
            out->source_all = oecm_span_eq(v, "#");
        if (level == 6)
            out->filter = (oecm_span){f + i, n - i};
        if (level >= 6 && is_wild(f, i, e))
            wild_filter = true;
        level++;
        if (e == n)
            break;
        i = e + 1;
    }
    if (level < 5)
        return false;
    if (!wild_filter)
        out->group_tail = out->filter;
    return true;
}

static oecm_err topic_il(const char *topic, bool source, char *out, size_t cap) {
    if (out == NULL || cap < OECM_IL_BUF)
        return OECM_E_NOSPACE;
    out[0] = '\0';
    oecm_topic t;
    if (topic == NULL || !oecm_topic_parse(topic, strlen(topic), &t))
        return OECM_E_ARG;
    oecm_span e = source ? t.source : t.app;
    size_t len;
    if (e.n == 0 || oecm_il_decode(e.p, e.n, out, cap, &len) != 0) {
        out[0] = '\0';
        return OECM_E_ARG;
    }
    return OECM_OK;
}

oecm_err oecm_topic_app(const char *topic, char *out, size_t cap) {
    return topic_il(topic, false, out, cap);
}

oecm_err oecm_topic_source(const char *topic, char *out, size_t cap) {
    return topic_il(topic, true, out, cap);
}
