/* Identification Link, base64url, dnp-encoding and Oi4Identifier against the
 * shared vectors (G-1.4, ADR-0030 §4), as oec-core tests/vectors.rs. */
#include <string.h>

#include "oecm_internal.h"
#include "unity.h"
#include "vectors_gen.h"

void setUp(void) {}
void tearDown(void) {}

/* "a,b" of the violation bits in vector order. */
static const char *codes(unsigned v, char *out, size_t cap) {
    oecm_buf b;
    oecm_buf_init(&b, (uint8_t *)out, cap - 1);
    for (unsigned i = 0; i < OECM_IL_VIOLATION_KINDS; i++) {
        if (v & (1u << i)) {
            if (b.len > 0)
                oecm_putc(&b, ',');
            oecm_puts(&b, oecm_il_violation_code(i));
        }
    }
    out[b.len] = '\0';
    return out;
}

static void test_identification_link_vectors(void) {
    char msg[64], got[128], buf[1024];
    for (size_t i = 0; i < VEC_IL_ALL_COUNT; i++) {
        const vec_il *v = &vec_il_all[i];
        snprintf(msg, sizeof msg, "%s", v->id.s);
        if (v->il.s) {
            TEST_ASSERT_EQUAL_STRING_MESSAGE(v->violations.s,
                                             codes(oecm_il_check(v->il.s, v->il.n), got, sizeof got), msg);
            if (v->valid && v->encoded.s) {
                oecm_buf b;
                oecm_buf_init(&b, (uint8_t *)buf, sizeof buf - 1);
                oecm_put_b64url(&b, (const uint8_t *)v->il.s, v->il.n);
                buf[b.len] = '\0';
                TEST_ASSERT_EQUAL_STRING_MESSAGE(v->encoded.s, buf, msg);
                TEST_ASSERT_EQUAL_size_t_MESSAGE(oecm_b64url_len(v->il.n), b.len, msg);
            }
        }
        if (v->encoded.s) {
            size_t n;
            unsigned viol = oecm_il_decode(v->encoded.s, v->encoded.n, buf, OECM_IL_MAX + 1, &n);
            TEST_ASSERT_EQUAL_STRING_MESSAGE(v->violations.s, codes(viol, got, sizeof got), msg);
            if (v->valid && v->il.s) {
                TEST_ASSERT_EQUAL_size_t_MESSAGE(v->il.n, n, msg);
                TEST_ASSERT_EQUAL_MEMORY_MESSAGE(v->il.s, buf, n, msg);
            }
        }
    }
}

static void test_oi4identifier_vectors(void) {
    char msg[64], got[128], out[OECM_IL_MAX + 1];
    for (size_t i = 0; i < VEC_OID_ALL_COUNT; i++) {
        const vec_oid *v = &vec_oid_all[i];
        snprintf(msg, sizeof msg, "%s", v->id.s);
        size_t n;
        unsigned viol = oecm_oi4identifier(v->manufacturer_uri.s, v->model.s, v->product_code.s,
                                           v->serial_number.s, out, sizeof out, &n);
        TEST_ASSERT_EQUAL_STRING_MESSAGE(v->violations.s, codes(viol, got, sizeof got), msg);
        if (v->valid)
            TEST_ASSERT_EQUAL_STRING_MESSAGE(v->oi4identifier.s, out, msg);
    }
}

static void test_dnp_vectors(void) {
    char msg[64], out[1024];
    for (size_t i = 0; i < VEC_DNP_ALL_COUNT; i++) {
        const vec_dnp *v = &vec_dnp_all[i];
        snprintf(msg, sizeof msg, "%s", v->id.s);
        oecm_buf b;
        oecm_buf_init(&b, (uint8_t *)out, sizeof out - 1);
        oecm_put_dnp(&b, v->input.s, v->input.n);
        out[b.len] = '\0';
        TEST_ASSERT_EQUAL_STRING_MESSAGE(v->output.s, out, msg);
    }
}

