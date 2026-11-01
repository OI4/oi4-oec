/* Unit tests of the logger core (D4, ADR-0028 §6): JSON reader and
 * writer, settings from the Config, filter, Event level, records and
 * history. No SDK, no broker; run under AddressSanitizer and UBSan by
 * oec/demos/check.sh (make test). */
#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "../src/core.h"
#include "../src/history.h"
#include "../src/json.h"

static int failures;

#define CHECK(cond)                                                                    \
    do {                                                                               \
        if (!(cond)) {                                                                 \
            fprintf(stderr, "%s:%d: CHECK failed: %s\n", __FILE__, __LINE__, #cond);   \
            failures++;                                                                \
        }                                                                              \
    } while (0)

/* ---- JSON ---------------------------------------------------------------- */

static void json_valid(void) {
    static const char *const ok[] = {
        "null", "true", "false", "0", "-0", "1.5e3", "-12.25E-2", "\"\"", "\"a\\\"b\\\\c\\/\"",
        "\"\\u00e4\\ud83d\\ude00\"", "[]", "{}", " [ 1 , 2 , {\"a\" : [ ] } ] ", "{\"a\":{\"b\":null}}",
        "\"\xc3\xa4\"",
    };
    static const char *const bad[] = {
        "", " ", "nul", "tru", "01", "1.", ".5", "-", "1e", "+1", "\"abc", "\"\\x\"", "\"\\u12\"",
        "\"a\tb\"", "[1,]", "[1 2]", "{\"a\"}", "{\"a\":1,}", "{a:1}", "{\"a\":1} x", "[", "{",
        "\"\\ud83d\"", "\"\\ude00\"", "\"\\ud83dx\"", "\"\xff\"", "\"\xc3\"",
    };
    lj_value v;
    for (size_t i = 0; i < sizeof ok / sizeof ok[0]; i++) {
        if (!lj_parse(ok[i], &v)) {
            fprintf(stderr, "valid JSON refused: %s\n", ok[i]);
            failures++;
        }
    }
    for (size_t i = 0; i < sizeof bad / sizeof bad[0]; i++) {
        if (lj_parse(bad[i], &v)) {
            fprintf(stderr, "invalid JSON accepted: %s\n", bad[i]);
            failures++;
        }
    }
    /* depth limit */
    char deep[2 * LJ_MAX_DEPTH + 8];
    size_t n = 0;
    for (int i = 0; i < LJ_MAX_DEPTH; i++)
        deep[n++] = '[';
    for (int i = 0; i < LJ_MAX_DEPTH; i++)
        deep[n++] = ']';
    deep[n] = 0;
    CHECK(lj_parse(deep, &v));
    memmove(deep + 1, deep, n + 1);
    deep[0] = '[';
    deep[n + 1] = ']';
    deep[n + 2] = 0;
    CHECK(!lj_parse(deep, &v));
}

static void json_access(void) {
    lj_value v, m, k;
    lj_iter it;
    char buf[64];
    double d;

    CHECK(lj_parse(" {\"a\": 1.5, \"b\": \"x\\u00e4\\ud83d\\ude00\\n\", \"c\": {\"d\": [1, 2]}, "
                   "\"a\": 2} ",
                   &v));
    CHECK(v.type == LJ_OBJECT);
    CHECK(lj_member(&v, "a", &m) && m.type == LJ_NUMBER && lj_number(&m, &d) && d == 1.5);
    CHECK(lj_member(&v, "b", &m) && m.type == LJ_STRING);
    CHECK(lj_string(&m, buf, sizeof buf) == 8 && strcmp(buf, "x\xc3\xa4\xf0\x9f\x98\x80\n") == 0);
    CHECK(lj_string(&m, buf, 8) == (size_t)-1);
    CHECK(lj_member(&v, "c", &m) && m.type == LJ_OBJECT && lj_member(&m, "d", &m) &&
          m.type == LJ_ARRAY && m.n == 6);
    CHECK(!lj_member(&v, "z", &m));
    CHECK(!lj_number(&v, &d));

    int count = 0;
    CHECK(lj_object_begin(&v, &it));
    while (lj_object_next(&it, &k, &m))
        count++;
    CHECK(count == 4);
    CHECK(lj_parse("[1]", &v) && !lj_object_begin(&v, &it) && !lj_member(&v, "a", &m));

    CHECK(lj_parse("\"Debug\"", &v) && lj_string_eq(&v, "Debug") && !lj_string_eq(&v, "Debu") &&
          !lj_string_eq(&v, "Debugs"));
    CHECK(lj_parse("\"\\u0044ebug\"", &v) && lj_string_eq(&v, "Debug"));
    CHECK(lj_parse("\"a\\u0000b\"", &v) && lj_string(&v, buf, sizeof buf) == (size_t)-1);
    CHECK(lj_parse("{\"\\u0061\": 7}", &v) && lj_member(&v, "a", &m) && lj_number(&m, &d) && d == 7);
}

static char *capture(void (*f)(FILE *, const void *), const void *arg) {
    char *buf = NULL;
    size_t len = 0;
    FILE *out = open_memstream(&buf, &len);
    if (!out)
        abort();
    f(out, arg);
    fclose(out);
    return buf;
}

static void write_string(FILE *f, const void *s) { lj_write_string(f, s); }

static void json_write(void) {
    char *s = capture(write_string, "a\"b\\c\n\t\x01\xc3\xa4/");
    CHECK(strcmp(s, "\"a\\\"b\\\\c\\n\\t\\u0001\xc3\xa4/\"") == 0);
    free(s);
    s = capture(write_string, NULL);
    CHECK(strcmp(s, "null") == 0);
    free(s);
}

/* ---- settings ------------------------------------------------------------ */

static bool same(const lg_settings *a, const lg_settings *b) {
    if (a->event != b->event || a->health != b->health || a->data != b->data ||
        a->n_sources != b->n_sources || a->min_level != b->min_level || a->format != b->format ||
        strcmp(a->file, b->file) != 0 || a->history != b->history)
        return false;
    for (int i = 0; i < a->n_sources; i++)
        if (strcmp(a->sources[i], b->sources[i]) != 0)
            return false;
    return true;
}

static void settings_defaults(void) {
    lg_settings s, d;
    lj_value def, ctx, group, obj, val;
    char err[128], value[LG_SOURCES_MAX + 1];
    int objects = 0;

    lg_settings_default(&d);
    CHECK(d.event && d.health && !d.data && d.n_sources == 0 && d.min_level == 6 &&
          d.format == LG_FORMAT_JSON && d.file[0] == 0 && d.history == 1000);

    /* The definition is valid JSON; each Value is the default. */
    CHECK(lj_parse(lg_config_definition(), &def));
    CHECK(lj_member(&def, "Context", &ctx) && ctx.type == LJ_OBJECT);
    CHECK(lj_member(&def, LG_CONFIG_GROUP, &group) && group.type == LJ_OBJECT);
    lg_settings_default(&s);
    for (int i = 0; lg_config_names[i]; i++) {
        CHECK(lj_member(&group, lg_config_names[i], &obj));
        CHECK(lj_member(&obj, "Type", &val) && val.type == LJ_STRING);
        CHECK(lj_member(&obj, "Name", &val) && val.type == LJ_OBJECT);
        CHECK(lj_member(&obj, "Value", &val) && lj_string(&val, value, sizeof value) != (size_t)-1);
        CHECK(lj_member(&obj, "DefaultValue", &val) && lj_string_eq(&val, value));
        CHECK(lg_settings_set(&s, lg_config_names[i], value, err, sizeof err));
        objects++;
    }
    CHECK(objects == 8);
    CHECK(same(&s, &d));
}

static void settings_set(void) {
    lg_settings s;
    char err[128];

    lg_settings_default(&s);
    CHECK(lg_settings_set(&s, "data", "true", err, sizeof err) && s.data);
    CHECK(lg_settings_set(&s, "event", "false", err, sizeof err) && !s.event);
    CHECK(lg_settings_set(&s, "event", "", err, sizeof err) && s.event); /* unset = default */
    CHECK(!lg_settings_set(&s, "health", "yes", err, sizeof err) && err[0]);
    CHECK(lg_settings_set(&s, "minEventLevel", "Warning", err, sizeof err) && s.min_level == 4);
    CHECK(!lg_settings_set(&s, "minEventLevel", "Loud", err, sizeof err));
    CHECK(lg_settings_set(&s, "format", "text", err, sizeof err) && s.format == LG_FORMAT_TEXT);
    CHECK(!lg_settings_set(&s, "format", "xml", err, sizeof err));
    CHECK(lg_settings_set(&s, "file", "logger.log", err, sizeof err) &&
          strcmp(s.file, "logger.log") == 0);
    CHECK(lg_settings_set(&s, "file", "", err, sizeof err) && s.file[0] == 0);
    CHECK(!lg_settings_set(&s, "file", "../etc/passwd", err, sizeof err));
    CHECK(!lg_settings_set(&s, "file", ".hidden", err, sizeof err));
    CHECK(!lg_settings_set(&s, "file", "a/b", err, sizeof err));
    CHECK(lg_settings_set(&s, "historySize", "10", err, sizeof err) && s.history == 10);
    CHECK(lg_settings_set(&s, "historySize", "10000", err, sizeof err) && s.history == 10000);
    CHECK(lg_settings_set(&s, "historySize", "500.0", err, sizeof err) && s.history == 500);
    CHECK(!lg_settings_set(&s, "historySize", "9", err, sizeof err) && s.history == 500);
    CHECK(!lg_settings_set(&s, "historySize", "10001", err, sizeof err));
    CHECK(!lg_settings_set(&s, "historySize", "12.5", err, sizeof err));
    CHECK(!lg_settings_set(&s, "historySize", "x", err, sizeof err));
    CHECK(!lg_settings_set(&s, "unknown", "x", err, sizeof err));

    CHECK(lg_settings_set(&s, "sources", "  vendor.com/Pump/P-3/0815 \t example.com\n", err,
                          sizeof err));
    CHECK(s.n_sources == 2 && strcmp(s.sources[0], "vendor.com/Pump/P-3/0815") == 0 &&
          strcmp(s.sources[1], "example.com") == 0);
    CHECK(lg_settings_set(&s, "sources", "", err, sizeof err) && s.n_sources == 0);
    char many[LG_SOURCES_MAX];
    many[0] = 0;
    for (int i = 0; i <= LG_MAX_SOURCES; i++)
        strcat(many, "a.b ");
    CHECK(!lg_settings_set(&s, "sources", many, err, sizeof err) && s.n_sources == 0);
    char longer[LG_SOURCE_MAX + 1];
    memset(longer, 'a', LG_SOURCE_MAX);
    longer[LG_SOURCE_MAX] = 0;
    CHECK(!lg_settings_set(&s, "sources", longer, err, sizeof err));
}

static void settings_proposal(void) {
    lg_settings s;
    char err[128];

    CHECK(lg_settings_from_proposal(
        "{\"common\":{\"data\":\"true\",\"event\":\"true\",\"file\":\"x.log\",\"format\":\"text\","
        "\"health\":\"false\",\"historySize\":\"20\",\"minEventLevel\":\"Error\","
        "\"sources\":\"a.com/M/P/1\",\"future\":\"ignored\"},\"other\":{}}",
        &s, err, sizeof err));
    CHECK(s.data && s.event && !s.health && strcmp(s.file, "x.log") == 0 &&
          s.format == LG_FORMAT_TEXT && s.history == 20 && s.min_level == 3 && s.n_sources == 1);
    CHECK(lg_settings_from_proposal("{\"common\":{}}", &s, err, sizeof err) && s.event &&
          s.history == 1000);
    CHECK(!lg_settings_from_proposal("{\"common\":{\"historySize\":\"1\"}}", &s, err, sizeof err) &&
          strstr(err, "historySize"));
    CHECK(!lg_settings_from_proposal("{\"common\":{\"event\":true}}", &s, err, sizeof err));
    CHECK(!lg_settings_from_proposal("{\"common\":[]}", &s, err, sizeof err));
    CHECK(!lg_settings_from_proposal("[]", &s, err, sizeof err));
    CHECK(!lg_settings_from_proposal("{", &s, err, sizeof err));
}

/* ---- levels, filter, accept --------------------------------------------- */

static void levels(void) {
    CHECK(strcmp(lg_level_name(0), "Emergency") == 0 && strcmp(lg_level_name(7), "Debug") == 0);
    CHECK(lg_level_name(8) == NULL && lg_level_name(-1) == NULL);
    CHECK(lg_level_parse("Notice") == 5 && lg_level_parse("notice") == -1);

    CHECK(lg_event_level("{\"Number\":13,\"Category\":\"CAT_SYSLOG_0\"}") == 5);
    CHECK(lg_event_level("{\"Number\":187,\"Category\":\"CAT_SYSLOG_0\",\"Details\":{}}") == 3);
    CHECK(lg_event_level("{\"Number\":0,\"Category\":\"CAT_STATUS_1\"}") == 6);
    CHECK(lg_event_level("{\"Number\":1073741824,\"Category\":\"CAT_STATUS_1\"}") == 4);
    CHECK(lg_event_level("{\"Number\":2147876864,\"Category\":\"CAT_STATUS_1\"}") == 3);
    CHECK(lg_event_level("{\"Number\":3221225472,\"Category\":\"CAT_STATUS_1\"}") == 3);
    CHECK(lg_event_level("{\"Number\":1,\"Category\":\"CAT_GENERIC_99\"}") == 5);
    CHECK(lg_event_level("{\"Category\":\"CAT_SYSLOG_0\"}") == 5);
    CHECK(lg_event_level("{\"Number\":-1,\"Category\":\"CAT_SYSLOG_0\"}") == 5);
    CHECK(lg_event_level("{\"Number\":1.5,\"Category\":\"CAT_SYSLOG_0\"}") == 5);
    CHECK(lg_event_level("not json") == 5);
}

static void filter(void) {
    lg_settings s;
    char err[64];
    lg_settings_default(&s);
    CHECK(lg_source_match(&s, "a.com/M/P/1", NULL));
    CHECK(lg_settings_set(&s, "sources", "a.com/M/P/1 b.com/X", err, sizeof err));
    CHECK(lg_source_match(&s, "a.com/M/P/1", NULL));
    CHECK(lg_source_match(&s, "z.com/A/B/C", "a.com/M/P/1"));
    CHECK(!lg_source_match(&s, "a.com/M/P/12", NULL));
    CHECK(lg_source_match(&s, "b.com/X/P/1", NULL));
    CHECK(lg_source_match(&s, "b.com/X", NULL));
    CHECK(!lg_source_match(&s, "b.com/XY/P/1", NULL));
    CHECK(!lg_source_match(&s, "z.com/A/B/C", "z.com/A/B/D"));
}

static void accept(void) {
    lg_settings s;
    char err[64];
    int level = 0;
    lg_msg ev = {LG_EVENT, "Utility", "a.com/M/P/1", "a.com/M/P/1", "Syslog/Debug",
                 "{\"Number\":15,\"Category\":\"CAT_SYSLOG_0\"}", 0};
    lg_msg health = {LG_HEALTH, "Utility", "a.com/M/P/1", "a.com/M/P/1", NULL,
                     "{\"Health\":\"NORMAL_0\",\"HealthScore\":100}", 0};
    lg_msg data = {LG_DATA, "OTConnector", "a.com/M/P/1", "v.com/S/T/1", "Oi4Pv", "{\"Pv\":1}", 0};

    lg_settings_default(&s);
    CHECK(!lg_accept(&s, &ev, &level)); /* Debug below Informational */
    CHECK(lg_settings_set(&s, "minEventLevel", "Debug", err, sizeof err));
    CHECK(lg_accept(&s, &ev, &level) && level == 7);
    CHECK(lg_accept(&s, &health, &level) && level == -1);
    CHECK(!lg_accept(&s, &data, &level));
    CHECK(lg_settings_set(&s, "data", "true", err, sizeof err) && lg_accept(&s, &data, &level));
    CHECK(lg_settings_set(&s, "event", "false", err, sizeof err) && !lg_accept(&s, &ev, &level));
    CHECK(lg_settings_set(&s, "minEventLevel", "Emergency", err, sizeof err) &&
          lg_accept(&s, &health, &level)); /* the level applies to Events only */
    CHECK(lg_settings_set(&s, "sources", "v.com/S", err, sizeof err) &&
          lg_accept(&s, &data, &level) && !lg_accept(&s, &health, &level));
}

/* ---- records ------------------------------------------------------------- */

struct rec {
    lg_format format;
    const lg_msg *m;
    int level;
};

static void write_rec(FILE *f, const void *arg) {
    const struct rec *r = arg;
    lg_write_record(f, r->format, r->m, r->level);
}

static void records(void) {
    char t[32];
    lj_value v, m;
    char buf[256];

    lg_format_time(1767225600123LL, t, sizeof t);
    CHECK(strcmp(t, "2026-01-01T00:00:00.123Z") == 0);
    lg_format_time(0, t, sizeof t);
    CHECK(strcmp(t, "1970-01-01T00:00:00.000Z") == 0);

    lg_msg ev = {LG_EVENT, "OTConnector", "a.com/M,20N/P/1", NULL, "Generic/Maintenance",
                 "{\"Number\":1,\"Category\":\"CAT_GENERIC_99\",\"Description\":\"a \\\"b\\\"\"}",
                 1767225600123LL};
    struct rec r = {LG_FORMAT_JSON, &ev, 5};
    char *s = capture(write_rec, &r);
    size_t n = strlen(s);
    CHECK(n > 0 && s[n - 1] == '\n' && strchr(s, '\n') == s + n - 1);
    CHECK(lj_parse(s, &v));
    CHECK(lj_member(&v, "time", &m) && lj_string_eq(&m, "2026-01-01T00:00:00.123Z"));
    CHECK(lj_member(&v, "resource", &m) && lj_string_eq(&m, "Event"));
    CHECK(lj_member(&v, "serviceType", &m) && lj_string_eq(&m, "OTConnector"));
    CHECK(lj_member(&v, "app", &m) && lj_string_eq(&m, "a.com/M,20N/P/1"));
    CHECK(lj_member(&v, "source", &m) && m.type == LJ_NULL);
    CHECK(lj_member(&v, "filter", &m) && lj_string_eq(&m, "Generic/Maintenance"));
    CHECK(lj_member(&v, "level", &m) && lj_string_eq(&m, "Notice"));
    CHECK(lj_member(&v, "payload", &m) && m.type == LJ_OBJECT && lj_member(&m, "Description", &m) &&
          lj_string(&m, buf, sizeof buf) == 5 && strcmp(buf, "a \"b\"") == 0);
    free(s);

    /* no level for other resources; an invalid payload becomes a string */
    lg_msg bad = {LG_HEALTH, "Utility", "a.com/M/P/1", "a.com/M/P/1", NULL, "{oops\n", 0};
    r = (struct rec){LG_FORMAT_JSON, &bad, -1};
    s = capture(write_rec, &r);
    CHECK(lj_parse(s, &v) && !lj_member(&v, "level", &m));
    CHECK(lj_member(&v, "payload", &m) && lj_string_eq(&m, "{oops\n"));
    free(s);

    /* text: one line */
    r = (struct rec){LG_FORMAT_TEXT, &ev, 5};
    s = capture(write_rec, &r);
    CHECK(strcmp(s, "2026-01-01T00:00:00.123Z Event Notice OTConnector a.com/M,20N/P/1 - "
                    "Generic/Maintenance {\"Number\":1,\"Category\":\"CAT_GENERIC_99\","
                    "\"Description\":\"a \\\"b\\\"\"}\n") == 0);
    free(s);
    r = (struct rec){LG_FORMAT_TEXT, &bad, -1};
    s = capture(write_rec, &r);
    CHECK(strcmp(s, "1970-01-01T00:00:00.000Z Health - Utility a.com/M/P/1 a.com/M/P/1 - {oops \n") ==
          0);
    free(s);
}

/* ---- history ------------------------------------------------------------- */

struct seen {
    unsigned n;
    uint64_t seq[16];
    char time[16][32];
};

static void on_record(void *ud, uint64_t seq, const char *json) {
    struct seen *s = ud;
    lj_value v, m;
    if (s->n < 16) {
        s->seq[s->n] = seq;
        if (lj_parse(json, &v) && lj_member(&v, "time", &m))
            lj_string(&m, s->time[s->n], sizeof s->time[0]);
    }
    s->n++;
}

struct series {
    unsigned n;
    char field[8][16];
    unsigned points[8];
    double first[8], last[8];
    int64_t last_time[8];
    lg_resource resource[8];
    bool has_source[8];
};

static void on_series(void *ud, const lg_series_view *v) {
    struct series *s = ud;
    if (s->n < 8) {
        snprintf(s->field[s->n], sizeof s->field[0], "%s", v->field);
        s->points[s->n] = v->n;
        s->first[s->n] = v->n ? v->points[0].value : NAN;
        s->last[s->n] = v->n ? v->points[v->n - 1].value : NAN;
        s->last_time[s->n] = v->n ? v->points[v->n - 1].time_ms : -1;
        s->resource[s->n] = v->resource;
        s->has_source[s->n] = v->source != NULL;
    }
    s->n++;
}

static void history(void) {
    CHECK(lg_history_new(LG_HISTORY_MIN - 1) == NULL);
    CHECK(lg_history_new(LG_HISTORY_MAX + 1) == NULL);
    lg_history *h = lg_history_new(LG_HISTORY_MIN);
    CHECK(h != NULL);
    if (!h)
        return;
    char payload[64];
    lg_msg m = {LG_DATA, "OTConnector", "a.com/M/P/1", "v.com/S/T/1", "Oi4Pv", payload, 0};
    for (int i = 1; i <= 15; i++) {
        snprintf(payload, sizeof payload,
                 "{\"Pv\":%d.5,\"Sv1\":%d,\"Name\":\"x\",\"On\":true,\"Arr\":[1]}", i, -i);
        m.time_ms = i * 1000LL;
        lg_history_add(h, &m, -1);
    }
    CHECK(lg_history_count(h) == 10);
    struct seen seen = {0};
    lg_history_each(h, on_record, &seen);
    CHECK(seen.n == 10 && seen.seq[0] == 6 && seen.seq[9] == 15);
    CHECK(strcmp(seen.time[0], "1970-01-01T00:00:06.000Z") == 0);

    struct series ser = {0};
    lg_history_series_each(h, on_series, &ser);
    CHECK(ser.n == 2 && lg_history_series_count(h) == 2);
    CHECK(strcmp(ser.field[0], "Pv") == 0 && ser.points[0] == 15 && ser.first[0] == 1.5 &&
          ser.last[0] == 15.5 && ser.last_time[0] == 15000 && ser.resource[0] == LG_DATA &&
          ser.has_source[0]);
    CHECK(strcmp(ser.field[1], "Sv1") == 0 && ser.last[1] == -15);

    /* Health: HealthScore only; no source */
    lg_msg health = {LG_HEALTH, "Utility", "b.com/M/P/1", NULL, NULL,
                     "{\"Health\":\"NORMAL_0\",\"HealthScore\":80}", 99000};
    lg_history_add(h, &health, -1);
    health.payload = "{\"Health\":\"NORMAL_0\"}";
    lg_history_add(h, &health, -1);
    /* Events add records only */
    lg_msg ev = {LG_EVENT, "Utility", "b.com/M/P/1", "b.com/M/P/1", "Syslog/Error",
                 "{\"Number\":11,\"Category\":\"CAT_SYSLOG_0\",\"Details\":{\"x\":1}}", 99001};
    lg_history_add(h, &ev, 3);
    memset(&ser, 0, sizeof ser);
    lg_history_series_each(h, on_series, &ser);
    CHECK(ser.n == 3 && strcmp(ser.field[2], "HealthScore") == 0 && ser.points[2] == 1 &&
          ser.last[2] == 80 && ser.resource[2] == LG_HEALTH && !ser.has_source[2]);

    /* resize keeps the newest */
    CHECK(lg_history_resize(h, 12));
    CHECK(lg_history_resize(h, LG_HISTORY_MIN) && lg_history_count(h) == 10);
    CHECK(!lg_history_resize(h, LG_HISTORY_MAX + 1));
    memset(&seen, 0, sizeof seen);
    lg_history_each(h, on_record, &seen);
    CHECK(seen.n == 10 && seen.seq[0] == 9 && seen.seq[9] == 18);
    CHECK(lg_history_resize(h, 20) && lg_history_count(h) == 10);
    lg_history_add(h, &ev, 3);
    memset(&seen, 0, sizeof seen);
    lg_history_each(h, on_record, &seen);
    CHECK(seen.n == 11 && seen.seq[0] == 9 && seen.seq[10] == 19);

    /* a series keeps its newest LG_SERIES_POINTS points */
    m.filter = "Long";
    for (int i = 0; i < LG_SERIES_POINTS + 5; i++) {
        snprintf(payload, sizeof payload, "{\"v\":%d}", i);
        m.time_ms = i;
        lg_history_add(h, &m, -1);
    }
    memset(&ser, 0, sizeof ser);
    lg_history_series_each(h, on_series, &ser);
    CHECK(ser.n == 4 && ser.points[3] == LG_SERIES_POINTS && ser.first[3] == 5 &&
          ser.last[3] == LG_SERIES_POINTS + 4);

    /* at most LG_MAX_SERIES series */
    for (int i = 0; i < LG_MAX_SERIES; i++) {
        snprintf(payload, sizeof payload, "{\"f%d\":1}", i);
        lg_history_add(h, &m, -1);
    }
    CHECK(lg_history_series_count(h) == LG_MAX_SERIES && lg_history_series_dropped(h) == 4);
    lg_history_free(h);
    lg_history_free(NULL);
}

int main(void) {
    json_valid();
    json_access();
    json_write();
    settings_defaults();
    settings_set();
    settings_proposal();
    levels();
    filter();
    accept();
    records();
    history();
    if (failures) {
        fprintf(stderr, "%d failure(s)\n", failures);
        return 1;
    }
    printf("logger unit tests: ok\n");
    return 0;
}
