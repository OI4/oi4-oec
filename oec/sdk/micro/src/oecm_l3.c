/*
 * Level 3 (ADR-0030, C-095): the device's ReferenceDesignation (MG 8.3.10,
 * MG 9.1.10; Get with Query, Set, Del, persisted) and the common service
 * NewDataSetWriterId (MG 8.4.1, MG 9.2.1), as the Rust provider decides
 * them (oec-core provider/refdes.rs, provider/action.rs,
 * message/action.rs, resource/reference_designation.rs).
 */
#include <float.h>
#include <string.h>

#include "oecm_internal.h"

#define KEY_REFDES "oecm.refdes"
#define KEY_WID "oecm.wid"
#define WID_BLOCK 256u      /* ids reserved per persisted limit */
#define WID_WINDOW_MS 60000u /* rate limit: WID_PER_WINDOW per window */
#define WID_PER_WINDOW 256u
#define WID_MAX 65535u

static const char *const aspects[] = {"Function", "Product", "Location"};
static const char *const resources[] = {"MAM",     "Health",          "Config",
                                        "Data",    "Metadata",        "Event",
                                        "Profile", "PublicationList", "SubscriptionList",
                                        "ReferenceDesignation"};

static bool is_null(oecm_span v) { return oecm_json_kind(v) == OECM_J_NULL; }

/* A present member that is not null (serde: Option, null = absent). */
static bool member(oecm_span obj, const char *key, oecm_span *v) {
    return oecm_json_get(obj, key, v) && !is_null(*v);
}

/* --- DateTime (oec-core message/value.rs) ------------------------------------ */

static bool digits(const char *s, size_t n) {
    for (size_t i = 0; i < n; i++)
        if (s[i] < '0' || s[i] > '9')
            return false;
    return true;
}

static unsigned num(const char *s, size_t n) {
    unsigned v = 0;
    for (size_t i = 0; i < n; i++)
        v = v * 10u + (unsigned)(s[i] - '0');
    return v;
}

bool oecm_date_time_ok(const char *s, size_t n) {
    /* the schema form: upper-case T and Z, no leap second */
    if (n < 20 || s[10] != 'T' || memcmp(s + 17, "60", 2) >= 0)
        return false;
    for (size_t i = 0; i < n; i++)
        if (s[i] == 't' || s[i] == 'z')
            return false;
    /* RFC 3339 */
    if (!digits(s, 4) || s[4] != '-' || !digits(s + 5, 2) || s[7] != '-' || !digits(s + 8, 2) ||
        !digits(s + 11, 2) || s[13] != ':' || !digits(s + 14, 2) || s[16] != ':' ||
        !digits(s + 17, 2))
        return false;
    static const uint8_t days[] = {31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31};
    unsigned y = num(s, 4), mo = num(s + 5, 2), d = num(s + 8, 2);
    if (mo < 1 || mo > 12 || d < 1)
        return false;
    bool leap = y % 4u == 0 && (y % 100u != 0 || y % 400u == 0);
    if (d > days[mo - 1] + (mo == 2 && leap ? 1u : 0u))
        return false;
    if (num(s + 11, 2) > 23 || num(s + 14, 2) > 59)
        return false;
    size_t i = 19;
    if (s[i] == '.') {
        size_t from = ++i;
        while (i < n && s[i] >= '0' && s[i] <= '9')
            i++;
        if (i == from)
            return false;
    }
    if (i + 1 == n && s[i] == 'Z')
        return true;
    return i + 6 == n && (s[i] == '+' || s[i] == '-') && digits(s + i + 1, 2) &&
           s[i + 3] == ':' && digits(s + i + 4, 2) && num(s + i + 1, 2) <= 23 &&
           num(s + i + 4, 2) <= 59;
}

/* --- ReferenceDesignation object (MG 8.3.10) --------------------------------- */

/* A string member value: 0 written (or valid), 1 invalid, 2 no room. o NULL:
 * validation only. */
static int put_string(oecm_buf *o, oecm_span v, char *sc, size_t cap) {
    if (oecm_json_kind(v) != OECM_J_STRING)
        return 1;
    if (o == NULL)
        return 0;
    size_t n;
    if (!oecm_json_str(v, sc, cap, &n))
        return 2;
    oecm_put_json_str(o, sc, n);
    return o->full ? 2 : 0;
}

