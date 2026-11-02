/*
 * Requests to the device from Level 2 (MG 8.2, MG 9.1; ADR-0030): Get of
 * the served resources and Set/Del answered with a Status event
 * (REQ-RES-EVENT-005, REQ-CMP-043), as the Rust provider decides them
 * (oec-core provider.rs, provider/write.rs, config.rs, sublist.rs). The
 * first violation is the first in document order (Rust: key order).
 */
#include <string.h>

#include "oecm_internal.h"

#define MASK "********"

static const char *const resources[] = {"MAM",     "Health",          "Config",
                                        "Data",    "Metadata",        "Event",
                                        "Profile", "PublicationList", "SubscriptionList",
                                        "ReferenceDesignation"};
#define RESOURCES (sizeof resources / sizeof resources[0])

/* The PublicationMode names and their bits ON_REQUEST 1, APPLICATION 2,
 * SOURCE 4, FILTER 8 (oec-core provider/data.rs). */
static const char *const mode_names[] = {"OFF_0",
                                         "ON_REQUEST_1",
                                         "APPLICATION_2",
                                         "SOURCE_3",
                                         "FILTER_4",
                                         "APPLICATION_SOURCE_5",
                                         "APPLICATION_FILTER_6",
                                         "SOURCE_FILTER_7",
                                         "APPLICATION_SOURCE_FILTER_8",
                                         "ON_REQUEST_FILTER_9",
                                         "ON_REQUEST_SOURCE_10",
                                         "ON_REQUEST_APPLICATION_11",
                                         "ON_REQUEST_SOURCE_FILTER_12",
                                         "ON_REQUEST_APPLICATION_FILTER_13",
                                         "ON_REQUEST_APPLICATION_SOURCE_14",
                                         "ON_REQUEST_APPLICATION_SOURCE_FILTER_15"};
static const uint8_t mode_bits[] = {0, 1, 2, 4, 8, 6, 10, 12, 14, 9, 5, 3, 13, 11, 7, 15};
#define MODES (sizeof mode_bits / sizeof mode_bits[0])

static const char *const pub_configs[] = {"NONE_0", "MODE_1", "INTERVAL_2",
                                          "MODE_AND_INTERVAL_3"};
static const char *const cfg_type_name[] = {"Boolean", "Number", "String"};

typedef struct req {
    oecm_state *s;
    oecm_topic t;
    oecm_span rest; /* the request topic from its Resource level on */
    oecm_span root;
    oecm_span id; /* the MessageId token: the ReplyTo */
} req;

/* --- Helpers ------------------------------------------------------------------ */

static size_t index_of(oecm_span a, const char *const *names, size_t n) {
    for (size_t i = 0; i < n; i++)
        if (oecm_span_eq(a, names[i]))
            return i;
    return SIZE_MAX;
}

static size_t json_index_of(oecm_span v, const char *const *names, size_t n) {
    for (size_t i = 0; i < n; i++)
        if (oecm_json_streq(v, names[i]))
            return i;
    return SIZE_MAX;
}

/* The first level of s and the rest after its '/' (rest.n == 0: none). */
static void split_level(oecm_span s, oecm_span *head, oecm_span *rest) {
    size_t i = 0;
    while (i < s.n && s.p[i] != '/')
        i++;
    *head = (oecm_span){s.p, i};
    *rest = i < s.n ? (oecm_span){s.p + i + 1, s.n - i - 1} : (oecm_span){NULL, 0};
}

static bool is_null(oecm_span v) { return oecm_json_kind(v) == OECM_J_NULL; }

/* A present member that is not null. */
static bool member(oecm_span obj, const char *key, oecm_span *v) {
    return oecm_json_get(obj, key, v) && !is_null(*v);
}

static bool json_u32(oecm_span v, uint32_t *out) {
    int64_t x;
    if (!oecm_json_i64(v, &x) || x < 0 || x > (int64_t)UINT32_MAX)
        return false;
    *out = (uint32_t)x;
    return true;
}

static bool cstr_eq(const char *a, size_t n, const char *b) {
    return strlen(b) == n && memcmp(a, b, n) == 0;
}

/* `^[0-9]+-<ServiceType>/.+` (REQ-MSG-007), on the token text. */
static bool message_id_ok(oecm_span v) {
    if (oecm_json_kind(v) != OECM_J_STRING)
        return false;
    const char *p = v.p + 1, *end = v.p + v.n - 1, *q = p;
    while (q < end && *q >= '0' && *q <= '9')
        q++;
    if (q == p || q == end || *q != '-')
        return false;
    const char *st = ++q;
    while (q < end && *q != '/')
        q++;
    if (q >= end || q + 1 == end)
        return false;
    oecm_span name = {st, (size_t)(q - st)};
    for (int i = OECM_ST_REGISTRY; i <= OECM_ST_IT_CONNECTOR; i++)
        if (oecm_span_eq(name, oecm_service_type_name((oecm_service_type)i)))
            return true;
    return false;
}

