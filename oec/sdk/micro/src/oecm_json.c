/*
 * JSON reader of the core (RFC 8259): a non-recursive validator and span
 * helpers over the validated text. No copies, no heap; the nesting depth is
 * bounded by OECM_JSON_DEPTH (ADR-0030 §4, C-094).
 */
#include <string.h>

#include "oecm_internal.h"

static size_t ws(const char *p, size_t i, size_t n) {
    while (i < n && (p[i] == ' ' || p[i] == '\t' || p[i] == '\n' || p[i] == '\r'))
        i++;
    return i;
}

static int hexval(char c) {
    if (c >= '0' && c <= '9')
        return c - '0';
    if (c >= 'a' && c <= 'f')
        return c - 'a' + 10;
    if (c >= 'A' && c <= 'F')
        return c - 'A' + 10;
    return -1;
}

static bool hex4(const char *p, size_t i, size_t n, unsigned *v) {
    if (n < 4 || i > n - 4)
        return false;
    unsigned x = 0;
    for (size_t k = 0; k < 4; k++) {
        int h = hexval(p[i + k]);
        if (h < 0)
            return false;
        x = x << 4 | (unsigned)h;
    }
    *v = x;
    return true;
}

/* The string token at p[i] == '"'; the index after it, 0 if invalid. */
static size_t scan_string(const char *p, size_t i, size_t n) {
    i++;
    while (i < n) {
        unsigned char c = (unsigned char)p[i];
        if (c == '"')
            return i + 1;
        if (c < 0x20)
            return 0;
        if (c != '\\') {
            i++;
            continue;
        }
        if (i + 1 >= n)
            return 0;
        char e = p[i + 1];
        if (e == 'u') {
            unsigned u;
            if (!hex4(p, i + 2, n, &u))
                return 0;
            i += 6;
            if (u >= 0xd800u && u <= 0xdbffu) {
                unsigned lo;
                if (i + 1 >= n || p[i] != '\\' || p[i + 1] != 'u' || !hex4(p, i + 2, n, &lo) ||
                    lo < 0xdc00u || lo > 0xdfffu)
                    return 0;
                i += 6;
            } else if (u >= 0xdc00u && u <= 0xdfffu) {
                return 0;
            }
            continue;
        }
        if (e != '"' && e != '\\' && e != '/' && e != 'b' && e != 'f' && e != 'n' && e != 'r' &&
            e != 't')
            return 0;
        i += 2;
    }
    return 0;
}

static size_t scan_literal(const char *p, size_t i, size_t n, const char *lit) {
    size_t k = strlen(lit);
    if (n - i < k || memcmp(p + i, lit, k) != 0)
        return 0;
    return i + k;
}

static size_t scan_number(const char *p, size_t i, size_t n) {
    size_t e = i;
    while (e < n && (p[e] == '-' || p[e] == '+' || p[e] == '.' || p[e] == 'e' || p[e] == 'E' ||
                     (p[e] >= '0' && p[e] <= '9')))
        e++;
    return oecm_is_json_number(p + i, e - i) ? e : 0;
}

static size_t scan_scalar(const char *p, size_t i, size_t n) {
    switch (p[i]) {
    case '"':
        return scan_string(p, i, n);
    case 't':
        return scan_literal(p, i, n, "true");
    case 'f':
        return scan_literal(p, i, n, "false");
    case 'n':
        return scan_literal(p, i, n, "null");
    default:
        return scan_number(p, i, n);
    }
}

/* A member key and its colon at p[i]; the index of the value, 0 if invalid. */
static size_t scan_key(const char *p, size_t i, size_t n) {
    if (i >= n || p[i] != '"')
        return 0;
    i = scan_string(p, i, n);
    if (i == 0)
        return 0;
    i = ws(p, i, n);
    if (i >= n || p[i] != ':')
        return 0;
    return ws(p, i + 1, n);
}

bool oecm_json_parse(const char *p, size_t n, oecm_span *root) {
    if (p == NULL || !oecm_utf8_valid(p, n))
        return false;
    uint32_t objects = 0; /* bit per level: 1 = object */
    unsigned depth = 0;
    size_t i = ws(p, 0, n);
    size_t start = i;
    for (;;) {
        /* a value at i */
        if (i >= n)
            return false;
        bool closed = false;
        if (p[i] == '{' || p[i] == '[') {
            if (depth == OECM_JSON_DEPTH)
                return false;
            bool obj = p[i] == '{';
            objects = objects << 1 | (obj ? 1u : 0u);
            depth++;
            i = ws(p, i + 1, n);
            if (i >= n)
                return false;
            if (p[i] != (obj ? '}' : ']')) {
                if (obj && (i = scan_key(p, i, n)) == 0)
                    return false;
                continue;
            }
            i++;
            depth--;
            objects >>= 1;
            closed = true;
        }
        if (!closed && (i = scan_scalar(p, i, n)) == 0)
            return false;
        /* after a value: close containers or find the next value */
        for (;;) {
            if (depth == 0) {
                size_t end = i;
                if (ws(p, i, n) != n)
                    return false;
                root->p = p + start;
                root->n = end - start;
                return true;
            }
            i = ws(p, i, n);
            if (i >= n)
                return false;
            bool obj = (objects & 1u) != 0;
            if (p[i] == ',') {
                i = ws(p, i + 1, n);
                if (obj && (i = scan_key(p, i, n)) == 0)
                    return false;
                break;
            }
            if (p[i] != (obj ? '}' : ']'))
                return false;
            i++;
            depth--;
            objects >>= 1;
        }
    }
}