static int put_optional(oecm_buf *o, oecm_span obj, const char *key, char *sc, size_t cap) {
    oecm_span v;
    if (!member(obj, key, &v))
        return 0;
    if (o) {
        oecm_puts(o, ",\"");
        oecm_puts(o, key);
        oecm_puts(o, "\":");
    }
    return put_string(o, v, sc, cap);
}

/* Designation {Value, Local?, Parent?} or its Parent {Value, Local?,
 * IdentificationLink?}, in the field order of the Rust types. */
static int put_designation(oecm_buf *o, oecm_span d, bool parent, char *sc, size_t cap) {
    oecm_span v;
    int e;
    if (oecm_json_kind(d) != OECM_J_OBJECT || !oecm_json_get(d, "Value", &v))
        return 1;
    if (o)
        oecm_puts(o, "{\"Value\":");
    if ((e = put_string(o, v, sc, cap)) != 0 || (e = put_optional(o, d, "Local", sc, cap)) != 0)
        return e;
    if (parent) {
        if ((e = put_optional(o, d, "IdentificationLink", sc, cap)) != 0)
            return e;
    } else if (member(d, "Parent", &v)) {
        if (o)
            oecm_puts(o, ",\"Parent\":");
        if ((e = put_designation(o, v, true, sc, cap)) != 0)
            return e;
    }
    if (o)
        oecm_putc(o, '}');
    return o && o->full ? 2 : 0;
}

static int put_rd(oecm_buf *o, oecm_span obj, char *sc, size_t cap) {
    if (oecm_json_kind(obj) != OECM_J_OBJECT)
        return 1;
    if (o)
        oecm_putc(o, '{');
    bool first = true;
    for (size_t i = 0; i < 3; i++) {
        oecm_span d;
        if (!member(obj, aspects[i], &d))
            continue;
        if (o) {
            oecm_puts(o, first ? "\"" : ",\"");
            oecm_puts(o, aspects[i]);
            oecm_puts(o, "\":");
        }
        first = false;
        int e = put_designation(o, d, false, sc, cap);
        if (e != 0)
            return e;
    }
    if (o)
        oecm_putc(o, '}');
    return o && o->full ? 2 : 0;
}

int oecm_refdes_encode(oecm_span obj, oecm_buf *out, char *scratch, size_t scratch_cap) {
    /* a decoding error before a size error, as the Rust provider has no
     * size limit */
    int e = put_rd(NULL, obj, NULL, 0);
    return e != 0 ? e : put_rd(out, obj, scratch, scratch_cap);
}

/* Whether refdes_buf[0..n) is a canonical object that fits. */
static bool stored_ok(oecm_state *s, size_t n) {
    oecm_span root;
    if (n == 0 || n > s->refdes_max || !oecm_json_parse(s->cfg->refdes_buf, n, &root))
        return false;
    size_t half = s->tx_cap / 2u;
    oecm_buf o;
    oecm_buf_init(&o, s->tx, s->refdes_max);
    return oecm_refdes_encode(root, &o, (char *)s->tx + half, s->tx_cap - half) == 0 &&
           o.len == n && memcmp(s->tx, s->cfg->refdes_buf, n) == 0;
}

/* --- Init ------------------------------------------------------------------------ */

static uint32_t wid_first(const oecm_state *s) { return oecm_refdes_writer(s) + 1u; }

oecm_err oecm_l3_init(oecm_state *s) {
    const oecm_config *c = s->cfg;
    const oecm_pal *p = s->pal;
    size_t max = c->refdes_cap < s->tx_cap / 2u ? c->refdes_cap : s->tx_cap / 2u;
    size_t room = oecm_refdes_room(s);
    s->refdes_max = max < room ? max : room;
    if (s->refdes_max < OECM_REFDES_MIN)
        return OECM_E_NOSPACE;
    uint8_t v[4];
    size_t n = 0;
    if (p->persist_load(p->ctx, KEY_WID, v, sizeof v, &n) != OECM_OK)
        return OECM_E_PAL;
    uint32_t limit = n == sizeof v ? (uint32_t)v[0] | (uint32_t)v[1] << 8 |
                                         (uint32_t)v[2] << 16 | (uint32_t)v[3] << 24
                                   : 0u;
    if (limit < wid_first(s) || limit > WID_MAX + 1u)
        limit = wid_first(s);
    s->wid_next = s->wid_limit = limit;
    n = 0;
    if (p->persist_load(p->ctx, KEY_REFDES, (uint8_t *)c->refdes_buf, c->refdes_cap, &n) !=
        OECM_OK)
        return OECM_E_PAL;
    s->refdes_len = n <= c->refdes_cap && stored_ok(s, n) ? n : 0u;
    return OECM_OK;
}