static bool own_source(const req *r) {
    return oecm_b64url_eq(r->t.source, r->s->il, r->s->il_len);
}

static size_t find_data(const oecm_config *c, oecm_span tag) {
    for (size_t j = 0; j < c->data_count; j++)
        if (oecm_span_eq(tag, c->data[j].tag))
            return j;
    return SIZE_MAX;
}

static size_t find_context(const oecm_config *c, oecm_span name) {
    for (size_t i = 0; i < c->config_count; i++)
        if (oecm_span_eq(name, c->config[i].name))
            return i;
    return SIZE_MAX;
}

static void status(req *r, const oecm_status *st) { (void)oecm_publish_status(r->s, st, r->id); }

/* --- Get (MG 8.2.2, REQ-CMP-041) ------------------------------------------------- */

/* The PaginationRequest (DataSetWriterId 1, REQ-PAG-001) of the first
 * helper DataSetMessage; false if the request is malformed. */
static bool pagination(const req *r, oecm_answer *a) {
    oecm_span msgs, k, m;
    if (!oecm_json_get(r->root, "Messages", &msgs))
        return true;
    if (oecm_json_kind(msgs) != OECM_J_ARRAY)
        return false;
    oecm_jit it;
    oecm_json_iter(msgs, &it);
    while (oecm_json_next(&it, &k, &m)) {
        oecm_span w, p, x;
        int64_t id;
        if (oecm_json_kind(m) != OECM_J_OBJECT || !oecm_json_get(m, "DataSetWriterId", &w) ||
            !oecm_json_i64(w, &id) || id != 1)
            continue;
        if (!oecm_json_get(m, "Payload", &p))
            return true;
        if (oecm_json_kind(p) != OECM_J_OBJECT || !oecm_json_get(p, "PerPage", &x) ||
            !json_u32(x, &a->per_page) || !oecm_json_get(p, "Page", &x) ||
            !json_u32(x, &a->page))
            return false;
        a->paginate = true;
        return true;
    }
    return true;
}

static void add(oecm_answer *a, size_t i) {
    if (a->n < OECM_ITEMS_MAX)
        a->items[a->n++] = (uint8_t)i;
}

static void get(req *r) {
    oecm_state *s = r->s;
    const oecm_config *c = s->cfg;
    oecm_answer a;
    memset(&a, 0, sizeof a);
    if (!pagination(r, &a))
        return;
    if (r->t.source.n != 0 && !own_source(r)) {
        status(r, &oecm_bad_not_found); /* under the AppId (REQ-CMP-047) */
        return;
    }
    oecm_span res = r->t.resource, f = r->t.filter;
    a.topic_rest = r->rest;
    a.reply_to = r->id;
    if (oecm_span_eq(res, "Metadata")) {
        size_t j = f.n != 0 ? find_data(c, f) : SIZE_MAX;
        if (r->t.source.n != 0 && j != SIZE_MAX)
            (void)oecm_publish_metadata(s, j, r->id);
        return;
    }
    if (oecm_span_eq(res, "MAM") || oecm_span_eq(res, "Health") ||
        oecm_span_eq(res, "Profile")) {
        if (f.n != 0)
            return;
        a.res = oecm_span_eq(res, "MAM")      ? OECM_R_MAM
                : oecm_span_eq(res, "Health") ? OECM_R_HEALTH
                                              : OECM_R_PROFILE;
        add(&a, 0);
    } else if (oecm_span_eq(res, "Config")) {
        a.res = OECM_R_CONFIG;
        for (size_t i = 0; i < c->config_count; i++)
            if (f.n == 0 || oecm_span_eq(f, c->config[i].name))
                add(&a, i);
    } else if (oecm_span_eq(res, "Data")) {
        a.res = OECM_R_DATA;
        for (size_t j = 0; j < c->data_count; j++)
            if (f.n == 0 || oecm_span_eq(f, c->data[j].tag))
                add(&a, j);
    } else if (oecm_span_eq(res, "PublicationList")) {
        oecm_span head, tag;
        split_level(f, &head, &tag);
        a.res = OECM_R_PUBLIST;
        for (size_t i = 0; i < oecm_publist_count(s); i++) {
            oecm_pub_entry e;
            oecm_publist_entry(s, i, &e);
            if (f.n == 0 || (oecm_span_eq(head, e.resource) &&
                             (tag.n == 0 || (e.filter && oecm_span_eq(tag, e.filter)))))
                add(&a, i);
        }
    } else if (oecm_span_eq(res, "SubscriptionList")) {
        oecm_span head, tag;
        split_level(f, &head, &tag);
        a.res = OECM_R_SUBLIST;
        for (size_t i = 0; i < oecm_sublist_count(s); i++) {
            const char *er, *et;
            oecm_sublist_entry(s, i, &er, &et);
            if (f.n == 0 || (oecm_span_eq(head, er) && (tag.n == 0 || oecm_span_eq(tag, et))))
                add(&a, i);
        }
    } else {
        return; /* not served by Get */
    }
    (void)oecm_answer_publish(s, &a);
}

