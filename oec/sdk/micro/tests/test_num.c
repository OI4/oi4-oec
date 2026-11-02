/* Numbers, time, SHA-1 and UUIDs of the core (oecm_num.c). */
#include <string.h>

#include "oecm_internal.h"
#include "unity.h"

void setUp(void) {}
void tearDown(void) {}

static char out[128];

static const char *f64(double v, unsigned decimals) {
    oecm_buf b;
    oecm_buf_init(&b, (uint8_t *)out, sizeof out - 1);
    oecm_put_f64(&b, v, decimals);
    TEST_ASSERT_FALSE(b.full);
    out[b.len] = '\0';
    return out;
}

static const char *at(int64_t ms) {
    oecm_buf b;
    oecm_buf_init(&b, (uint8_t *)out, sizeof out - 1);
    oecm_put_time(&b, ms);
    out[b.len] = '\0';
    return out;
}

static void test_put_f64_fixed_decimals(void) {
    TEST_ASSERT_EQUAL_STRING("21.50", f64(21.5, 2));
    TEST_ASSERT_EQUAL_STRING("-3.1", f64(-3.14159, 1));
    TEST_ASSERT_EQUAL_STRING("0", f64(0.4, 0));
    TEST_ASSERT_EQUAL_STRING("1", f64(0.5, 0));
    TEST_ASSERT_EQUAL_STRING("0.00", f64(-0.001, 2)); /* no "-0.00" */
    TEST_ASSERT_EQUAL_STRING("1234567.12345", f64(1234567.12345, 5));
    TEST_ASSERT_EQUAL_STRING("0.000000001", f64(1e-9, 12)); /* at most 9 decimals */
}

static void test_put_f64_large_and_not_finite(void) {
    TEST_ASSERT_EQUAL_STRING("1.0e15", f64(1e15, 2));
    TEST_ASSERT_EQUAL_STRING("-1.23456789012346e20", f64(-1.2345678901234567e20, 0));
    TEST_ASSERT_EQUAL_STRING("1.7976931348623e308", f64(1.7976931348623e308, 3));
    double zero = 0.0;
    TEST_ASSERT_EQUAL_STRING("0", f64(1.0 / zero, 2));
    TEST_ASSERT_EQUAL_STRING("0", f64(zero / zero, 2));
    /* every output is a JSON number */
    static const double vs[] = {1e14, 9.999999e14, 1e300, -5e-300, 123.456};
    for (size_t i = 0; i < sizeof vs / sizeof vs[0]; i++)
        for (unsigned d = 0; d <= 9; d++) {
            const char *s = f64(vs[i], d);
            TEST_ASSERT_TRUE_MESSAGE(oecm_is_json_number(s, strlen(s)), s);
        }
}

static void test_json_number_grammar(void) {
    static const char *const ok[] = {"0", "-0", "1.5", "-12.25e+3", "1E9", "0.0e-0"};
    static const char *const bad[] = {"", "01", "+1", "1.", ".5", "1e", "-", "1e+", "0x1", "NaN",
                                      "1 "};
    for (size_t i = 0; i < sizeof ok / sizeof ok[0]; i++)
        TEST_ASSERT_TRUE_MESSAGE(oecm_is_json_number(ok[i], strlen(ok[i])), ok[i]);
    for (size_t i = 0; i < sizeof bad / sizeof bad[0]; i++)
        TEST_ASSERT_FALSE_MESSAGE(oecm_is_json_number(bad[i], strlen(bad[i])), bad[i]);
}