/* --- Get, Set, Del (MG 9.1.10) --------------------------------------------------- */

int oecm_l3_refdes_get(oecm_state *s, oecm_span root, bool with_source) {
    oecm_span q = {NULL, 0};
    /* the Query: the Payload of the first DataSetMessage with
     * DataSetWriterId 9 (REQ-QRY-001), only without Source
     * (REQ-RES-REFDES-013) */
    oecm_span msgs, k, m;
    if (!with_source && oecm_json_get(root, "Messages", &msgs) &&
        oecm_json_kind(msgs) == OECM_J_ARRAY) {
        oecm_jit it;
        oecm_json_iter(msgs, &it);
        while (oecm_json_next(&it, &k, &m)) {
            oecm_span w, p;
            int64_t id;
            if (oecm_json_kind(m) != OECM_J_OBJECT || !oecm_json_get(m, "DataSetWriterId", &w) ||
                !oecm_json_i64(w, &id) || id != 9)
                continue;
            if (oecm_json_get(m, "Payload", &p) && oecm_json_kind(p) == OECM_J_OBJECT)
                q = p;
            break;
        }
    }
    if (q.n == 0)
        return s->refdes_len != 0;
    oecm_span parent, aspect;
    size_t from = 0, to = 3;
    if (!oecm_json_get(q, "Parent", &parent) || oecm_json_kind(parent) != OECM_J_STRING)
        return -1;
    if (member(q, "Aspect", &aspect)) {
        size_t i = 0;
        while (i < 3 && !oecm_json_streq(aspect, aspects[i]))
            i++;
        if (i == 3)
            return -1;
        from = i;
        to = i + 1;
    }
    size_t n;
    if (s->refdes_len == 0 || !oecm_json_str(parent, (char *)s->tx, s->tx_cap, &n))
        return 0; /* nothing stored, or longer than any stored link */
    oecm_span rd, d, p, il;
    if (!oecm_json_parse(s->cfg->refdes_buf, s->refdes_len, &rd))
        return 0;
    for (size_t i = from; i < to; i++)
        if (member(rd, aspects[i], &d) && member(d, "Parent", &p) &&
            member(p, "IdentificationLink", &il) && oecm_json_str_eq(il, (char *)s->tx, n))
            return 1;
    return 0;
}

const oecm_status *oecm_l3_refdes_set(oecm_state *s, oecm_span payload, oecm_span reply_to) {
    const oecm_config *c = s->cfg;
    size_t half = s->tx_cap / 2u;
    oecm_buf o;
    oecm_buf_init(&o, s->tx, s->refdes_max);
    int e = oecm_refdes_encode(payload, &o, (char *)s->tx + half, s->tx_cap - half);
    if (e == 1)
        return &oecm_bad_decoding;
    if (e != 0)
        return &oecm_bad_out_of_range;
    if (o.len != s->refdes_len || memcmp(c->refdes_buf, s->tx, o.len) != 0) {
        /* persisted before it is used (REQ-RES-REFDES-002) */
        if (s->pal->persist_save(s->pal->ctx, KEY_REFDES, s->tx, o.len) != OECM_OK)
            return &oecm_bad_internal_error;
        memcpy(c->refdes_buf, s->tx, o.len);
        s->refdes_len = o.len;
        /* the answer does not replace the publication on change
         * (REQ-MQTT-019) */
        s->refdes_due = true;
    }
    oecm_answer a = {.res = OECM_R_REFDES,
                     .topic_res = "ReferenceDesignation",
                     .topic_source = true,
                     .reply_to = reply_to,
                     .n = 1};
    (void)oecm_answer_publish(s, &a);
    return &oecm_good;
}

const oecm_status *oecm_l3_refdes_del(oecm_state *s) {
    if (s->refdes_len == 0)
        return &oecm_bad_not_found;
    if (s->pal->persist_save(s->pal->ctx, KEY_REFDES, (const uint8_t *)s->cfg->refdes_buf, 0) !=
        OECM_OK)
        return &oecm_bad_internal_error;
    s->refdes_len = 0;
    s->refdes_due = false;
    return &oecm_good;
}

/* --- NewDataSetWriterId (MG 9.2.1) ------------------------------------------------ */