/* --- Set and Del (MG 9.1) ---------------------------------------------------------- */

/* The one DataSetMessage of the resource; helper DataSets (DataSetWriterId
 * 1 … 9, REQ-MSG-019) aside. NULL: found. */
static const oecm_status *single(const req *r, oecm_span *dsm) {
    oecm_span msgs, k, m;
    if (!oecm_json_get(r->root, "Messages", &msgs) || oecm_json_kind(msgs) != OECM_J_ARRAY)
        return &oecm_bad_decoding;
    size_t count = 0;
    oecm_jit it;
    oecm_json_iter(msgs, &it);
    while (oecm_json_next(&it, &k, &m)) {
        oecm_span w;
        int64_t id;
        if (oecm_json_kind(m) == OECM_J_OBJECT && oecm_json_get(m, "DataSetWriterId", &w) &&
            oecm_json_i64(w, &id) && id >= 1 && id <= 9)
            continue;
        if (count++ == 0)
            *dsm = m;
    }
    if (count != 1)
        return &oecm_bad_invalid_argument;
    return oecm_json_kind(*dsm) == OECM_J_OBJECT ? NULL : &oecm_bad_decoding;
}

/* The Payload object of the one DataSetMessage. NULL: found. */
static const oecm_status *payload_object(const req *r, oecm_span *payload) {
    oecm_span dsm;
    const oecm_status *e = single(r, &dsm);
    if (e)
        return e;
    if (!oecm_json_get(dsm, "Payload", payload) || oecm_json_kind(*payload) != OECM_J_OBJECT)
        return &oecm_bad_decoding;
    return NULL;
}

/* --- Set Data (REQ-RES-DATA-009, 011, 012) --- */

static bool special_double(oecm_span v, double *out) {
    if (oecm_json_streq(v, "NaN"))
        *out = __builtin_nan("");
    else if (oecm_json_streq(v, "Infinity"))
        *out = __builtin_inf();
    else if (oecm_json_streq(v, "-Infinity"))
        *out = -__builtin_inf();
    else
        return false;
    return true;
}

static bool is_finite(double x) { return x == x && x - x == 0.0; }

/* The value of field f in v; NULL: valid. */
static const oecm_status *field_check(const oecm_field *f, oecm_span v) {
    int64_t i;
    double d;
    bool b, nul;
    switch (f->type) {
    case OECM_TYPE_BOOLEAN:
        return oecm_json_bool(v, &b) ? NULL : &oecm_bad_type_mismatch;
    case OECM_TYPE_INT32:
        return oecm_json_i64(v, &i) && i >= INT32_MIN && i <= INT32_MAX
                   ? NULL
                   : &oecm_bad_type_mismatch;
    case OECM_TYPE_UINT32:
        return oecm_json_i64(v, &i) && i >= 0 && i <= (int64_t)UINT32_MAX
                   ? NULL
                   : &oecm_bad_type_mismatch;
    case OECM_TYPE_FLOAT:
    case OECM_TYPE_DOUBLE:
        if (special_double(v, &d))
            return NULL;
        if (!oecm_json_f64(v, &d))
            return &oecm_bad_type_mismatch;
        if (f->type == OECM_TYPE_FLOAT && is_finite(d) && !is_finite((double)(float)d))
            return &oecm_bad_type_mismatch;
        return NULL;
    case OECM_TYPE_STRING:
        if (oecm_json_kind(v) != OECM_J_STRING)
            return &oecm_bad_type_mismatch;
        /* the buffer of the device bounds the value */
        return oecm_json_str_bytes(v, &nul) < f->cap && !nul ? NULL : &oecm_bad_out_of_range;
    }
    return &oecm_bad_type_mismatch;
}

/* Writes a checked value; true if it changed. */
static bool field_write(const oecm_field *f, oecm_span v) {
    union {
        bool b;
        int32_t i;
        uint32_t u;
        float f;
        double d;
    } x;
    size_t size = 0;
    int64_t i = 0;
    double d = 0.0;
    switch (f->type) {
    case OECM_TYPE_BOOLEAN:
        (void)oecm_json_bool(v, &x.b);
        size = sizeof x.b;
        break;
    case OECM_TYPE_INT32:
        (void)oecm_json_i64(v, &i);
        x.i = (int32_t)i;
        size = sizeof x.i;
        break;
    case OECM_TYPE_UINT32:
        (void)oecm_json_i64(v, &i);
        x.u = (uint32_t)i;
        size = sizeof x.u;
        break;
    case OECM_TYPE_FLOAT:
    case OECM_TYPE_DOUBLE:
        if (!special_double(v, &d))
            (void)oecm_json_f64(v, &d);
        if (f->type == OECM_TYPE_FLOAT) {
            x.f = (float)d;
            size = sizeof x.f;
        } else {
            x.d = d;
            size = sizeof x.d;
        }
        break;
    case OECM_TYPE_STRING: {
        char *dst = f->value;
        size_t n = 0;
        while (n < f->cap && dst[n] != '\0')
            n++;
        if (n < f->cap && oecm_json_str_eq(v, dst, n))
            return false;
        (void)oecm_json_str(v, dst, f->cap, NULL);
        return true;
    }
    }
    if (memcmp(f->value, &x, size) == 0)
        return false;
    memcpy(f->value, &x, size);
    return true;
}

