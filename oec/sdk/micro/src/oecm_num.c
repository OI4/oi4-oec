/*
 * Numbers, timestamps and UUIDs without the C library (ADR-0030 §2): the
 * core links against memcmp/memcpy/memmove/memset/strchr/strlen only.
 */
#include <string.h>

#include "oecm_internal.h"

/* --- Doubles ----------------------------------------------------------------- */

/* 1e0 … 1e22 are exact doubles. */
static const double pow10_exact[] = {1e0,  1e1,  1e2,  1e3,  1e4,  1e5,  1e6,  1e7,
                                     1e8,  1e9,  1e10, 1e11, 1e12, 1e13, 1e14, 1e15,
                                     1e16, 1e17, 1e18, 1e19, 1e20, 1e21, 1e22};

static double scale10(double v, int e) {
    while (e > 22) {
        v *= 1e22;
        e -= 22;
    }
    while (e < -22) {
        v /= 1e22;
        e += 22;
    }
    return e >= 0 ? v * pow10_exact[e] : v / pow10_exact[-e];
}

static bool is_digit(char c) { return c >= '0' && c <= '9'; }

/* End of a JSON number token at p[i], 0 if there is none. */
static size_t number_end(const char *p, size_t i, size_t n) {
    if (i < n && p[i] == '-')
        i++;
    if (i >= n)
        return 0;
    if (p[i] == '0') {
        i++;
    } else if (p[i] >= '1' && p[i] <= '9') {
        while (i < n && is_digit(p[i]))
            i++;
    } else {
        return 0;
    }
    if (i < n && p[i] == '.') {
        i++;
        if (i >= n || !is_digit(p[i]))
            return 0;
        while (i < n && is_digit(p[i]))
            i++;
    }
    if (i < n && (p[i] == 'e' || p[i] == 'E')) {
        i++;
        if (i < n && (p[i] == '+' || p[i] == '-'))
            i++;
        if (i >= n || !is_digit(p[i]))
            return 0;
        while (i < n && is_digit(p[i]))
            i++;
    }
    return i;
}

bool oecm_is_json_number(const char *p, size_t n) { return n > 0 && number_end(p, 0, n) == n; }

/* 19 significant digits, then the decimal exponent; not correctly rounded
 * in every case, exact for up to 15 significant digits and exponents
 * within ±22. */
bool oecm_parse_f64(const char *p, size_t n, double *out) {
    if (!oecm_is_json_number(p, n))
        return false;
    size_t i = 0;
    bool neg = p[0] == '-';
    if (neg)
        i++;
    uint64_t m = 0;
    int digits = 0;
    long e = 0;
    bool point = false;
    for (; i < n && p[i] != 'e' && p[i] != 'E'; i++) {
        if (p[i] == '.') {
            point = true;
            continue;
        }
        if (digits < 19) {
            if (m != 0 || p[i] != '0')
                digits++;
            m = m * 10u + (uint64_t)(p[i] - '0');
            if (point)
                e--;
        } else if (!point) {
            e++;
        }
    }
    if (i < n) {
        i++;
        bool eneg = false;
        if (p[i] == '+' || p[i] == '-')
            eneg = p[i++] == '-';
        long x = 0;
        for (; i < n; i++)
            if (x < 100000)
                x = x * 10 + (p[i] - '0');
        e += eneg ? -x : x;
    }
    double v = (double)m;
    if (m != 0) {
        if (e > 400)
            e = 400;
        if (e < -400)
            e = -400;
        v = scale10(v, (int)e);
    }
    *out = neg ? -v : v;
    return true;
}

static bool is_finite(double v) { return v == v && v - v == 0.0; }

