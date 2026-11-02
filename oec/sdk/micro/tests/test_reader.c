/* The JSON reader of the core (oecm_json.c, RFC 8259). */
#include <string.h>

#include "oecm_internal.h"
#include "unity.h"

void setUp(void) {}
void tearDown(void) {}

static oecm_span root;

static bool parse(const char *s) { return oecm_json_parse(s, strlen(s), &root); }

static void test_valid_documents(void) {
    static const char *const ok[] = {
        "{}", "[]", " 0 ", "\"\"", "null", "true", "false", "-1.5e3",
        "{\"a\":[1,{\"b\":null}],\"c\":\"\\u00e4\\ud83d\\ude00\\n\"}",
        "\"\xc3\xa4\xe2\x82\xac\xf0\x9f\x98\x80\"", /* UTF-8 of 2, 3 and 4 octets */
        "[[[[[[[[[[[[[[[[]]]]]]]]]]]]]]]]",         /* depth 16 */
    };
    for (size_t i = 0; i < sizeof ok / sizeof ok[0]; i++)
        TEST_ASSERT_TRUE_MESSAGE(parse(ok[i]), ok[i]);
}

static void test_invalid_documents(void) {
    static const char *const bad[] = {
        "", " ", "{", "[1,]", "{\"a\":1,}", "{\"a\" 1}", "{a:1}", "[1 2]", "01", "1.", "+1",
        "nul", "True", "\"a", "\"\\x\"", "\"\\u12\"", "\"\x01\"", /* control character */
        "\"\\ud800\"",                                              /* lone surrogate */
        "\"\\udc00\\ud800\"", "\"\xc3\"", "\"\xc0\xaf\"",           /* overlong */
        "\"\xed\xa0\x80\"",                                         /* UTF-8 surrogate */
        "\"\xf4\x90\x80\x80\"",                                     /* above U+10FFFF */
        "\"\xff\"", "1 2", "{} x", "[[[[[[[[[[[[[[[[[]]]]]]]]]]]]]]]]]", /* depth 17 */
    };
    for (size_t i = 0; i < sizeof bad / sizeof bad[0]; i++)
        TEST_ASSERT_FALSE_MESSAGE(parse(bad[i]), bad[i]);
    TEST_ASSERT_FALSE(oecm_json_parse("\"a\0b\"", 5, &root)); /* raw NUL */
}

static void test_kinds_and_iteration(void) {
    TEST_ASSERT_TRUE(parse("{\"a\":1,\"b\":[true,null,\"x\"],\"c\":{}}"));
    TEST_ASSERT_EQUAL(OECM_J_OBJECT, oecm_json_kind(root));
    oecm_jit it;
    oecm_span k, v;
    oecm_json_iter(root, &it);
    static const oecm_jkind kinds[] = {OECM_J_NUMBER, OECM_J_ARRAY, OECM_J_OBJECT};
    static const char *const keys[] = {"a", "b", "c"};
    size_t n = 0;
    while (oecm_json_next(&it, &k, &v)) {
        TEST_ASSERT_TRUE(n < 3);
        TEST_ASSERT_TRUE(oecm_json_streq(k, keys[n]));
        TEST_ASSERT_EQUAL(kinds[n], oecm_json_kind(v));
        n++;
    }
    TEST_ASSERT_EQUAL_size_t(3, n);
    TEST_ASSERT_TRUE(oecm_json_get(root, "b", &v));
    static const oecm_jkind elements[] = {OECM_J_BOOL, OECM_J_NULL, OECM_J_STRING};
    oecm_json_iter(v, &it);
    n = 0;
    while (oecm_json_next(&it, &k, &v))
        TEST_ASSERT_EQUAL(elements[n++], oecm_json_kind(v));
    TEST_ASSERT_EQUAL_size_t(3, n);
    TEST_ASSERT_TRUE(oecm_json_get(root, "c", &v));
    oecm_json_iter(v, &it);
    TEST_ASSERT_FALSE(oecm_json_next(&it, &k, &v));
}