static const oecm_status *set_data(req *r) {
    oecm_state *s = r->s;
    const oecm_config *c = s->cfg;
    size_t j = r->t.filter.n != 0 ? find_data(c, r->t.filter) : SIZE_MAX;
    if (j == SIZE_MAX)
        return &oecm_bad_not_found;
    const oecm_data *d = &c->data[j];
    if (!d->writable)
        return &oecm_bad_not_writable;
    oecm_span payload, v;
    const oecm_status *e = payload_object(r, &payload);
    if (e)
        return e;
    for (size_t k = 0; k < d->field_count; k++) {
        if (!oecm_json_get(payload, d->fields[k].name, &v))
            return &oecm_bad_invalid_argument;
        if ((e = field_check(&d->fields[k], v)) != NULL)
            return e;
    }
    bool changed = false;
    for (size_t k = 0; k < d->field_count; k++) {
        (void)oecm_json_get(payload, d->fields[k].name, &v);
        changed |= field_write(&d->fields[k], v);
    }
    if (changed) {
        /* the answer does not replace the publication on change
         * (REQ-RES-DATA-012 notes, REQ-MQTT-019) */
        s->data_due = (uint8_t)(s->data_due | (1u << j));
        if (c->on_data)
            c->on_data(c->app_ctx, j);
    }
    (void)oecm_publish_data(s, j, r->id);
    return &oecm_good;
}

/* --- Set PublicationList (REQ-RES-PUBLIST-016, 022) --- */

static uint8_t mode_key(uint8_t bits) {
    uint8_t k = (uint8_t)(bits & 14u);
    return k != 0 ? k : bits;
}

static bool entry_is(oecm_span f, const oecm_pub_entry *e) {
    if (e->filter == NULL)
        return oecm_span_eq(f, e->resource);
    size_t n = strlen(e->resource);
    return f.n == n + 1 + strlen(e->filter) && memcmp(f.p, e->resource, n) == 0 &&
           f.p[n] == '/' && memcmp(f.p + n + 1, e->filter, f.n - n - 1) == 0;
}

/* Checks the requested entry p (a PublicationList entry DataSet) against
 * e: NULL if unchanged. */
static const oecm_status *publist_change(const req *r, oecm_span p, const oecm_pub_entry *e) {
    oecm_span v, k, x;
    /* decodable (else BadDecodingError) */
    if (!oecm_json_get(p, "Resource", &v) || json_index_of(v, resources, RESOURCES) == SIZE_MAX)
        return &oecm_bad_decoding;
    bool resource_same = oecm_json_streq(v, e->resource);
    if (!oecm_json_get(p, "Source", &v) || oecm_json_kind(v) != OECM_J_STRING)
        return &oecm_bad_decoding;
    bool source_same = oecm_json_str_eq(v, r->s->il, r->s->il_len);
    bool filter_same = e->filter == NULL;
    if (member(p, "Filter", &v)) {
        if (oecm_json_kind(v) != OECM_J_STRING)
            return &oecm_bad_decoding;
        filter_same = e->filter != NULL && oecm_json_streq(v, e->filter);
    }
    int64_t id;
    if (!oecm_json_get(p, "DataSetWriterId", &v) || !oecm_json_i64(v, &id) || id < 0 ||
        id > 65535)
        return &oecm_bad_decoding;
    uint8_t mode = 1; /* absent: ON_REQUEST_1 */
    if (member(p, "Mode", &v)) {
        size_t m = json_index_of(v, mode_names, MODES);
        if (m == SIZE_MAX)
            return &oecm_bad_decoding;
        mode = mode_bits[m];
    }
    uint32_t interval = 0;
    if (member(p, "Interval", &v) && !json_u32(v, &interval))
        return &oecm_bad_decoding;
    bool precisions = false;
    if (member(p, "Precisions", &v)) {
        if (oecm_json_kind(v) != OECM_J_OBJECT)
            return &oecm_bad_decoding;
        oecm_jit it;
        oecm_json_iter(v, &it);
        while (oecm_json_next(&it, &k, &x)) {
            double d;
            if (!oecm_json_f64(x, &d))
                return &oecm_bad_decoding;
            precisions |= d != 0.0;
        }
    }
    size_t config = 0; /* absent: NONE_0 */
    if (member(p, "Config", &v) &&
        (config = json_index_of(v, pub_configs, sizeof pub_configs / sizeof pub_configs[0])) ==
            SIZE_MAX)
        return &oecm_bad_decoding;
    /* read-only fields; every entry of the device has Config NONE_0 */
    if (!resource_same || !source_same || !filter_same || (uint64_t)id != e->writer ||
        config != 0)
        return &oecm_bad_not_writable;
    uint8_t current = 0;
    for (size_t m = 0; m < MODES; m++)
        if (cstr_eq(mode_names[m], strlen(mode_names[m]), e->mode))
            current = mode_bits[m];
    if (mode_key(mode) != mode_key(current) || interval != 0 || precisions)
        return &oecm_bad_not_writable;
    return NULL;
}

