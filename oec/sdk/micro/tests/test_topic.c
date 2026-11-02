/* The topic parser of the core (oecm_topic.c, MG 6.3). */
#include <string.h>

#include "oecm_internal.h"
#include "unity.h"

void setUp(void) {}
void tearDown(void) {}

static oecm_topic t;

static bool parse(const char *s) { return oecm_topic_parse(s, strlen(s), &t); }

static void test_levels(void) {
    TEST_ASSERT_TRUE(parse("Oi4v2/OTConnector/QUJD/Get/Data/REVG/Temp/Inner"));
    TEST_ASSERT_TRUE(oecm_span_eq(t.st, "OTConnector"));
    TEST_ASSERT_TRUE(oecm_span_eq(t.app, "QUJD"));
    TEST_ASSERT_TRUE(oecm_span_eq(t.method, "Get"));
    TEST_ASSERT_TRUE(oecm_span_eq(t.resource, "Data"));
    TEST_ASSERT_TRUE(oecm_span_eq(t.source, "REVG"));
    TEST_ASSERT_TRUE(oecm_span_eq(t.filter, "Temp/Inner")); /* several levels */
    TEST_ASSERT_TRUE(parse("Oi4v2/Registry/QUJD/Get/MAM"));
    TEST_ASSERT_EQUAL_size_t(0, t.source.n);
    TEST_ASSERT_EQUAL_size_t(0, t.filter.n);
    TEST_ASSERT_TRUE(parse("Oi4v2/Registry/QUJD/Get/MAM/REVG"));
    TEST_ASSERT_TRUE(oecm_span_eq(t.source, "REVG"));
    TEST_ASSERT_EQUAL_size_t(0, t.filter.n);
}

static void test_rejects(void) {
    static const char *const bad[] = {
        "",
        "Oi4v2/Registry/QUJD/Get",        /* fewer than 5 levels */
        "Oi4v3/Registry/QUJD/Get/MAM",    /* other version */
        "Oi4v2//QUJD/Get/MAM",            /* empty level */
        "Oi4v2/Registry/QUJD/Get/MAM/",   /* empty Source */
        "Oi4v2/Registry/QUJD/Get/MAM/S/", /* empty Filter */
        "Oi4v2/Registry/QUJD/Get/Data/S/a//b",
        "Oi4v2/Registry/QUJD/Get/Data/S/a/",
        "Oi4v2/Registry/+/Get/MAM", /* wildcards */
        "Oi4v2/Registry/QUJD/Get/#",
    };
    for (size_t i = 0; i < sizeof bad / sizeof bad[0]; i++)
        TEST_ASSERT_FALSE_MESSAGE(parse(bad[i]), bad[i]);
    TEST_ASSERT_FALSE(oecm_topic_parse("Oi4v2/R/QUJD/Get/MAM\0x", 22, &t)); /* NUL */
    TEST_ASSERT_FALSE(oecm_topic_parse(NULL, 0, &t));
}

static void test_b64url(void) {
    /* RFC 4648 §10 vectors in base64url without padding */
    static const char *const plain[] = {"", "f", "fo", "foo", "foob", "fooba", "foobar"};
    static const char *const enc[] = {"", "Zg", "Zm8", "Zm9v", "Zm9vYg", "Zm9vYmE", "Zm9vYmFy"};
    for (size_t i = 0; i < 7; i++) {
        oecm_span e = {enc[i], strlen(enc[i])};
        TEST_ASSERT_TRUE_MESSAGE(oecm_b64url_eq(e, plain[i], strlen(plain[i])), enc[i]);
    }
    oecm_span pad = {"Zg==", 4}, other = {"Zh", 2}, std = {"-_8", 3};
    TEST_ASSERT_FALSE(oecm_b64url_eq(pad, "f", 1));   /* padding is not canonical */
    TEST_ASSERT_FALSE(oecm_b64url_eq(other, "f", 1)); /* non-zero spare bits */
    TEST_ASSERT_TRUE(oecm_b64url_eq(std, "\xfb\xff", 2));
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_levels);
    RUN_TEST(test_rejects);
    RUN_TEST(test_b64url);
    return UNITY_END();
}
