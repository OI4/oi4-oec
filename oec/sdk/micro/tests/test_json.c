/* The output buffer and the JSON encoding. */
#include <string.h>

#include "oecm_internal.h"
#include "unity.h"

void setUp(void) {}
void tearDown(void) {}

static char out[64];

static oecm_buf fresh(size_t cap) {
    oecm_buf b;
    memset(out, 'X', sizeof out);
    oecm_buf_init(&b, (uint8_t *)out, cap);
    return b;
}

static void test_overflow_sets_full_and_never_writes_past_cap(void) {
    oecm_buf b = fresh(4);
    oecm_puts(&b, "abc");
    TEST_ASSERT_FALSE(b.full);
    oecm_puts(&b, "de");
    TEST_ASSERT_TRUE(b.full);
    TEST_ASSERT_TRUE(b.len <= 4);
    TEST_ASSERT_EQUAL_CHAR('X', out[4]);
    oecm_putc(&b, 'z');
    oecm_put_u64(&b, 12345);
    TEST_ASSERT_TRUE(b.full);
    TEST_ASSERT_EQUAL_CHAR('X', out[4]);
}

static void test_integers(void) {
    oecm_buf b = fresh(sizeof out - 1);
    oecm_put_u64(&b, 0);
    oecm_putc(&b, ' ');
    oecm_put_u64(&b, UINT64_MAX);
    oecm_putc(&b, ' ');
    oecm_put_i64(&b, -1);
    oecm_putc(&b, ' ');
    oecm_put_i64(&b, INT64_MIN);
    oecm_putc(&b, ' ');
    oecm_put_i64(&b, 2147483647);
    out[b.len] = '\0';
    TEST_ASSERT_EQUAL_STRING("0 18446744073709551615 -1 -9223372036854775808 2147483647", out);
}

static void test_json_escaping(void) {
    oecm_buf b = fresh(sizeof out - 1);
    const char s[] = "a\"b\\c\n\t\x01\x1f\x7f/\xc3\xa4";
    oecm_put_json_str(&b, s, sizeof s - 1);
    out[b.len] = '\0';
    TEST_ASSERT_EQUAL_STRING("\"a\\\"b\\\\c\\n\\t\\u0001\\u001f\x7f/\xc3\xa4\"", out);
}

static void test_b64url_lengths(void) {
    TEST_ASSERT_EQUAL_size_t(0, oecm_b64url_len(0));
    TEST_ASSERT_EQUAL_size_t(2, oecm_b64url_len(1));
    TEST_ASSERT_EQUAL_size_t(3, oecm_b64url_len(2));
    TEST_ASSERT_EQUAL_size_t(4, oecm_b64url_len(3));
    TEST_ASSERT_EQUAL_size_t(OECM_IL_ENCODED_MAX, oecm_b64url_len(OECM_IL_MAX));
    oecm_buf b = fresh(sizeof out - 1);
    oecm_put_b64url(&b, (const uint8_t *)"\xfb\xff", 2);
    out[b.len] = '\0';
    TEST_ASSERT_EQUAL_STRING("-_8", out);
}

static void test_dnp_keeps_unreserved(void) {
    oecm_buf b = fresh(sizeof out - 1);
    oecm_put_dnp(&b, "aZ09-._~ ,/", 11);
    out[b.len] = '\0';
    TEST_ASSERT_EQUAL_STRING("aZ09-._~,20,2C,2F", out);
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_overflow_sets_full_and_never_writes_past_cap);
    RUN_TEST(test_integers);
    RUN_TEST(test_json_escaping);
    RUN_TEST(test_b64url_lengths);
    RUN_TEST(test_dnp_keeps_unreserved);
    return UNITY_END();
}