static const oecm_status *set_publist(req *r) {
    oecm_state *s = r->s;
    size_t n = oecm_publist_count(s), i = 0;
    oecm_pub_entry e;
    for (; i < n; i++) {
        oecm_publist_entry(s, i, &e);
        if (r->t.filter.n != 0 && entry_is(r->t.filter, &e))
            break;
    }
    if (i == n)
        return &oecm_bad_not_found;
    oecm_span dsm, p;
    const oecm_status *err = single(r, &dsm);
    if (err)
        return err;
    if (!oecm_json_get(dsm, "Payload", &p) || oecm_json_kind(p) != OECM_J_OBJECT)
        return &oecm_bad_decoding;
    if ((err = publist_change(r, p, &e)) != NULL)
        return err;
    oecm_answer a;
    memset(&a, 0, sizeof a);
    a.res = OECM_R_PUBLIST;
    a.topic_rest = r->rest;
    a.reply_to = r->id;
    add(&a, i);
    (void)oecm_answer_publish(s, &a);
    return &oecm_good;
}

/* --- Set and Del SubscriptionList (REQ-RES-SUBLIST-010, 011, SF-111) --- */

/* Whether the WriterGroupName of TopicPath p[0..n) is f: the resource level
 * and, unless one of them is a wildcard or the Source level is '#', the
 * Filter levels (oec-core sublist.rs group_of). */
static bool group_is(const char *p, size_t n, oecm_span f) {
    size_t start[7], ends[7], levels = 0, i = 0;
    while (levels < 7) {
        start[levels] = i;
        while (i < n && p[i] != '/')
            i++;
        ends[levels++] = i;
        if (i == n)
            break;
        i++;
    }
    if (levels < 5)
        return false;
    for (size_t k = 0; k < 4; k++)
        if (ends[k] - start[k] == 1 && p[start[k]] == '#')
            return false;
    const char *res = p + start[4];
    size_t rn = ends[4] - start[4];
    if (rn == 0 || (rn == 1 && (res[0] == '+' || res[0] == '#')))
        return false;
    bool wild = levels > 5 && ends[5] - start[5] == 1 && p[start[5]] == '#';
    bool filter = levels > 6;
    if (filter) {
        for (size_t k = start[6]; k <= n; k++) {
            size_t b = k;
            while (k < n && p[k] != '/')
                k++;
            if (k - b == 1 && (p[b] == '+' || p[b] == '#'))
                wild = true;
        }
    }
    if (!filter || wild)
        return f.n == rn && memcmp(f.p, res, rn) == 0;
    size_t fn = n - start[6];
    return f.n == rn + 1 + fn && memcmp(f.p, res, rn) == 0 && f.p[rn] == '/' &&
           memcmp(f.p + rn + 1, p + start[6], fn) == 0;
}

static const oecm_status *sublist(req *r, bool del) {
    oecm_state *s = r->s;
    if (r->t.filter.n == 0)
        return &oecm_bad_not_found;
    oecm_span dsm, p, v;
    const oecm_status *err = single(r, &dsm);
    if (err)
        return err;
    if (!oecm_json_get(dsm, "Payload", &p) || oecm_json_kind(p) != OECM_J_OBJECT ||
        !oecm_json_get(p, "TopicPath", &v) || oecm_json_kind(v) != OECM_J_STRING)
        return &oecm_bad_decoding;
    oecm_span x;
    uint32_t interval;
    if (member(p, "Interval", &x) && !json_u32(x, &interval))
        return &oecm_bad_decoding;
    if (member(p, "Config", &x) && !oecm_json_streq(x, "NONE_0") && !oecm_json_streq(x, "CONF_1"))
        return &oecm_bad_decoding;
    /* the decoded TopicPath in tx (free until the answer) */
    size_t n;
    char *path = (char *)s->tx;
    if (!oecm_json_str(v, path, s->tx_cap, &n) || !group_is(path, n, r->t.filter))
        return &oecm_bad_invalid_argument;
    bool listed = false;
    for (size_t i = 0; i < oecm_sublist_count(s) && !listed; i++) {
        oecm_buf b;
        oecm_buf_init(&b, s->tx + n + 1, s->tx_cap - n - 1);
        oecm_put_sublist_path(&b, s, i);
        listed = !b.full && b.len == n && memcmp(b.p, path, n) == 0;
    }
    /* every subscription of the device is NONE_0 */
    if (del)
        return listed ? &oecm_bad_not_writable : &oecm_bad_not_found;
    return listed ? &oecm_bad_not_writable : &oecm_bad_not_supported;
}