void oecm_put_f64(oecm_buf *b, double v, unsigned decimals) {
    if (!is_finite(v)) {
        oecm_puts(b, "0");
        return;
    }
    if (decimals > 9)
        decimals = 9;
    double a = v < 0 ? -v : v;
    if (a < 1e15) {
        double scaled = a * pow10_exact[decimals] + 0.5;
        if (scaled < 1.8e19) {
            uint64_t r = (uint64_t)scaled;
            uint64_t unit = (uint64_t)pow10_exact[decimals];
            if (v < 0 && r != 0)
                oecm_putc(b, '-');
            oecm_put_u64(b, r / unit);
            if (decimals > 0) {
                char frac[9];
                uint64_t f = r % unit;
                for (unsigned i = decimals; i > 0; i--) {
                    frac[i - 1] = (char)('0' + f % 10u);
                    f /= 10u;
                }
                oecm_putc(b, '.');
                oecm_put(b, frac, decimals);
            }
            return;
        }
    }
    /* d.dddddddddddddde±x with 15 significant digits */
    int e = 0;
    while (a >= 1e22) {
        a /= 1e22;
        e += 22;
    }
    while (a >= 10.0) {
        a /= 10.0;
        e++;
    }
    uint64_t d = (uint64_t)(a * 1e14 + 0.5);
    if (d >= 1000000000000000u) {
        d /= 10u;
        e++;
    }
    char t[15];
    for (int i = 14; i >= 0; i--) {
        t[i] = (char)('0' + d % 10u);
        d /= 10u;
    }
    size_t last = 14;
    while (last > 1 && t[last] == '0')
        last--;
    if (v < 0)
        oecm_putc(b, '-');
    oecm_putc(b, t[0]);
    oecm_putc(b, '.');
    oecm_put(b, t + 1, last);
    oecm_putc(b, 'e');
    oecm_put_u64(b, (uint64_t)e);
}

/* --- Time -------------------------------------------------------------------- */

static void put2(oecm_buf *b, unsigned v) {
    char d[2] = {(char)('0' + v / 10u % 10u), (char)('0' + v % 10u)};
    oecm_put(b, d, 2);
}

void oecm_put_time(oecm_buf *b, int64_t unix_ms) {
    if (unix_ms < 0)
        unix_ms = 0;
    uint64_t ms = (uint64_t)unix_ms;
    uint64_t secs = ms / 1000u;
    uint64_t days = secs / 86400u;
    unsigned sod = (unsigned)(secs % 86400u);
    /* civil_from_days (H. Hinnant), days since 1970-01-01 >= 0 */
    uint64_t z = days + 719468u;
    uint64_t era = z / 146097u;
    uint64_t doe = z - era * 146097u;
    uint64_t yoe = (doe - doe / 1460u + doe / 36524u - doe / 146096u) / 365u;
    uint64_t y = yoe + era * 400u;
    uint64_t doy = doe - (365u * yoe + yoe / 4u - yoe / 100u);
    uint64_t mp = (5u * doy + 2u) / 153u;
    unsigned d = (unsigned)(doy - (153u * mp + 2u) / 5u + 1u);
    unsigned m = (unsigned)(mp < 10u ? mp + 3u : mp - 9u);
    if (m <= 2u)
        y++;
    char yy[4] = {(char)('0' + y / 1000u % 10u), (char)('0' + y / 100u % 10u),
                  (char)('0' + y / 10u % 10u), (char)('0' + y % 10u)};
    oecm_put(b, yy, 4);
    oecm_putc(b, '-');
    put2(b, m);
    oecm_putc(b, '-');
    put2(b, d);
    oecm_putc(b, 'T');
    put2(b, sod / 3600u);
    oecm_putc(b, ':');
    put2(b, sod / 60u % 60u);
    oecm_putc(b, ':');
    put2(b, sod % 60u);
    oecm_putc(b, '.');
    unsigned frac = (unsigned)(ms % 1000u);
    oecm_putc(b, (char)('0' + frac / 100u));
    put2(b, frac % 100u);
    oecm_putc(b, 'Z');
}

/* --- SHA-1 and UUIDv5 ---------------------------------------------------------- */

static uint32_t rol(uint32_t x, unsigned n) { return x << n | x >> (32u - n); }