static void test_parse_f64(void) {
    double v;
    TEST_ASSERT_TRUE(oecm_parse_f64("21.5", 4, &v));
    TEST_ASSERT_EQUAL_DOUBLE(21.5, v);
    TEST_ASSERT_TRUE(oecm_parse_f64("-12.25e+3", 9, &v));
    TEST_ASSERT_EQUAL_DOUBLE(-12250.0, v);
    TEST_ASSERT_TRUE(oecm_parse_f64("0.001", 5, &v));
    TEST_ASSERT_EQUAL_DOUBLE(0.001, v);
    TEST_ASSERT_TRUE(oecm_parse_f64("123456789012345678901234", 24, &v));
    TEST_ASSERT_DOUBLE_WITHIN(1e9, 1.2345678901234568e23, v);
    TEST_ASSERT_TRUE(oecm_parse_f64("1e999", 5, &v));
    TEST_ASSERT_TRUE(v > 1.7976931348623157e308); /* +inf: the caller checks the range */
    TEST_ASSERT_TRUE(oecm_parse_f64("1e-999", 6, &v));
    TEST_ASSERT_EQUAL_DOUBLE(0.0, v);
    TEST_ASSERT_FALSE(oecm_parse_f64("1.", 2, &v));
}

static void test_put_time(void) {
    TEST_ASSERT_EQUAL_STRING("1970-01-01T00:00:00.000Z", at(0));
    TEST_ASSERT_EQUAL_STRING("1970-01-01T00:00:00.000Z", at(-5));
    TEST_ASSERT_EQUAL_STRING("2024-02-29T23:59:59.999Z", at(1709251199999));
    TEST_ASSERT_EQUAL_STRING("2026-11-10T08:00:00.000Z", at(1794297600000));
    TEST_ASSERT_EQUAL_STRING("2000-03-01T00:00:00.001Z", at(951868800001));
}

static void hex(const uint8_t *p, size_t n, char *s) {
    static const char d[] = "0123456789abcdef";
    for (size_t i = 0; i < n; i++) {
        s[2 * i] = d[p[i] >> 4];
        s[2 * i + 1] = d[p[i] & 15u];
    }
    s[2 * n] = '\0';
}

static void test_sha1_vectors(void) {
    uint8_t h[20];
    char s[41];
    oecm_sha1 c;
    oecm_sha1_init(&c);
    oecm_sha1_update(&c, "abc", 3);
    oecm_sha1_final(&c, h);
    hex(h, 20, s);
    TEST_ASSERT_EQUAL_STRING("a9993e364706816aba3e25717850c26c9cd0d89d", s);
    /* 448 bits: the padding needs a second block; fed in pieces */
    const char *m = "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq";
    oecm_sha1_init(&c);
    oecm_sha1_update(&c, m, 5);
    oecm_sha1_update(&c, m + 5, strlen(m) - 5);
    oecm_sha1_final(&c, h);
    hex(h, 20, s);
    TEST_ASSERT_EQUAL_STRING("84983e441c3bd26ebaae4aa1f95129e5e54670f1", s);
    oecm_sha1_init(&c);
    oecm_sha1_final(&c, h);
    hex(h, 20, s);
    TEST_ASSERT_EQUAL_STRING("da39a3ee5e6b4b0d3255bfef95601890afd80709", s);
}

static void test_uuid5(void) {
    /* RFC 4122 name space DNS; Python: uuid.uuid5(uuid.NAMESPACE_DNS, "python.org") */
    static const uint8_t dns[16] = {0x6b, 0xa7, 0xb8, 0x10, 0x9d, 0xad, 0x11, 0xd1,
                                    0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8};
    uint8_t u[16];
    oecm_uuid5(dns, "python.org", 10, u);
    oecm_buf b;
    oecm_buf_init(&b, (uint8_t *)out, sizeof out - 1);
    oecm_put_uuid(&b, u);
    out[b.len] = '\0';
    TEST_ASSERT_EQUAL_STRING("886313e1-3b8a-5372-9b90-0c9aee199e5d", out);
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_put_f64_fixed_decimals);
    RUN_TEST(test_put_f64_large_and_not_finite);
    RUN_TEST(test_json_number_grammar);
    RUN_TEST(test_parse_f64);
    RUN_TEST(test_put_time);
    RUN_TEST(test_sha1_vectors);
    RUN_TEST(test_uuid5);
    return UNITY_END();
}