/* --- Set Config (REQ-RES-CONFIG-003, 005, 011, 022, 023, 025) --- */

/* A LocalizedText equal to {en-US, text}. */
static bool lt_eq(oecm_span v, const char *text) {
    oecm_span l, t;
    return oecm_json_kind(v) == OECM_J_OBJECT && oecm_json_get(v, "Locale", &l) &&
           oecm_json_get(v, "Text", &t) && oecm_json_kind(t) == OECM_J_STRING &&
           oecm_json_streq(l, "en-US") && oecm_json_streq(t, text);
}

static bool opt_lt_eq(oecm_span v, const char *text) {
    return text ? lt_eq(v, text) : is_null(v);
}

static bool opt_str_eq(oecm_span v, const char *s) {
    return s ? oecm_json_streq(v, s) : is_null(v);
}

static bool flag_eq(oecm_span v, bool flag) {
    bool b;
    return oecm_json_bool(v, &b) && b == flag;
}

static bool context_same(oecm_span v, const oecm_cfg_context *x) {
    oecm_span n, d;
    if (oecm_json_kind(v) != OECM_J_OBJECT || !oecm_json_get(v, "Name", &n) ||
        !lt_eq(n, x->title))
        return false;
    if (member(v, "Description", &d))
        return x->description && lt_eq(d, x->description);
    return x->description == NULL;
}

static bool number_same(oecm_span validation, const char *key, const char *def) {
    oecm_span v;
    double a, b;
    if (!member(validation, key, &v))
        return def == NULL;
    return oecm_json_f64(v, &a) && def && oecm_parse_f64(def, strlen(def), &b) && a == b;
}

static bool validation_same(oecm_span v, const oecm_cfg_object *o) {
    oecm_span x, k, e;
    if (oecm_json_kind(v) != OECM_J_OBJECT)
        return false;
    uint32_t length;
    if (member(v, "Length", &x)) {
        if (!json_u32(x, &length) || o->length == 0 || length != o->length)
            return false;
    } else if (o->length != 0) {
        return false;
    }
    if (!number_same(v, "Min", o->min) || !number_same(v, "Max", o->max) ||
        member(v, "Pattern", &x))
        return false;
    if (!member(v, "Values", &x))
        return o->values == NULL;
    if (oecm_json_kind(x) != OECM_J_ARRAY || o->values == NULL)
        return false;
    size_t i = 0;
    oecm_jit it;
    oecm_json_iter(x, &it);
    while (oecm_json_next(&it, &k, &e))
        if (oecm_json_kind(e) != OECM_J_STRING || i >= o->value_count ||
            !oecm_json_streq(e, o->values[i++]))
            return false;
    return i == o->value_count;
}

static size_t utf8_chars(const char *p, size_t n) {
    size_t chars = 0;
    for (size_t i = 0; i < n; i++)
        if (((uint8_t)p[i] & 0xc0u) != 0x80u)
            chars++;
    return chars;
}

/* check_value of oec-core config.rs and the buffer of the device; the
 * value is decoded into tx. NULL: valid. */
static const oecm_status *check_value(oecm_state *s, const oecm_cfg_object *o, oecm_span v) {
    char *buf = (char *)s->tx;
    size_t n;
    if (!oecm_json_str(v, buf, s->tx_cap, &n))
        return &oecm_bad_out_of_range;
    if (n == 0)
        return NULL;
    if (o->sensitive && cstr_eq(buf, n, MASK))
        return &oecm_bad_out_of_range;
    if (o->type == OECM_CFG_BOOLEAN && !cstr_eq(buf, n, "true") && !cstr_eq(buf, n, "false"))
        return &oecm_bad_type_mismatch;
    if (o->type == OECM_CFG_NUMBER && !oecm_is_json_number(buf, n))
        return &oecm_bad_type_mismatch;
    if (o->length != 0 && utf8_chars(buf, n) > o->length)
        return &oecm_bad_out_of_range;
    if (o->type == OECM_CFG_NUMBER && (o->min || o->max)) {
        double x, m;
        if (oecm_parse_f64(buf, n, &x) &&
            ((o->min && oecm_parse_f64(o->min, strlen(o->min), &m) && x < m) ||
             (o->max && oecm_parse_f64(o->max, strlen(o->max), &m) && x > m)))
            return &oecm_bad_out_of_range;
    }
    if (o->values) {
        size_t i = 0;
        while (i < o->value_count && !cstr_eq(buf, n, o->values[i]))
            i++;
        if (i == o->value_count)
            return &oecm_bad_out_of_range;
    }
    /* the buffer of the device bounds the value */
    if (n >= o->cap || strlen(buf) != n)
        return &oecm_bad_out_of_range;
    return NULL;
}