oecm_jkind oecm_json_kind(oecm_span v) {
    if (v.p == NULL || v.n == 0)
        return OECM_J_INVALID;
    switch (v.p[0]) {
    case 'n':
        return OECM_J_NULL;
    case 't':
    case 'f':
        return OECM_J_BOOL;
    case '"':
        return OECM_J_STRING;
    case '[':
        return OECM_J_ARRAY;
    case '{':
        return OECM_J_OBJECT;
    default:
        return OECM_J_NUMBER;
    }
}

/* End of the (validated) value starting at p < end. */
static const char *skip_value(const char *p, const char *end) {
    if (*p == '"') {
        for (p++; p < end && *p != '"'; p++)
            if (*p == '\\')
                p++;
        return p < end ? p + 1 : end;
    }
    if (*p == '{' || *p == '[') {
        unsigned depth = 0;
        for (; p < end; p++) {
            if (*p == '"') {
                p = skip_value(p, end) - 1;
            } else if (*p == '{' || *p == '[') {
                depth++;
            } else if (*p == '}' || *p == ']') {
                if (--depth == 0)
                    return p + 1;
            }
        }
        return end;
    }
    while (p < end && *p != ',' && *p != '}' && *p != ']' && *p != ' ' && *p != '\t' &&
           *p != '\n' && *p != '\r')
        p++;
    return p;
}

static const char *skip_ws(const char *p, const char *end) {
    while (p < end && (*p == ' ' || *p == '\t' || *p == '\n' || *p == '\r'))
        p++;
    return p;
}

void oecm_json_iter(oecm_span container, oecm_jit *it) {
    oecm_jkind k = oecm_json_kind(container);
    if (k != OECM_J_ARRAY && k != OECM_J_OBJECT) {
        it->p = it->end = container.p;
        return;
    }
    it->p = container.p + 1;
    it->end = container.p + container.n - 1; /* the closing bracket */
}

bool oecm_json_next(oecm_jit *it, oecm_span *key, oecm_span *value) {
    const char *p = skip_ws(it->p, it->end);
    if (p < it->end && *p == ',')
        p = skip_ws(p + 1, it->end);
    if (p >= it->end)
        return false;
    if (*p == '"') {
        const char *e = skip_value(p, it->end);
        const char *c = skip_ws(e, it->end);
        if (c < it->end && *c == ':') {
            if (key) {
                key->p = p;
                key->n = (size_t)(e - p);
            }
            p = skip_ws(c + 1, it->end);
        } else if (key) {
            key->p = NULL;
            key->n = 0;
        }
    } else if (key) {
        key->p = NULL;
        key->n = 0;
    }
    const char *e = skip_value(p, it->end);
    value->p = p;
    value->n = (size_t)(e - p);
    it->p = e;
    return true;
}

bool oecm_json_get(oecm_span obj, const char *key, oecm_span *value) {
    if (oecm_json_kind(obj) != OECM_J_OBJECT)
        return false;
    oecm_jit it;
    oecm_span k, v;
    bool found = false;
    size_t kn = strlen(key);
    oecm_json_iter(obj, &it);
    while (oecm_json_next(&it, &k, &v)) {
        if (oecm_json_str_eq(k, key, kn)) {
            *value = v;
            found = true;
        }
    }
    return found;
}

