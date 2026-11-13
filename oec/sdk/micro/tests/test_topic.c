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

static bool match(const char *f, const char *topic) {
    return oecm_mqtt_match(f, strlen(f), topic, strlen(topic));
}

/* The cases of oec-core topic/subscription.rs mqtt_wildcards. */
static void test_mqtt_match(void) {
    TEST_ASSERT_TRUE(match("a/#", "a"));
    TEST_ASSERT_TRUE(match("a/#", "a/b/c"));
    TEST_ASSERT_TRUE(match("a/+/c", "a/b/c"));
    TEST_ASSERT_TRUE(match("a/+/c", "a//c"));
    TEST_ASSERT_TRUE(match("#", "a/b"));
    TEST_ASSERT_TRUE(match("+", ""));
    TEST_ASSERT_TRUE(match("a/b", "a/b"));
    TEST_ASSERT_FALSE(match("a/+", "a/b/c"));
    TEST_ASSERT_FALSE(match("a/+/c", "a/b"));
    TEST_ASSERT_FALSE(match("a/b", "a/b/c"));
    TEST_ASSERT_FALSE(match("a/b/c", "a/b"));
    TEST_ASSERT_FALSE(match("a/b", "a/c"));
    TEST_ASSERT_FALSE(match("#", "$SYS/x"));
    TEST_ASSERT_FALSE(match("+/x", "$SYS/x"));
    TEST_ASSERT_TRUE(match("$SYS/#", "$SYS/x"));
}

static oecm_sub_parts parts;

static bool sub(const char *f) {
    return oecm_sub_filter_parse(f, strlen(f), &parts);
}

static void test_consumer_filters(void) {
    TEST_ASSERT_TRUE(sub("Oi4v2/OTConnector/+/Pub/Data/+/Climate"));
    TEST_ASSERT_TRUE(oecm_span_eq(parts.resource, "Data"));
    TEST_ASSERT_TRUE(oecm_span_eq(parts.filter, "Climate"));
    TEST_ASSERT_TRUE(oecm_span_eq(parts.group_tail, "Climate"));
    TEST_ASSERT_FALSE(parts.source_all);
    TEST_ASSERT_TRUE(sub("Oi4v2/+/+/+/Event/+/Status/+"));
    TEST_ASSERT_TRUE(oecm_span_eq(parts.filter, "Status/+"));
    TEST_ASSERT_EQUAL_size_t(0, parts.group_tail.n); /* a wildcard Filter level */
    TEST_ASSERT_TRUE(sub("Oi4v2/OTConnector/+/Pub/Health/#"));
    TEST_ASSERT_TRUE(parts.source_all);
    TEST_ASSERT_EQUAL_size_t(0, parts.filter.n);
    TEST_ASSERT_TRUE(sub("Oi4v2/Registry/abc/Pub/MAM"));
    static const char *const bad[] = {
        "",
        "Oi4v1/OTConnector/+/Pub/Data",
        "Oi4v2/OTConnector/+/Pub",           /* fewer than 5 levels */
        "Oi4v2/OTConnector/+/Get/Data",      /* not Pub */
        "Oi4v2/OTConnector/+/Pub/+",         /* Resource level a wildcard */
        "Oi4v2/OTConnector/+/Pub/#",
        "Oi4v2/#",
        "Oi4v2/OTConnector/+/Pub/Data/#/x",  /* '#' not the last level */
        "Oi4v2/OTConnector/a+/Pub/Data",     /* wildcard inside a level */
        "Oi4v2/OTConnector//Pub/Data",       /* empty level */
        "Oi4v2/OTConnector/+/Pub/Data/",
        "Oi4v2/OTConnector/+/Pub/Data/\xff", /* not UTF-8 */
    };
    for (size_t i = 0; i < sizeof bad / sizeof bad[0]; i++)
        TEST_ASSERT_FALSE_MESSAGE(sub(bad[i]), bad[i]);
}

static void test_topic_identification_links(void) {
    char il[OECM_IL_BUF];
    /* base64url of "provider.com/Sensor/S-1/4711" and "other.com/X/Y/1" */
    const char *tp = "Oi4v2/OTConnector/cHJvdmlkZXIuY29tL1NlbnNvci9TLTEvNDcxMQ/Pub/Data/"
                    "b3RoZXIuY29tL1gvWS8x/Climate";
    TEST_ASSERT_EQUAL(OECM_OK, oecm_topic_app(tp, il, sizeof il));
    TEST_ASSERT_EQUAL_STRING("provider.com/Sensor/S-1/4711", il);
    TEST_ASSERT_EQUAL(OECM_OK, oecm_topic_source(tp, il, sizeof il));
    TEST_ASSERT_EQUAL_STRING("other.com/X/Y/1", il);
    TEST_ASSERT_EQUAL(OECM_E_NOSPACE, oecm_topic_app(tp, il, 16));
    TEST_ASSERT_EQUAL(OECM_E_ARG,
                      oecm_topic_source("Oi4v2/OTConnector/b3RoZXIuY29tL1gvWS8x/Pub/MAM", il,
                                        sizeof il));
    TEST_ASSERT_EQUAL_STRING("", il);
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_topic_app("Oi4v2/OTConnector/Zg==/Pub/MAM", il, sizeof il));
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_topic_app("a/b/c/d/e", il, sizeof il));
    TEST_ASSERT_EQUAL(OECM_E_ARG, oecm_topic_app(NULL, il, sizeof il));
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_levels);
    RUN_TEST(test_rejects);
    RUN_TEST(test_b64url);
    RUN_TEST(test_mqtt_match);
    RUN_TEST(test_consumer_filters);
    RUN_TEST(test_topic_identification_links);
    return UNITY_END();
}