/* oec-core config.rs object_change; NULL: valid. */
static const oecm_status *object_check(oecm_state *s, const oecm_cfg_object *o, oecm_span om) {
    oecm_span value, k, j;
    if (!oecm_json_get(om, "Value", &value))
        return &oecm_bad_invalid_argument;
    if (oecm_json_kind(value) != OECM_J_STRING)
        return &oecm_bad_type_mismatch;
    if (oecm_json_get(om, "Type", &j) && !oecm_json_streq(j, cfg_type_name[o->type]))
        return &oecm_bad_type_mismatch;
    oecm_jit it;
    oecm_json_iter(om, &it);
    while (oecm_json_next(&it, &k, &j)) {
        bool same = true;
        if (oecm_json_streq(k, "Name"))
            same = lt_eq(j, o->name);
        else if (oecm_json_streq(k, "Description"))
            same = opt_lt_eq(j, o->description);
        else if (oecm_json_streq(k, "Unit"))
            same = opt_str_eq(j, o->unit);
        else if (oecm_json_streq(k, "DefaultValue"))
            same = opt_str_eq(j, o->default_value) ||
                   (o->sensitive && oecm_json_streq(j, MASK) && o->default_value &&
                    o->default_value[0] != '\0');
        else if (oecm_json_streq(k, "Mandatory"))
            same = flag_eq(j, o->mandatory);
        else if (oecm_json_streq(k, "Sensitive"))
            same = flag_eq(j, o->sensitive);
        else if (oecm_json_streq(k, "Validation"))
            same = validation_same(j, o);
        if (!same)
            return &oecm_bad_not_writable;
    }
    if (o->sensitive && oecm_json_streq(value, MASK))
        return NULL; /* keeps the value (REQ-RES-CONFIG-025) */
    return check_value(s, o, value);
}

static const oecm_cfg_group *find_group(const oecm_cfg_context *x, oecm_span key) {
    for (size_t g = 0; g < x->count; g++)
        if (oecm_json_streq(key, x->groups[g].key))
            return &x->groups[g];
    return NULL;
}

static const oecm_cfg_object *find_object(const oecm_cfg_group *grp, oecm_span key) {
    for (size_t k = 0; k < grp->count; k++)
        if (oecm_json_streq(key, grp->objects[k].key))
            return &grp->objects[k];
    return NULL;
}

static const oecm_status *config_check(oecm_state *s, const oecm_cfg_context *x, oecm_span p) {
    size_t objects = 0;
    oecm_jit it;
    oecm_span gk, gj;
    oecm_json_iter(p, &it);
    while (oecm_json_next(&it, &gk, &gj)) {
        if (oecm_json_streq(gk, "Context")) {
            if (!context_same(gj, x))
                return &oecm_bad_not_writable;
            continue;
        }
        const oecm_cfg_group *grp = find_group(x, gk);
        if (grp == NULL)
            return &oecm_bad_not_found;
        if (oecm_json_kind(gj) != OECM_J_OBJECT)
            return &oecm_bad_decoding;
        oecm_jit git;
        oecm_span ok, oj;
        oecm_json_iter(gj, &git);
        while (oecm_json_next(&git, &ok, &oj)) {
            if (oecm_json_streq(ok, "Name")) {
                if (!lt_eq(oj, grp->name))
                    return &oecm_bad_not_writable;
                continue;
            }
            if (oecm_json_streq(ok, "Description")) {
                if (!opt_lt_eq(oj, grp->description))
                    return &oecm_bad_not_writable;
                continue;
            }
            const oecm_cfg_object *o = find_object(grp, ok);
            if (o == NULL)
                return &oecm_bad_not_found;
            if (oecm_json_kind(oj) != OECM_J_OBJECT)
                return &oecm_bad_decoding;
            objects++;
            const oecm_status *e = object_check(s, o, oj);
            if (e)
                return e;
        }
        for (size_t k = 0; k < grp->count; k++)
            if (grp->objects[k].mandatory && !oecm_json_get(gj, grp->objects[k].key, &oj))
                return &oecm_bad_invalid_argument;
    }
    return objects == 0 ? &oecm_bad_invalid_argument : NULL;
}

/* Writes the checked values (the last of duplicate keys); true if one
 * changed. */