static void test_base64url_canonical(void) {
    char out[OECM_IL_MAX + 1];
    size_t n;
    TEST_ASSERT_EQUAL_UINT(0, oecm_il_decode("YQ", 2, out, sizeof out, &n));
    TEST_ASSERT_EQUAL_STRING("a", out);
    TEST_ASSERT_EQUAL_UINT(OECM_IL_ENCODING, oecm_il_decode("YR", 2, out, sizeof out, &n));
    TEST_ASSERT_EQUAL_UINT(OECM_IL_ENCODING, oecm_il_decode("YQ==", 4, out, sizeof out, &n));
    TEST_ASSERT_EQUAL_UINT(OECM_IL_ENCODING, oecm_il_decode("Y", 1, out, sizeof out, &n));
    TEST_ASSERT_EQUAL_UINT(OECM_IL_ENCODING, oecm_il_decode("Y+Q", 3, out, sizeof out, &n));
    TEST_ASSERT_EQUAL_UINT(OECM_IL_EMPTY, oecm_il_decode("", 0, out, sizeof out, &n));
}

static void test_too_long_decode_is_bounded(void) {
    /* 344 characters decode to 258 octets: more than the buffer holds. */
    char e[345], out[OECM_IL_MAX + 1];
    for (size_t i = 0; i < 344; i++)
        e[i] = "ZXhh"[i % 4]; /* "exa" repeated */
    e[344] = '\0';
    size_t n;
    unsigned v = oecm_il_decode(e, 344, out, sizeof out, &n);
    TEST_ASSERT_TRUE(v & OECM_IL_TOO_LONG);
    TEST_ASSERT_EQUAL_size_t(OECM_IL_MAX, n);
}

static void test_utf8(void) {
    TEST_ASSERT_TRUE(oecm_utf8_valid("Gr\xc3\xbc\xc3\x9f" "e \xe2\x82\xac \xf0\x9f\x98\x80", 16));
    TEST_ASSERT_FALSE(oecm_utf8_valid("\xc3", 1));
    TEST_ASSERT_FALSE(oecm_utf8_valid("\xc0\xaf", 2));       /* overlong */
    TEST_ASSERT_FALSE(oecm_utf8_valid("\xed\xa0\x80", 3));   /* surrogate */
    TEST_ASSERT_FALSE(oecm_utf8_valid("\xf4\x90\x80\x80", 4)); /* > U+10FFFF */
    TEST_ASSERT_FALSE(oecm_utf8_valid("\xe2\x82", 2));
}

static void test_manufacturer_uri(void) {
    TEST_ASSERT_TRUE(oecm_manufacturer_uri_valid("provider.com", 12));
    TEST_ASSERT_TRUE(oecm_manufacturer_uri_valid("a-b.c.de", 8));
    TEST_ASSERT_FALSE(oecm_manufacturer_uri_valid("www.provider.com", 16));
    TEST_ASSERT_FALSE(oecm_manufacturer_uri_valid("provider", 8));
    TEST_ASSERT_FALSE(oecm_manufacturer_uri_valid("-a.com", 6));
    TEST_ASSERT_FALSE(oecm_manufacturer_uri_valid("a-.com", 6));
    TEST_ASSERT_FALSE(oecm_manufacturer_uri_valid("a..com", 6));
    TEST_ASSERT_FALSE(oecm_manufacturer_uri_valid("Provider.com", 12));
    TEST_ASSERT_FALSE(oecm_manufacturer_uri_valid("provider.com/x", 14));
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_identification_link_vectors);
    RUN_TEST(test_oi4identifier_vectors);
    RUN_TEST(test_dnp_vectors);
    RUN_TEST(test_base64url_canonical);
    RUN_TEST(test_too_long_decode_is_bounded);
    RUN_TEST(test_utf8);
    RUN_TEST(test_manufacturer_uri);
    return UNITY_END();
}