/* The members with the key; *v: the last. */
static size_t field(oecm_span obj, const char *key, oecm_span *v) {
    size_t n = 0;
    oecm_jit it;
    oecm_span k, x;
    oecm_json_iter(obj, &it);
    while (oecm_json_next(&it, &k, &x))
        if (oecm_json_streq(k, key)) {
            *v = x;
            n++;
        }
    return n;
}

/* An optional field of a Rust struct: a duplicate is an error (serde),
 * null is absent. */
static bool opt_field(oecm_span obj, const char *key, oecm_span *v, bool *present) {
    size_t n = field(obj, key, v);
    *present = n == 1 && !is_null(*v);
    return n <= 1;
}

static bool u_field(oecm_span obj, const char *key, uint32_t max, uint32_t *out) {
    oecm_span v;
    int64_t x;
    if (field(obj, key, &v) != 1 || !oecm_json_i64(v, &x) || x < 0 || x > (int64_t)max)
        return false;
    *out = (uint32_t)x;
    return true;
}

static bool time_field(oecm_span v) {
    char t[64];
    size_t n;
    return oecm_json_kind(v) == OECM_J_STRING && oecm_json_str(v, t, sizeof t, &n) &&
           oecm_date_time_ok(t, n);
}

/* PublisherId `<ServiceType>/<IL>` (REQ-MSG-009) into out. */
static bool publisher_id(oecm_span v, char *out, size_t cap, size_t *n) {
    if (oecm_json_kind(v) != OECM_J_STRING || !oecm_json_str(v, out, cap, n))
        return false;
    size_t i = 0;
    while (i < *n && out[i] != '/')
        i++;
    if (i == *n)
        return false;
    oecm_span st = {out, i};
    bool known = false;
    for (int k = OECM_ST_REGISTRY; k <= OECM_ST_IT_CONNECTOR; k++)
        known = known || oecm_span_eq(st, oecm_service_type_name((oecm_service_type)k));
    return known && oecm_il_check(out + i + 1, *n - i - 1) == 0;
}