static void sha1_block(oecm_sha1 *c) {
    uint32_t w[80];
    for (unsigned i = 0; i < 16; i++)
        w[i] = (uint32_t)c->block[4 * i] << 24 | (uint32_t)c->block[4 * i + 1] << 16 |
               (uint32_t)c->block[4 * i + 2] << 8 | (uint32_t)c->block[4 * i + 3];
    for (unsigned i = 16; i < 80; i++)
        w[i] = rol(w[i - 3] ^ w[i - 8] ^ w[i - 14] ^ w[i - 16], 1);
    uint32_t a = c->h[0], b = c->h[1], cc = c->h[2], d = c->h[3], e = c->h[4];
    for (unsigned i = 0; i < 80; i++) {
        uint32_t f, k;
        if (i < 20) {
            f = (b & cc) | (~b & d);
            k = 0x5a827999u;
        } else if (i < 40) {
            f = b ^ cc ^ d;
            k = 0x6ed9eba1u;
        } else if (i < 60) {
            f = (b & cc) | (b & d) | (cc & d);
            k = 0x8f1bbcdcu;
        } else {
            f = b ^ cc ^ d;
            k = 0xca62c1d6u;
        }
        uint32_t t = rol(a, 5) + f + e + k + w[i];
        e = d;
        d = cc;
        cc = rol(b, 30);
        b = a;
        a = t;
    }
    c->h[0] += a;
    c->h[1] += b;
    c->h[2] += cc;
    c->h[3] += d;
    c->h[4] += e;
}

void oecm_sha1_init(oecm_sha1 *c) {
    c->h[0] = 0x67452301u;
    c->h[1] = 0xefcdab89u;
    c->h[2] = 0x98badcfeu;
    c->h[3] = 0x10325476u;
    c->h[4] = 0xc3d2e1f0u;
    c->len = 0;
}

void oecm_sha1_update(oecm_sha1 *c, const void *p, size_t n) {
    const uint8_t *s = p;
    for (size_t i = 0; i < n; i++) {
        c->block[c->len % 64u] = s[i];
        c->len++;
        if (c->len % 64u == 0)
            sha1_block(c);
    }
}

void oecm_sha1_final(oecm_sha1 *c, uint8_t out[20]) {
    uint64_t bits = c->len * 8u;
    uint8_t pad = 0x80;
    oecm_sha1_update(c, &pad, 1);
    pad = 0;
    while (c->len % 64u != 56u)
        oecm_sha1_update(c, &pad, 1);
    uint8_t l[8];
    for (unsigned i = 0; i < 8; i++)
        l[i] = (uint8_t)(bits >> (56u - 8u * i));
    oecm_sha1_update(c, l, 8);
    for (unsigned i = 0; i < 20; i++)
        out[i] = (uint8_t)(c->h[i / 4u] >> (24u - 8u * (i % 4u)));
}

void oecm_uuid5(const uint8_t ns[16], const void *name, size_t n, uint8_t out[16]) {
    oecm_sha1 c;
    uint8_t h[20];
    oecm_sha1_init(&c);
    oecm_sha1_update(&c, ns, 16);
    oecm_sha1_update(&c, name, n);
    oecm_sha1_final(&c, h);
    memcpy(out, h, 16);
    out[6] = (uint8_t)((out[6] & 0x0fu) | 0x50u);
    out[8] = (uint8_t)((out[8] & 0x3fu) | 0x80u);
}

void oecm_put_uuid(oecm_buf *b, const uint8_t u[16]) {
    static const char hex[] = "0123456789abcdef";
    char t[36];
    size_t k = 0;
    for (unsigned i = 0; i < 16; i++) {
        if (i == 4 || i == 6 || i == 8 || i == 10)
            t[k++] = '-';
        t[k++] = hex[u[i] >> 4];
        t[k++] = hex[u[i] & 15u];
    }
    oecm_put(b, t, sizeof t);
}
