/*
 * Identification Link (MG 4.1), base64url topic elements (MG 4.2),
 * Oi4Identifier and dnp-encoding (MG 4.3, MG A3). Same rules and violation
 * codes as oec-core il.rs, checked against the shared vectors.
 */
#include <string.h>

#include "oecm_internal.h"

static const char *const violation_codes[OECM_IL_VIOLATION_KINDS] = {
    "empty",           "too-long", "forbidden-character", "empty-authority", "uppercase-authority",
    "encoding",        "manufacturer-uri",
};

const char *oecm_il_violation_code(unsigned bit_index) {
    return bit_index < OECM_IL_VIOLATION_KINDS ? violation_codes[bit_index] : "";
}

static bool is_alpha(unsigned char c) { return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'); }
static bool is_digit(unsigned char c) { return c >= '0' && c <= '9'; }
static bool is_alnum(unsigned char c) { return is_alpha(c) || is_digit(c); }

/* Letters, digits, unreserved and reserved characters of IEC 61406-1
 * Annex A (REQ-IL-016). */
static bool il_allowed(unsigned char c) {
    return is_alnum(c) || (c != 0 && strchr("-._~:/?#[]@!$&'()*+,;=", c) != NULL);
}

/* Length of "scheme://", 0 without a valid scheme (RFC 3986). */
static size_t scheme_len(const char *s, size_t n) {
    for (size_t i = 0; i + 3 <= n; i++) {
        if (s[i] == ':' && s[i + 1] == '/' && s[i + 2] == '/') {
            if (i == 0 || !is_alpha((unsigned char)s[0]))
                return 0;
            for (size_t j = 1; j < i; j++) {
                unsigned char c = (unsigned char)s[j];
                if (!is_alnum(c) && c != '+' && c != '-' && c != '.')
                    return 0;
            }
            return i + 3;
        }
    }
    return 0;
}

unsigned oecm_il_check(const char *s, size_t n) {
    if (n == 0)
        return OECM_IL_EMPTY;
    unsigned v = 0;
    if (n > OECM_IL_MAX)
        v |= OECM_IL_TOO_LONG;
    for (size_t i = 0; i < n; i++) {
        if (!il_allowed((unsigned char)s[i])) {
            v |= OECM_IL_FORBIDDEN;
            break;
        }
    }
    size_t start = scheme_len(s, n);
    size_t end = start;
    while (end < n && s[end] != '/' && s[end] != '?' && s[end] != '#')
        end++;
    if (end == start) {
        v |= OECM_IL_EMPTY_AUTHORITY;
    } else {
        for (size_t i = start; i < end; i++) {
            if (s[i] >= 'A' && s[i] <= 'Z') {
                v |= OECM_IL_UPPERCASE_AUTHORITY;
                break;
            }
        }
    }
    return v;
}

/* --- base64url (RFC 4648 §5, unpadded, canonical) -------------------------- */

static const char b64_alphabet[] =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

static int b64_value(unsigned char c) {
    if (c >= 'A' && c <= 'Z')
        return c - 'A';
    if (c >= 'a' && c <= 'z')
        return c - 'a' + 26;
    if (c >= '0' && c <= '9')
        return c - '0' + 52;
    if (c == '-')
        return 62;
    if (c == '_')
        return 63;
    return -1;
}

size_t oecm_b64url_len(size_t n) { return n / 3u * 4u + (n % 3u == 0 ? 0u : n % 3u + 1u); }

void oecm_put_b64url(oecm_buf *b, const uint8_t *s, size_t n) {
    size_t i = 0;
    char q[4];
    for (; i + 3 <= n; i += 3) {
        uint32_t x = (uint32_t)s[i] << 16 | (uint32_t)s[i + 1] << 8 | s[i + 2];
        q[0] = b64_alphabet[x >> 18 & 63u];
        q[1] = b64_alphabet[x >> 12 & 63u];
        q[2] = b64_alphabet[x >> 6 & 63u];
        q[3] = b64_alphabet[x & 63u];
        oecm_put(b, q, 4);
    }
    if (n - i == 1) {
        uint32_t x = (uint32_t)s[i] << 16;
        q[0] = b64_alphabet[x >> 18 & 63u];
        q[1] = b64_alphabet[x >> 12 & 63u];
        oecm_put(b, q, 2);
    } else if (n - i == 2) {
        uint32_t x = (uint32_t)s[i] << 16 | (uint32_t)s[i + 1] << 8;
        q[0] = b64_alphabet[x >> 18 & 63u];
        q[1] = b64_alphabet[x >> 12 & 63u];
        q[2] = b64_alphabet[x >> 6 & 63u];
        oecm_put(b, q, 3);
    }
}

unsigned oecm_il_decode(const char *e, size_t n, char *out, size_t cap, size_t *out_len) {
    *out_len = 0;
    if (cap == 0)
        return OECM_IL_ENCODING;
    out[0] = '\0';
    if (n % 4u == 1u)
        return OECM_IL_ENCODING;
    size_t len = 0;
    bool truncated = false;
    uint32_t acc = 0;
    unsigned bits = 0;
    for (size_t i = 0; i < n; i++) {
        int v = b64_value((unsigned char)e[i]);
        if (v < 0)
            return OECM_IL_ENCODING;
        acc = (acc << 6 | (uint32_t)v) & 0xffffu;
        bits += 6;
        if (bits >= 8) {
            bits -= 8;
            if (len + 1 < cap)
                out[len++] = (char)(uint8_t)(acc >> bits);
            else
                truncated = true;
        }
    }
    /* Canonical: the unused low bits of the last character are zero. */
    if (bits > 0 && (acc & ((1u << bits) - 1u)) != 0)
        return OECM_IL_ENCODING;
    out[len] = '\0';
    *out_len = len;
    unsigned v = oecm_il_check(out, len);
    if (truncated)
        v |= OECM_IL_TOO_LONG;
    return v;
}

/* --- dnp-encoding, ManufacturerUri, Oi4Identifier ------------------------- */

static bool dnp_keep(unsigned char c) { return is_alnum(c) || c == '-' || c == '.' || c == '_' || c == '~'; }

void oecm_put_dnp(oecm_buf *b, const char *s, size_t n) {
    static const char hex[] = "0123456789ABCDEF";
    for (size_t i = 0; i < n; i++) {
        unsigned char c = (unsigned char)s[i];
        if (dnp_keep(c)) {
            oecm_putc(b, (char)c);
        } else {
            char m[3] = {',', hex[c >> 4], hex[c & 15u]};
            oecm_put(b, m, 3);
        }
    }
}

static size_t dnp_len(const char *s, size_t n) {
    size_t len = 0;
    for (size_t i = 0; i < n; i++)
        len += dnp_keep((unsigned char)s[i]) ? 1u : 3u;
    return len;
}

bool oecm_manufacturer_uri_valid(const char *s, size_t n) {
    if (n >= 4 && memcmp(s, "www.", 4) == 0)
        return false;
    size_t labels = 0;
    size_t start = 0;
    for (size_t i = 0; i <= n; i++) {
        if (i < n && s[i] != '.') {
            unsigned char c = (unsigned char)s[i];
            if (!((c >= 'a' && c <= 'z') || is_digit(c) || c == '-'))
                return false;
            continue;
        }
        if (i == start || s[start] == '-' || s[i - 1] == '-')
            return false;
        labels++;
        start = i + 1;
    }
    return labels >= 2;
}

unsigned oecm_oi4identifier(const char *manufacturer_uri, const char *model,
                            const char *product_code, const char *serial_number, char *out,
                            size_t cap, size_t *out_len) {
    *out_len = 0;
    if (cap > 0)
        out[0] = '\0';
    const char *uri = manufacturer_uri ? manufacturer_uri : "";
    size_t uri_len = strlen(uri);
    oecm_buf b;
    oecm_buf_init(&b, (uint8_t *)out, cap > 0 ? cap - 1 : 0);
    for (size_t i = 0; i < uri_len; i++) {
        char c = uri[i];
        oecm_putc(&b, c >= 'A' && c <= 'Z' ? (char)(c - 'A' + 'a') : c);
    }
    if (b.full || !oecm_manufacturer_uri_valid(out, b.len))
        return OECM_IL_MANUFACTURER_URI;
    const char *elements[3] = {model, product_code, serial_number};
    size_t total = uri_len;
    for (size_t k = 0; k < 3; k++) {
        const char *e = elements[k] ? elements[k] : "";
        size_t n = strlen(e);
        oecm_putc(&b, '/');
        if (n == 0) {
            oecm_puts(&b, "nd");
            total += 3;
        } else {
            oecm_put_dnp(&b, e, n);
            total += 1 + dnp_len(e, n);
        }
    }
    /* Every element character is allowed and the authority is the
     * lower-case ManufacturerUri, so only the length can fail. */
    if (b.full || total > OECM_IL_MAX)
        return OECM_IL_TOO_LONG;
    out[b.len] = '\0';
    *out_len = b.len;
    return oecm_il_check(out, b.len);
}

bool oecm_utf8_valid(const char *str, size_t n) {
    const unsigned char *s = (const unsigned char *)str;
    size_t i = 0;
    while (i < n) {
        unsigned char c = s[i];
        size_t more;
        uint32_t cp;
        if (c < 0x80) {
            i++;
            continue;
        } else if (c >= 0xc2 && c <= 0xdf) {
            more = 1;
            cp = c & 0x1fu;
        } else if (c >= 0xe0 && c <= 0xef) {
            more = 2;
            cp = c & 0x0fu;
        } else if (c >= 0xf0 && c <= 0xf4) {
            more = 3;
            cp = c & 0x07u;
        } else {
            return false;
        }
        if (n - i <= more)
            return false;
        for (size_t k = 1; k <= more; k++) {
            if ((s[i + k] & 0xc0u) != 0x80u)
                return false;
            cp = cp << 6 | (s[i + k] & 0x3fu);
        }
        if ((more == 2 && (cp < 0x800 || (cp >= 0xd800 && cp <= 0xdfff))) ||
            (more == 3 && (cp < 0x10000 || cp > 0x10ffff)))
            return false;
        i += more + 1;
    }
    return true;
}