/* ByteString: base64 with padding (oec-core message/value.rs). */
static bool base64_ok(const char *s, size_t n) {
    size_t body = n;
    while (body > 0 && s[body - 1] == '=')
        body--;
    size_t pad = n - body;
    if (n % 4u != 0 || pad > 2 || (pad != 0 && body % 4u != 4u - pad))
        return false;
    for (size_t i = 0; i < body; i++) {
        char c = s[i];
        if (!((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
              c == '+' || c == '/'))
            return false;
    }
    return true;
}

/* ActionRequest (REQ-ACT-010); *ours: DataSetWriterId 8, ActionTargetId 1. */
static bool action_request_ok(oecm_span m, bool *ours) {
    uint32_t dsw, target, id, x;
    oecm_span v;
    bool present;
    if (oecm_json_kind(m) != OECM_J_OBJECT || !u_field(m, "DataSetWriterId", 0xFFFFu, &dsw) ||
        !u_field(m, "ActionTargetId", 0xFFFFu, &target) || !u_field(m, "RequestId", 0xFFFFu, &id))
        return false;
    if (!opt_field(m, "MetaDataVersion", &v, &present))
        return false;
    if (present && (oecm_json_kind(v) != OECM_J_OBJECT ||
                    !u_field(v, "MajorVersion", UINT32_MAX, &x) ||
                    !u_field(v, "MinorVersion", UINT32_MAX, &x)))
        return false;
    if (!opt_field(m, "Timestamp", &v, &present) || (present && !time_field(v)))
        return false;
    if (field(m, "Payload", &v) != 1)
        return false;
    /* the ActionState of a request is 1 */
    if (!u_field(m, "ActionState", 1u, &x) || x != 1u)
        return false;
    *ours = dsw == 8u && target == 1u;
    return true;
}

typedef struct call {
    char requestor[OECM_IL_MAX + 16];
    size_t requestor_len;
    oecm_span correlation; /* n == 0: none */
    oecm_span messages;
} call;

/* Decodes the ua-action-request as the Rust type does and checks its
 * address (REQ-ACT-005, REQ-ACT-006); false: no response (REQ-ACT-017). */
static bool call_ok(oecm_state *s, const char *topic, const oecm_topic *t, oecm_span root,
                    call *c) {
    oecm_span v;
    bool present, ours = false;
    char *scratch = (char *)s->tx;
    size_t n;
    if (field(root, "MessageId", &v) != 1 || !oecm_message_id_ok(v))
        return false;
    if (field(root, "MessageType", &v) != 1 || !oecm_json_streq(v, "ua-action-request"))
        return false;
    /* the PublisherId is this application */
    if (field(root, "PublisherId", &v) != 1 || !publisher_id(v, scratch, s->tx_cap, &n))
        return false;
    const char *st = oecm_service_type_name(s->cfg->service_type);
    size_t stn = strlen(st);
    if (n != stn + 1u + s->il_len || memcmp(scratch, st, stn) != 0 ||
        memcmp(scratch + stn + 1u, s->il, s->il_len) != 0)
        return false;
    if (field(root, "Timestamp", &v) != 1 || !time_field(v))
        return false;
    if (field(root, "RequestorId", &v) != 1 ||
        !publisher_id(v, c->requestor, sizeof c->requestor, &c->requestor_len))
        return false;
    c->correlation = (oecm_span){NULL, 0};
    if (!opt_field(root, "CorrelationData", &v, &present))
        return false;
    if (present) {
        if (oecm_json_kind(v) != OECM_J_STRING || !oecm_json_str(v, scratch, s->tx_cap, &n) ||
            !base64_ok(scratch, n))
            return false;
        c->correlation = v;
    }
    /* the ResponseAddress is the Reply topic of the Call topic */
    if (field(root, "ResponseAddress", &v) != 1 || oecm_json_kind(v) != OECM_J_STRING ||
        !oecm_json_str(v, scratch, s->tx_cap, &n))
        return false;
    size_t head = (size_t)(t->method.p - topic), tail = strlen(t->resource.p);
    if (n != head + 6u + tail || memcmp(scratch, topic, head) != 0 ||
        memcmp(scratch + head, "Reply/", 6) != 0 || memcmp(scratch + head + 6, t->resource.p, tail) != 0)
        return false;
    double hint;
    if (field(root, "TimeoutHint", &v) != 1 || !oecm_json_f64(v, &hint) || !(hint > 0.0) ||
        hint > DBL_MAX)
        return false;
    if (field(root, "Messages", &v) != 1 || oecm_json_kind(v) != OECM_J_ARRAY)
        return false;
    c->messages = v;
    oecm_jit it;
    oecm_span k, m;
    size_t count = 0;
    bool any = false;
    oecm_json_iter(v, &it);
    while (oecm_json_next(&it, &k, &m)) {
        if (!action_request_ok(m, &ours))
            return false;
        any = any || ours;
        count++;
    }
    return count != 0 && any;
}

/* A new DataSetWriterId: not used before, also across restarts (REQ-ACT-023):
 * the persisted limit is raised by WID_BLOCK before an id at or above it
 * is given. */
static const oecm_status *reserve(oecm_state *s, oecm_span payload, uint32_t *id) {
    oecm_span r;
    bool known = false;
    if (oecm_json_kind(payload) != OECM_J_OBJECT || !oecm_json_get(payload, "Resource", &r))
        return &oecm_bad_invalid_argument;
    for (size_t i = 0; i < sizeof resources / sizeof resources[0]; i++)
        known = known || oecm_json_streq(r, resources[i]);
    if (!known)
        return &oecm_bad_invalid_argument;
    uint32_t now = oecm_mono(s);
    if (s->wid_count == 0 || now - s->wid_window_ms >= WID_WINDOW_MS) {
        s->wid_window_ms = now;
        s->wid_count = 0;
    }
    if (s->wid_count >= WID_PER_WINDOW)
        return &oecm_bad_resource_unavailable;
    if (s->wid_next > WID_MAX)
        s->wid_next = s->wid_limit = wid_first(s); /* after 65535: from the first again */
    if (s->wid_next >= s->wid_limit) {
        uint32_t limit = s->wid_next + WID_BLOCK;
        if (limit > WID_MAX + 1u)
            limit = WID_MAX + 1u;
        uint8_t v[4] = {(uint8_t)limit, (uint8_t)(limit >> 8), (uint8_t)(limit >> 16),
                        (uint8_t)(limit >> 24)};
        if (s->pal->persist_save(s->pal->ctx, KEY_WID, v, sizeof v) != OECM_OK)
            return &oecm_bad_internal_error;
        s->wid_limit = limit;
    }
    *id = s->wid_next++;
    s->wid_count++;
    return NULL;
}

static void put_header(oecm_buf *b, oecm_state *s, const call *c, int64_t now) {
    oecm_puts(b, "{\"MessageId\":\"");
    oecm_put_id(b, s, s->id_ms, s->id_counter);
    oecm_puts(b, "\",\"MessageType\":\"ua-action-response\",\"PublisherId\":\"");
    oecm_put_publisher(b, s);
    oecm_puts(b, "\",\"Timestamp\":\"");
    oecm_put_time(b, now);
    oecm_puts(b, "\",\"RequestorId\":");
    oecm_put_json_str(b, c->requestor, c->requestor_len);
    if (c->correlation.n != 0) {
        /* base64 needs no escape: the decoded text is the token content */
        size_t n;
        oecm_puts(b, ",\"CorrelationData\":\"");
        if (b->full || !oecm_json_str(c->correlation, (char *)b->p + b->len, b->cap - b->len, &n))
            b->full = true;
        else
            b->len += n;
        oecm_putc(b, '"');
    }
    oecm_puts(b, ",\"Messages\":[");
}

static void put_response(oecm_buf *b, uint32_t request_id, const oecm_status *st, uint32_t id) {
    oecm_puts(b, "{\"DataSetWriterId\":8,\"ActionTargetId\":1,\"RequestId\":");
    oecm_put_u64(b, request_id);
    if (st) {
        oecm_puts(b, ",\"Status\":{\"Code\":");
        oecm_put_u64(b, st->number);
        oecm_puts(b, "},\"ActionState\":2,\"Payload\":{}}");
    } else {
        oecm_puts(b, ",\"Status\":{},\"ActionState\":2,\"Payload\":{\"DataSetWriterId\":");
        oecm_put_u64(b, id);
        oecm_puts(b, ",\"Ttl\":60}}");
    }
}

void oecm_l3_call(oecm_state *s, const char *topic, const oecm_topic *t, const uint8_t *payload,
                  size_t len) {
    oecm_span root;
    call c;
    if (payload == NULL || !oecm_json_parse((const char *)payload, len, &root) ||
        oecm_json_kind(root) != OECM_J_OBJECT || !call_ok(s, topic, t, root, &c))
        return;
    /* the Source of the topic, else the AppId; only the own is known */
    bool known = t->source.n == 0 || oecm_b64url_eq(t->source, s->il, s->il_len);
    oecm_buf b;
    oecm_buf_init(&b, s->tx, s->tx_cap);
    oecm_put(&b, topic, (size_t)(t->method.p - topic));
    oecm_puts(&b, "Reply/");
    oecm_puts(&b, t->resource.p);
    oecm_putc(&b, '\0');
    if (b.full)
        return;
    size_t at = b.len, limit = oecm_payload_limit(s, at - 1u);
    int64_t now = oecm_wall(s);
    size_t count = 0;
    bool open = false;
    oecm_jit it;
    oecm_span k, m;
    oecm_json_iter(c.messages, &it);
    while (oecm_json_next(&it, &k, &m)) {
        uint32_t dsw = 0, target = 0, request_id = 0, id = 0;
        oecm_span p;
        (void)u_field(m, "DataSetWriterId", 0xFFFFu, &dsw);
        (void)u_field(m, "ActionTargetId", 0xFFFFu, &target);
        (void)u_field(m, "RequestId", 0xFFFFu, &request_id);
        if (dsw != 8u || target != 1u)
            continue;
        (void)field(m, "Payload", &p);
        const oecm_status *st = known ? reserve(s, p, &id) : &oecm_bad_not_found;
        /* as many responses per message as fit (REQ-ACT-015) */
        for (int attempt = 0; attempt < 2; attempt++) {
            if (!open) {
                oecm_next_message_id(s, now);
                oecm_buf_init(&b, s->tx, s->tx_cap);
                b.len = at;
                put_header(&b, s, &c, now);
                open = true;
                count = 0;
            }
            size_t mark = b.len;
            if (count != 0)
                oecm_putc(&b, ',');
            put_response(&b, request_id, st, id);
            if (!b.full && b.len + 2u <= b.cap && b.len + 2u - at <= limit) {
                count++;
                break;
            }
            b.len = mark;
            b.full = false;
            if (count == 0)
                return; /* one response does not fit */
            oecm_puts(&b, "]}");
            if (oecm_publish(s, at, b.len - at) != OECM_OK)
                return;
            open = false;
        }
    }
    if (!open)
        return;
    oecm_puts(&b, "]}");
    (void)oecm_publish(s, at, b.len - at);
}