static void test_get_last_duplicate_and_escaped_keys(void) {
    TEST_ASSERT_TRUE(parse("{\"a\":1,\"x\":{\"a\":3},\"a\":2,\"\\u0062\":4}"));
    oecm_span v;
    int64_t i;
    TEST_ASSERT_TRUE(oecm_json_get(root, "a", &v));
    TEST_ASSERT_TRUE(oecm_json_i64(v, &i));
    TEST_ASSERT_EQUAL_INT64(2, i); /* the last duplicate, not the nested one */
    TEST_ASSERT_TRUE(oecm_json_get(root, "b", &v));
    TEST_ASSERT_TRUE(oecm_json_i64(v, &i));
    TEST_ASSERT_EQUAL_INT64(4, i);
    TEST_ASSERT_FALSE(oecm_json_get(root, "y", &v));
}

static void test_strings(void) {
    TEST_ASSERT_TRUE(parse("\"a\\\"\\\\\\/\\b\\f\\n\\r\\t\\u00e4\\ud83d\\ude00\""));
    char s[32];
    size_t n;
    TEST_ASSERT_TRUE(oecm_json_str(root, s, sizeof s, &n));
    TEST_ASSERT_EQUAL_STRING("a\"\\/\b\f\n\r\t\xc3\xa4\xf0\x9f\x98\x80", s);
    TEST_ASSERT_EQUAL_size_t(strlen(s), n);
    TEST_ASSERT_EQUAL_size_t(n, oecm_json_str_bytes(root, &(bool){false}));
    TEST_ASSERT_EQUAL_size_t(11, oecm_json_str_chars(root));
    TEST_ASSERT_TRUE(oecm_json_str_eq(root, s, n));
    TEST_ASSERT_FALSE(oecm_json_str(root, s, n, &n)); /* no room for the NUL */
    TEST_ASSERT_TRUE(parse("\"a\\u0000b\""));
    bool nul = false;
    TEST_ASSERT_EQUAL_size_t(3, oecm_json_str_bytes(root, &nul));
    TEST_ASSERT_TRUE(nul);
    TEST_ASSERT_TRUE(parse("1"));
    TEST_ASSERT_FALSE(oecm_json_streq(root, "1")); /* not a string */
}

static void test_numbers_and_bools(void) {
    int64_t i;
    double d;
    bool b;
    TEST_ASSERT_TRUE(parse("-9223372036854775808"));
    TEST_ASSERT_TRUE(oecm_json_i64(root, &i));
    TEST_ASSERT_EQUAL_INT64(INT64_MIN, i);
    TEST_ASSERT_TRUE(parse("9223372036854775808"));
    TEST_ASSERT_FALSE(oecm_json_i64(root, &i)); /* out of range */
    TEST_ASSERT_TRUE(parse("1.0"));
    TEST_ASSERT_FALSE(oecm_json_i64(root, &i)); /* not an integer token */
    TEST_ASSERT_TRUE(oecm_json_f64(root, &d));
    TEST_ASSERT_EQUAL_DOUBLE(1.0, d);
    TEST_ASSERT_TRUE(parse("\"1\""));
    TEST_ASSERT_FALSE(oecm_json_f64(root, &d));
    TEST_ASSERT_TRUE(parse("false"));
    TEST_ASSERT_TRUE(oecm_json_bool(root, &b));
    TEST_ASSERT_FALSE(b);
    TEST_ASSERT_TRUE(parse("0"));
    TEST_ASSERT_FALSE(oecm_json_bool(root, &b));
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_valid_documents);
    RUN_TEST(test_invalid_documents);
    RUN_TEST(test_kinds_and_iteration);
    RUN_TEST(test_get_last_duplicate_and_escaped_keys);
    RUN_TEST(test_strings);
    RUN_TEST(test_numbers_and_bools);
    return UNITY_END();
}