/* Decodes the next unit of a string body at *pp into out (1 … 4 octets). */
static size_t unit(const char **pp, uint8_t out[4]) {
    const char *p = *pp;
    if (*p != '\\') {
        out[0] = (uint8_t)*p;
        *pp = p + 1;
        return 1;
    }
    char e = p[1];
    *pp = p + 2;
    switch (e) {
    case 'b':
        out[0] = '\b';
        return 1;
    case 'f':
        out[0] = '\f';
        return 1;
    case 'n':
        out[0] = '\n';
        return 1;
    case 'r':
        out[0] = '\r';
        return 1;
    case 't':
        out[0] = '\t';
        return 1;
    case 'u':
        break;
    default:
        out[0] = (uint8_t)e;
        return 1;
    }
    uint32_t cp = 0;
    for (int k = 2; k < 6; k++)
        cp = cp << 4 | (uint32_t)hexval(p[k]);
    *pp = p + 6;
    if (cp >= 0xd800u && cp <= 0xdbffu) {
        uint32_t lo = 0;
        for (int k = 8; k < 12; k++)
            lo = lo << 4 | (uint32_t)hexval(p[k]);
        cp = 0x10000u + ((cp - 0xd800u) << 10) + (lo - 0xdc00u);
        *pp = p + 12;
    }
    if (cp < 0x80u) {
        out[0] = (uint8_t)cp;
        return 1;
    }
    if (cp < 0x800u) {
        out[0] = (uint8_t)(0xc0u | cp >> 6);
        out[1] = (uint8_t)(0x80u | (cp & 0x3fu));
        return 2;
    }
    if (cp < 0x10000u) {
        out[0] = (uint8_t)(0xe0u | cp >> 12);
        out[1] = (uint8_t)(0x80u | (cp >> 6 & 0x3fu));
        out[2] = (uint8_t)(0x80u | (cp & 0x3fu));
        return 3;
    }
    out[0] = (uint8_t)(0xf0u | cp >> 18);
    out[1] = (uint8_t)(0x80u | (cp >> 12 & 0x3fu));
    out[2] = (uint8_t)(0x80u | (cp >> 6 & 0x3fu));
    out[3] = (uint8_t)(0x80u | (cp & 0x3fu));
    return 4;
}

bool oecm_json_str_eq(oecm_span v, const char *s, size_t n) {
    if (oecm_json_kind(v) != OECM_J_STRING || v.n < 2)
        return false;
    const char *p = v.p + 1, *end = v.p + v.n - 1;
    size_t k = 0;
    while (p < end) {
        uint8_t u[4];
        size_t m = unit(&p, u);
        if (m > n - k || memcmp(u, s + k, m) != 0)
            return false;
        k += m;
    }
    return k == n;
}

bool oecm_json_streq(oecm_span v, const char *s) { return oecm_json_str_eq(v, s, strlen(s)); }

bool oecm_json_str(oecm_span v, char *out, size_t cap, size_t *len) {
    if (oecm_json_kind(v) != OECM_J_STRING || v.n < 2 || cap == 0)
        return false;
    const char *p = v.p + 1, *end = v.p + v.n - 1;
    size_t k = 0;
    while (p < end) {
        uint8_t u[4];
        size_t m = unit(&p, u);
        if (m >= cap - k)
            return false;
        memcpy(out + k, u, m);
        k += m;
    }
    out[k] = '\0';
    if (len)
        *len = k;
    return true;
}

size_t oecm_json_str_bytes(oecm_span v, bool *nul) {
    *nul = false;
    if (oecm_json_kind(v) != OECM_J_STRING || v.n < 2)
        return 0;
    const char *p = v.p + 1, *end = v.p + v.n - 1;
    size_t k = 0;
    while (p < end) {
        uint8_t u[4];
        size_t m = unit(&p, u);
        if (u[0] == 0u)
            *nul = true;
        k += m;
    }
    return k;
}

size_t oecm_json_str_chars(oecm_span v) {
    if (oecm_json_kind(v) != OECM_J_STRING || v.n < 2)
        return 0;
    const char *p = v.p + 1, *end = v.p + v.n - 1;
    size_t chars = 0;
    while (p < end) {
        uint8_t u[4];
        (void)unit(&p, u);
        if ((u[0] & 0xc0u) != 0x80u)
            chars++;
    }
    return chars;
}

bool oecm_json_i64(oecm_span v, int64_t *out) {
    if (oecm_json_kind(v) != OECM_J_NUMBER)
        return false;
    size_t i = 0;
    bool neg = v.p[0] == '-';
    if (neg)
        i++;
    if (i >= v.n)
        return false;
    uint64_t x = 0;
    for (; i < v.n; i++) {
        char c = v.p[i];
        if (c < '0' || c > '9')
            return false;
        unsigned d = (unsigned)(c - '0');
        if (x > (UINT64_MAX - d) / 10u)
            return false;
        x = x * 10u + d;
    }
    if (neg) {
        if (x > (uint64_t)INT64_MAX + 1u)
            return false;
        *out = x == 0 ? 0 : -(int64_t)(x - 1u) - 1;
    } else {
        if (x > (uint64_t)INT64_MAX)
            return false;
        *out = (int64_t)x;
    }
    return true;
}

bool oecm_json_f64(oecm_span v, double *out) {
    return oecm_json_kind(v) == OECM_J_NUMBER && oecm_parse_f64(v.p, v.n, out);
}

bool oecm_json_bool(oecm_span v, bool *out) {
    if (oecm_json_kind(v) != OECM_J_BOOL)
        return false;
    *out = v.p[0] == 't';
    return true;
}