static bool config_write(oecm_state *s, const oecm_cfg_context *x, oecm_span p) {
    bool changed = false;
    for (size_t g = 0; g < x->count; g++) {
        const oecm_cfg_group *grp = &x->groups[g];
        oecm_span gj, oj, v;
        if (!oecm_json_get(p, grp->key, &gj))
            continue;
        for (size_t k = 0; k < grp->count; k++) {
            const oecm_cfg_object *o = &grp->objects[k];
            if (!oecm_json_get(gj, o->key, &oj) || !oecm_json_get(oj, "Value", &v) ||
                (o->sensitive && oecm_json_streq(v, MASK)))
                continue;
            size_t n;
            char *buf = (char *)s->tx;
            if (!oecm_json_str(v, buf, s->tx_cap, &n) || n >= o->cap)
                continue; /* checked before */
            if (strlen(o->value) == n && memcmp(o->value, buf, n) == 0)
                continue;
            memcpy(o->value, buf, n + 1);
            changed = true;
        }
    }
    return changed;
}

static const oecm_status *set_config(req *r) {
    oecm_state *s = r->s;
    const oecm_config *c = s->cfg;
    size_t i = r->t.filter.n != 0 ? find_context(c, r->t.filter) : SIZE_MAX;
    if (i == SIZE_MAX)
        return &oecm_bad_not_found;
    const oecm_cfg_context *x = &c->config[i];
    oecm_span payload;
    const oecm_status *e = payload_object(r, &payload);
    if (e)
        return e;
    if ((e = config_check(s, x, payload)) != NULL)
        return e;
    if (config_write(s, x, payload)) {
        s->config_ms[i] = oecm_wall(s);
        /* the answer does not replace the publication on change
         * (REQ-RES-CONFIG-022 notes, REQ-MQTT-019) */
        s->config_due = (uint8_t)(s->config_due | (1u << i));
        if (c->on_config)
            c->on_config(c->app_ctx, i);
    }
    (void)oecm_publish_config(s, i, r->id);
    return &oecm_good;
}

static void write_req(req *r, bool del) {
    if (r->t.source.n == 0)
        return;
    if (!own_source(r)) {
        status(r, &oecm_bad_not_found); /* under the AppId (REQ-CMP-047) */
        return;
    }
    oecm_span res = r->t.resource;
    const oecm_status *st = &oecm_bad_not_found;
    if (oecm_span_eq(res, "SubscriptionList"))
        st = sublist(r, del);
    else if (del)
        st = &oecm_bad_not_found;
    else if (oecm_span_eq(res, "Data"))
        st = set_data(r);
    else if (oecm_span_eq(res, "PublicationList"))
        st = set_publist(r);
    else if (oecm_span_eq(res, "Config"))
        st = set_config(r);
    else if (oecm_span_eq(res, "Metadata"))
        st = r->t.filter.n != 0 && find_data(r->s->cfg, r->t.filter) != SIZE_MAX
                 ? &oecm_bad_not_writable
                 : &oecm_bad_not_found;
    status(r, st);
}

/* --- Dispatch ------------------------------------------------------------------------ */

void oecm_l2_message(oecm_state *s, const char *topic, const uint8_t *payload, size_t len) {
    req r;
    memset(&r, 0, sizeof r);
    r.s = s;
    size_t tn = strlen(topic);
    if (!oecm_topic_parse(topic, tn, &r.t) ||
        !oecm_span_eq(r.t.st, oecm_service_type_name(s->cfg->service_type)) ||
        !oecm_b64url_eq(r.t.app, s->il, s->il_len))
        return;
    bool get_ = oecm_span_eq(r.t.method, "Get"), del = oecm_span_eq(r.t.method, "Del");
    if (!get_ && !del && !oecm_span_eq(r.t.method, "Set"))
        return;
    if (index_of(r.t.resource, resources, RESOURCES) == SIZE_MAX)
        return;
    if (r.t.source.n != 0 && !own_source(&r)) {
        char il[OECM_IL_MAX + 1];
        size_t n;
        if (oecm_il_decode(r.t.source.p, r.t.source.n, il, sizeof il, &n) != 0)
            return; /* no valid topic (REQ-TOP-*) */
    }
    if ((oecm_span_eq(r.t.resource, "PublicationList") ||
         oecm_span_eq(r.t.resource, "SubscriptionList")) &&
        r.t.filter.n != 0) {
        oecm_span head, tag;
        split_level(r.t.filter, &head, &tag);
        if (index_of(head, resources, RESOURCES) == SIZE_MAX)
            return; /* no ListFilter */
    }
    if (payload == NULL ||
        !oecm_json_parse((const char *)payload, len, &r.root) ||
        oecm_json_kind(r.root) != OECM_J_OBJECT || !oecm_json_get(r.root, "MessageId", &r.id) ||
        !message_id_ok(r.id))
        return;
    r.rest = (oecm_span){r.t.resource.p, (size_t)(topic + tn - r.t.resource.p)};
    if (get_)
        get(&r);
    else
        write_req(&r, del);
}
