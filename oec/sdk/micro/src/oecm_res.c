/*
 * Level 2 resources (MG 8.2.3, MG 8.3, MG 9.1): the DataSetMessages of
 * MAM, Health, Profile, PublicationList, SubscriptionList, Config, Data and
 * Metadata, their pagination and the publications after a connect, on
 * change and per Interval. Same wire form as the Rust provider (oec-core
 * provider and its modules).
 */
#include <string.h>

#include "oecm_internal.h"

const oecm_status oecm_good = {0u, "Good", "The operation succeeded."};
const oecm_status oecm_bad_decoding = {
    0x80070000u, "BadDecodingError", "Decoding halted because of invalid data in the stream."};
const oecm_status oecm_bad_not_writable = {
    0x803B0000u, "BadNotWritable", "The access level does not allow writing to the Node."};
const oecm_status oecm_bad_out_of_range = {0x803C0000u, "BadOutOfRange",
                                           "The value was out of range."};
const oecm_status oecm_bad_not_supported = {0x803D0000u, "BadNotSupported",
                                            "The requested operation is not supported."};
const oecm_status oecm_bad_not_found = {
    0x803E0000u, "BadNotFound",
    "A requested item was not found or a search operation ended without success."};
const oecm_status oecm_bad_type_mismatch = {
    0x80740000u, "BadTypeMismatch",
    "The value supplied for the attribute is not of the same type as the attribute's value."};
const oecm_status oecm_bad_invalid_argument = {0x80AB0000u, "BadInvalidArgument",
                                               "One or more arguments are invalid."};

/* Namespace of the publisher-defined DataSetClassIds (oec-core data.rs). */
static const uint8_t class_ns[16] = {0xe7, 0x95, 0x41, 0x13, 0xfb, 0x31, 0x53, 0x34,
                                     0x8b, 0x17, 0x76, 0x07, 0x5c, 0x4e, 0xb3, 0x11};

static const char *const fixed_class[] = {OECM_CLASS_MAM,     OECM_CLASS_HEALTH,
                                          OECM_CLASS_PROFILE, OECM_CLASS_PUBLIST,
                                          OECM_CLASS_SUBLIST, OECM_CLASS_CONFIG};

static const unsigned builtin[] = {1u, 6u, 7u, 10u, 11u, 12u}; /* by oecm_type */
static const char *const cfg_type_name[] = {"Boolean", "Number", "String"};

#define NO_REPLY ((oecm_span){NULL, 0})
#define MASK "********"

unsigned oecm_config_writer(const oecm_state *s, size_t ctx) {
    (void)s;
    return OECM_WRITER_FIRST + (unsigned)ctx;
}

unsigned oecm_data_writer(const oecm_state *s, size_t data) {
    return OECM_WRITER_FIRST + (unsigned)s->cfg->config_count + (unsigned)data;
}

static uint32_t data_version(const oecm_state *s, size_t j) {
    const oecm_data *d = &s->cfg->data[j];
    if (d->version != 0)
        return d->version;
    const uint8_t *c = s->class_id[j];
    uint32_t v = (uint32_t)c[0] << 24 | (uint32_t)c[1] << 16 | (uint32_t)c[2] << 8 | c[3];
    return v != 0 ? v : 1u;
}

/* --- Lists ------------------------------------------------------------------- */

size_t oecm_publist_count(const oecm_state *s) {
    return 6u + s->cfg->config_count + 2u * s->cfg->data_count;
}

void oecm_publist_entry(const oecm_state *s, size_t i, oecm_pub_entry *e) {
    static const char *const fixed[] = {"MAM", "Health", "Profile"};
    static const char *const status[] = {"Status/Good", "Status/Uncertain", "Status/Bad"};
    size_t nc = s->cfg->config_count;
    if (i < 3) {
        *e = (oecm_pub_entry){fixed[i], NULL, OECM_WRITER_MAM + (unsigned)i,
                              "ON_REQUEST_SOURCE_10"};
    } else if (i < 6) {
        *e = (oecm_pub_entry){"Event", status[i - 3], OECM_WRITER_STATUS + (unsigned)(i - 3),
                              "FILTER_4"};
    } else if (i < 6 + nc) {
        *e = (oecm_pub_entry){"Config", s->cfg->config[i - 6].name,
                              oecm_config_writer(s, i - 6), "ON_REQUEST_FILTER_9"};
    } else {
        size_t j = (i - 6 - nc) / 2;
        *e = (oecm_pub_entry){(i - 6 - nc) % 2 == 0 ? "Data" : "Metadata", s->cfg->data[j].tag,
                              oecm_data_writer(s, j), "ON_REQUEST_FILTER_9"};
    }
}

size_t oecm_sublist_count(const oecm_state *s) {
    size_t n = s->cfg->config_count;
    for (size_t j = 0; j < s->cfg->data_count; j++)
        if (s->cfg->data[j].writable)
            n++;
    return n;
}

void oecm_sublist_entry(const oecm_state *s, size_t i, const char **resource, const char **tag) {
    if (i < s->cfg->config_count) {
        *resource = "Config";
        *tag = s->cfg->config[i].name;
        return;
    }
    i -= s->cfg->config_count;
    for (size_t j = 0; j < s->cfg->data_count; j++) {
        if (!s->cfg->data[j].writable)
            continue;
        if (i-- == 0) {
            *resource = "Data";
            *tag = s->cfg->data[j].tag;
            return;
        }
    }
    *resource = *tag = "";
}

void oecm_put_sublist_path(oecm_buf *b, const oecm_state *s, size_t i) {
    const char *res, *tag;
    oecm_sublist_entry(s, i, &res, &tag);
    oecm_puts(b, "Oi4v2/");
    oecm_puts(b, oecm_service_type_name(s->cfg->service_type));
    oecm_putc(b, '/');
    oecm_put_b64url(b, (const uint8_t *)s->il, s->il_len);
    oecm_puts(b, "/Set/");
    oecm_puts(b, res);
    oecm_putc(b, '/');
    oecm_put_b64url(b, (const uint8_t *)s->il, s->il_len);
    oecm_putc(b, '/');
    oecm_puts(b, tag);
}

/* --- Payloads ------------------------------------------------------------------ */

static void put_cfg_value(oecm_buf *b, const oecm_cfg_object *o, const char *v) {
    if (o->sensitive && v[0] != '\0')
        oecm_puts(b, "\"" MASK "\"");
    else
        oecm_put_json_cstr(b, v);
}

static void put_key(oecm_buf *b, const char *key) {
    oecm_putc(b, ',');
    oecm_put_json_cstr(b, key);
    oecm_putc(b, ':');
}

static void put_config(oecm_buf *b, const oecm_cfg_context *c) {
    oecm_puts(b, "{\"Context\":{\"Name\":");
    oecm_put_localized(b, c->title);
    if (c->description) {
        oecm_puts(b, ",\"Description\":");
        oecm_put_localized(b, c->description);
    }
    oecm_putc(b, '}');
    for (size_t g = 0; g < c->count; g++) {
        const oecm_cfg_group *grp = &c->groups[g];
        put_key(b, grp->key);
        oecm_puts(b, "{\"Name\":");
        oecm_put_localized(b, grp->name);
        if (grp->description) {
            oecm_puts(b, ",\"Description\":");
            oecm_put_localized(b, grp->description);
        }
        for (size_t k = 0; k < grp->count; k++) {
            const oecm_cfg_object *o = &grp->objects[k];
            put_key(b, o->key);
            oecm_puts(b, "{\"Type\":\"");
            oecm_puts(b, cfg_type_name[o->type]);
            oecm_puts(b, "\",\"Value\":");
            put_cfg_value(b, o, o->value);
            if (o->default_value) {
                oecm_puts(b, ",\"DefaultValue\":");
                put_cfg_value(b, o, o->default_value);
            }
            if (o->unit) {
                oecm_puts(b, ",\"Unit\":");
                oecm_put_json_cstr(b, o->unit);
            }
            oecm_puts(b, ",\"Name\":");
            oecm_put_localized(b, o->name);
            if (o->description) {
                oecm_puts(b, ",\"Description\":");
                oecm_put_localized(b, o->description);
            }
            if (o->length != 0 || o->min || o->max || o->values) {
                char sep = '{';
                oecm_puts(b, ",\"Validation\":");
                if (o->length != 0) {
                    oecm_putc(b, sep);
                    sep = ',';
                    oecm_puts(b, "\"Length\":");
                    oecm_put_u64(b, o->length);
                }
                if (o->min) {
                    oecm_putc(b, sep);
                    sep = ',';
                    oecm_puts(b, "\"Min\":");
                    oecm_puts(b, o->min);
                }
                if (o->max) {
                    oecm_putc(b, sep);
                    sep = ',';
                    oecm_puts(b, "\"Max\":");
                    oecm_puts(b, o->max);
                }
                if (o->values) {
                    oecm_putc(b, sep);
                    oecm_puts(b, "\"Values\":[");
                    for (size_t v = 0; v < o->value_count; v++) {
                        if (v)
                            oecm_putc(b, ',');
                        oecm_put_json_cstr(b, o->values[v]);
                    }
                    oecm_putc(b, ']');
                }
                oecm_putc(b, '}');
            }
            if (o->mandatory)
                oecm_puts(b, ",\"Mandatory\":true");
            if (o->sensitive)
                oecm_puts(b, ",\"Sensitive\":true");
            oecm_putc(b, '}');
        }
        oecm_putc(b, '}');
    }
    oecm_putc(b, '}');
}

static size_t bounded_len(const char *s, size_t cap) {
    size_t n = 0;
    while (n < cap && s[n] != '\0')
        n++;
    return n;
}

static bool same(const char *a, const char *b) {
    size_t n = strlen(a);
    return n == strlen(b) && memcmp(a, b, n) == 0;
}

static void put_double(oecm_buf *b, double v, unsigned decimals) {
    if (v != v)
        oecm_puts(b, "\"NaN\"");
    else if (v - v != 0.0)
        oecm_puts(b, v > 0 ? "\"Infinity\"" : "\"-Infinity\"");
    else
        oecm_put_f64(b, v, decimals);
}

static void put_field_value(oecm_buf *b, const oecm_field *f) {
    switch (f->type) {
    case OECM_TYPE_BOOLEAN:
        oecm_puts(b, *(const bool *)f->value ? "true" : "false");
        break;
    case OECM_TYPE_INT32:
        oecm_put_i64(b, *(const int32_t *)f->value);
        break;
    case OECM_TYPE_UINT32:
        oecm_put_u64(b, *(const uint32_t *)f->value);
        break;
    case OECM_TYPE_FLOAT:
        put_double(b, (double)*(const float *)f->value, f->decimals);
        break;
    case OECM_TYPE_DOUBLE:
        put_double(b, *(const double *)f->value, f->decimals);
        break;
    case OECM_TYPE_STRING: {
        const char *v = f->value;
        size_t n = bounded_len(v, f->cap);
        oecm_put_json_str(b, v, oecm_utf8_valid(v, n) ? n : 0);
        break;
    }
    }
}

static void put_data(oecm_buf *b, const oecm_data *d) {
    for (size_t i = 0; i < d->field_count; i++) {
        oecm_putc(b, i ? ',' : '{');
        oecm_put_json_cstr(b, d->fields[i].name);
        oecm_putc(b, ':');
        put_field_value(b, &d->fields[i]);
    }
    oecm_putc(b, '}');
}

/* One DataSetMessage of the resource (an item of an answer). */
static void put_item(oecm_buf *b, const oecm_state *s, enum oecm_res res, size_t i, int64_t now) {
    switch (res) {
    case OECM_R_MAM:
        oecm_put_dsm(b, s, OECM_WRITER_MAM, "", NULL, -1, 0, NULL);
        oecm_put_mam(b, s);
        break;
    case OECM_R_HEALTH:
        oecm_put_dsm(b, s, OECM_WRITER_HEALTH, "", NULL, -1, 0, NULL);
        oecm_put_health(b, (oecm_health)s->health, s->score);
        break;
    case OECM_R_PROFILE:
        oecm_put_dsm(b, s, OECM_WRITER_PROFILE, "", NULL, -1, 0, "ua-keyframe");
        oecm_puts(b, "{\"Resources\":[\"MAM\",\"Health\",\"Config\",\"Data\",\"Metadata\","
                     "\"Event\",\"Profile\",\"PublicationList\",\"SubscriptionList\"]}");
        break;
    case OECM_R_PUBLIST: {
        oecm_pub_entry e;
        oecm_publist_entry(s, i, &e);
        oecm_put_dsm(b, s, OECM_WRITER_PUBLIST, e.resource, e.filter, -1, 0, "ua-keyframe");
        oecm_puts(b, "{\"Resource\":\"");
        oecm_puts(b, e.resource);
        oecm_puts(b, "\",\"Source\":");
        oecm_put_json_str(b, s->il, s->il_len);
        if (e.filter) {
            oecm_puts(b, ",\"Filter\":");
            oecm_put_json_cstr(b, e.filter);
        }
        oecm_puts(b, ",\"DataSetWriterId\":");
        oecm_put_u64(b, e.writer);
        oecm_puts(b, ",\"Mode\":\"");
        oecm_puts(b, e.mode);
        oecm_puts(b, "\"}");
        break;
    }
    case OECM_R_SUBLIST: {
        const char *r, *tag;
        oecm_sublist_entry(s, i, &r, &tag);
        oecm_put_dsm(b, s, OECM_WRITER_SUBLIST, r, tag, -1, 0, "ua-keyframe");
        oecm_puts(b, "{\"TopicPath\":\"");
        oecm_put_sublist_path(b, s, i);
        oecm_puts(b, "\",\"Config\":\"NONE_0\"}");
        break;
    }
    case OECM_R_CONFIG:
        oecm_put_dsm(b, s, oecm_config_writer(s, i), s->cfg->config[i].name, NULL,
                     s->config_ms[i], 0, "ua-keyframe");
        put_config(b, &s->cfg->config[i]);
        break;
    case OECM_R_DATA:
        oecm_put_dsm(b, s, oecm_data_writer(s, i), s->cfg->data[i].tag, NULL, now,
                     data_version(s, i), "ua-keyframe");
        put_data(b, &s->cfg->data[i]);
        break;
    }
    oecm_putc(b, '}');
}

/* --- Answers and pagination (MG 8.2.3, oec-core pagination.rs) -------------- */

typedef struct page {
    bool paginated;
    uint32_t total, per_page, page;
    uint64_t first_ms; /* PaginationId */
    uint32_t first_counter;
    size_t lo, hi; /* items[lo, hi) */
} page;

static const uint8_t *page_class(const oecm_state *s, const oecm_answer *a, size_t lo, size_t hi) {
    if (a->res != OECM_R_DATA || lo >= hi)
        return NULL;
    const uint8_t *c = s->class_id[a->items[lo]];
    for (size_t i = lo + 1; i < hi; i++)
        if (memcmp(s->class_id[a->items[i]], c, 16) != 0)
            return NULL;
    return c;
}

static void put_page(oecm_buf *b, const oecm_state *s, const oecm_answer *a, const page *p,
                     const uint8_t *data_class, int64_t now) {
    const char *fixed = a->res == OECM_R_DATA ? NULL : fixed_class[a->res];
    oecm_put_nm(b, s, "ua-data", fixed, data_class, a->reply_to);
    oecm_puts(b, ",\"Messages\":[");
    bool first = true;
    if (p->paginated) {
        oecm_put_dsm(b, s, OECM_WRITER_PAGINATION, "Pagination", NULL, -1, 0, "ua-keyframe");
        oecm_puts(b, "{\"TotalCount\":");
        oecm_put_u64(b, p->total);
        oecm_puts(b, ",\"PerPage\":");
        oecm_put_u64(b, p->per_page);
        oecm_puts(b, ",\"Page\":");
        oecm_put_u64(b, p->page);
        oecm_puts(b, ",\"HasNext\":");
        oecm_puts(b, (uint64_t)p->page * p->per_page < p->total ? "true" : "false");
        oecm_puts(b, ",\"PaginationId\":\"");
        oecm_put_id(b, s, p->first_ms, p->first_counter);
        oecm_puts(b, "\"}}");
        first = false;
    }
    for (size_t i = p->lo; i < p->hi; i++) {
        if (!first)
            oecm_putc(b, ',');
        first = false;
        put_item(b, s, a->res, a->items[i], now);
    }
    oecm_puts(b, "]}");
}

static bool fits(const size_t *sizes, size_t n, size_t k, size_t limit, size_t overhead) {
    for (size_t i = 0; i < n; i += k) {
        size_t sum = overhead;
        for (size_t j = i; j < n && j < i + k; j++)
            sum += sizes[j] + 1u;
        if (sum > limit)
            return false;
    }
    return true;
}

static size_t largest_fit(const size_t *sizes, size_t n, size_t bound, size_t limit,
                          size_t overhead) {
    size_t smallest = SIZE_MAX;
    for (size_t i = 0; i < n; i++)
        if (sizes[i] < smallest)
            smallest = sizes[i];
    smallest = n == 0 || smallest == SIZE_MAX ? 1u : smallest + 1u;
    size_t room = limit > overhead ? (limit - overhead) / smallest : 0u;
    if (bound > room)
        bound = room;
    for (size_t k = bound; k >= 1; k--)
        if (fits(sizes, n, k, limit, overhead))
            return k;
    return 1;
}

static void put_answer_topic(oecm_buf *b, const oecm_state *s, const oecm_answer *a) {
    oecm_put_pub_prefix(b, s);
    if (a->topic_rest.n != 0) {
        oecm_put(b, a->topic_rest.p, a->topic_rest.n);
    } else {
        oecm_puts(b, a->topic_res);
        if (a->topic_source) {
            oecm_putc(b, '/');
            oecm_put_b64url(b, (const uint8_t *)s->il, s->il_len);
            if (a->topic_filter) {
                oecm_putc(b, '/');
                oecm_puts(b, a->topic_filter);
            }
        }
    }
    oecm_putc(b, '\0');
}

static size_t counted_page(oecm_state *s, const oecm_answer *a, bool paginated, int64_t now) {
    uint64_t ms = s->id_ms;
    uint32_t counter = s->id_counter;
    s->id_ms = UINT64_MAX; /* the longest MessageId */
    s->id_counter = UINT32_MAX;
    page p = {paginated, UINT32_MAX, UINT32_MAX, UINT32_MAX, UINT64_MAX, UINT32_MAX, 0, 0};
    static const uint8_t probe[16];
    oecm_buf b;
    oecm_buf_count(&b);
    put_page(&b, s, a, &p, a->res == OECM_R_DATA && a->n != 0 ? probe : NULL, now);
    s->id_ms = ms;
    s->id_counter = counter;
    return b.len;
}

oecm_err oecm_answer_publish(oecm_state *s, const oecm_answer *a) {
    oecm_buf b;
    oecm_buf_init(&b, s->tx, s->tx_cap);
    put_answer_topic(&b, s, a);
    if (b.full)
        return OECM_E_NOSPACE;
    size_t at = b.len;
    size_t limit = oecm_payload_limit(s, at - 1u);
    int64_t now = oecm_wall(s);
    size_t n = a->n;
    size_t sizes[OECM_ITEMS_MAX];
    size_t total = 0;
    for (size_t i = 0; i < n; i++) {
        oecm_buf c;
        oecm_buf_count(&c);
        put_item(&c, s, a->res, a->items[i], now);
        sizes[i] = c.len;
        total += c.len + 1u;
    }
    size_t single = counted_page(s, a, false, now);
    size_t paged = counted_page(s, a, true, now);

    /* the plan (REQ-PAG-002 … 008) */
    bool paginated;
    size_t per_page;
    uint32_t page_from, page_to; /* page numbers to publish */
    if (!a->paginate || a->per_page == 0) {
        if (single + total <= limit) {
            paginated = false;
            per_page = n > 0 ? n : 1u;
            page_from = page_to = 1;
        } else {
            paginated = true;
            per_page = largest_fit(sizes, n, n, limit, paged);
            page_from = 1;
            page_to = (uint32_t)((n + per_page - 1u) / per_page);
        }
    } else {
        paginated = true;
        size_t wanted = a->per_page;
        size_t bounded = wanted < n ? wanted : n;
        per_page = fits(sizes, n, bounded > 0 ? bounded : 1u, limit, paged)
                       ? wanted
                       : largest_fit(sizes, n, bounded, limit, paged);
        if (a->page == 0) {
            page_from = 1;
            page_to = n == 0 ? 1u : (uint32_t)((n + per_page - 1u) / per_page);
        } else {
            page_from = page_to = a->page;
        }
    }
    page p = {paginated, (uint32_t)n, (uint32_t)per_page, 0, 0, 0, 0, 0};
    for (uint32_t pg = page_from; pg >= page_from && pg <= page_to; pg++) {
        uint64_t lo = (uint64_t)(pg - 1u) * per_page, hi = (uint64_t)pg * per_page;
        p.page = pg;
        p.lo = lo < n ? (size_t)lo : n;
        p.hi = hi < n ? (size_t)hi : n;
        if (!paginated) {
            p.lo = 0;
            p.hi = n;
        }
        oecm_next_message_id(s, now);
        if (pg == page_from) {
            p.first_ms = s->id_ms;
            p.first_counter = s->id_counter;
        }
        oecm_buf_init(&b, s->tx, s->tx_cap);
        b.len = at;
        put_page(&b, s, a, &p, page_class(s, a, p.lo, p.hi), now);
        if (b.full)
            return OECM_E_NOSPACE;
        oecm_err err = oecm_publish(s, at, b.len - at);
        if (err != OECM_OK)
            return err;
    }
    return OECM_OK;
}

/* --- Publications ---------------------------------------------------------------- */

oecm_err oecm_publish_data(oecm_state *s, size_t data, oecm_span reply_to) {
    oecm_answer a = {.res = OECM_R_DATA,
                     .topic_res = "Data",
                     .topic_source = true,
                     .topic_filter = s->cfg->data[data].tag,
                     .reply_to = reply_to,
                     .n = 1};
    a.items[0] = (uint8_t)data;
    return oecm_answer_publish(s, &a);
}

oecm_err oecm_publish_config(oecm_state *s, size_t ctx, oecm_span reply_to) {
    oecm_answer a = {.res = OECM_R_CONFIG,
                     .topic_res = "Config",
                     .topic_source = true,
                     .topic_filter = s->cfg->config[ctx].name,
                     .reply_to = reply_to,
                     .n = 1};
    a.items[0] = (uint8_t)ctx;
    return oecm_answer_publish(s, &a);
}

static oecm_err publish_list(oecm_state *s, enum oecm_res res, const char *name, size_t n) {
    oecm_answer a = {.res = res, .topic_res = name, .n = n};
    for (size_t i = 0; i < n; i++)
        a.items[i] = (uint8_t)i;
    return oecm_answer_publish(s, &a);
}

static void put_field_meta(oecm_buf *b, const oecm_state *s, size_t j, const oecm_field *f) {
    uint8_t id[16];
    oecm_uuid5(s->class_id[j], f->name, strlen(f->name), id);
    unsigned t = builtin[f->type];
    oecm_puts(b, "{\"ArrayDimensions\":[],\"BuiltInType\":");
    oecm_put_u64(b, t);
    oecm_puts(b, ",\"DataSetFieldId\":\"");
    oecm_put_uuid(b, id);
    oecm_puts(b, "\",\"DataType\":\"i=");
    oecm_put_u64(b, t);
    oecm_puts(b, "\",\"Description\":");
    oecm_put_localized(b, f->description ? f->description : "");
    oecm_puts(b, ",\"FieldFlags\":0,\"MaxStringLength\":0,\"Name\":");
    oecm_put_json_cstr(b, f->name);
    oecm_puts(b, ",\"Properties\":[");
    if (f->eu_low) {
        oecm_puts(b, "{\"Key\":\"EURange\",\"Value\":{\"UaType\":22,\"Value\":{\"High\":");
        oecm_puts(b, f->eu_high);
        oecm_puts(b, ",\"Low\":");
        oecm_puts(b, f->eu_low);
        oecm_puts(b, ",\"UaTypeId\":\"i=884\"}}}");
    }
    if (f->unit) {
        if (f->eu_low)
            oecm_putc(b, ',');
        oecm_puts(b, "{\"Key\":\"EngineeringUnits\",\"Value\":{\"UaType\":22,\"Value\":{"
                     "\"DisplayName\":{\"Text\":");
        oecm_put_json_cstr(b, f->unit);
        oecm_puts(b, "},\"NamespaceUri\":\"http://www.opcfoundation.org/UA/units/un/cefact\","
                     "\"UaTypeId\":\"i=887\",\"UnitId\":-1}}}");
    }
    oecm_puts(b, "],\"ValueRank\":-1}");
}

static void put_metadata(oecm_buf *b, const oecm_state *s, size_t j, oecm_span reply_to) {
    const oecm_data *d = &s->cfg->data[j];
    uint32_t v = data_version(s, j);
    oecm_put_nm(b, s, "ua-metadata", NULL, NULL, reply_to);
    oecm_puts(b, ",\"DataSetWriterId\":");
    oecm_put_u64(b, oecm_data_writer(s, j));
    oecm_puts(b, ",\"WriterGroupName\":");
    oecm_put_json_cstr(b, d->tag);
    oecm_puts(b, ",\"DataSetWriterName\":");
    oecm_put_json_str(b, s->il, s->il_len);
    oecm_puts(b, ",\"Timestamp\":\"");
    oecm_put_time(b, s->meta_ms);
    oecm_puts(b, "\",\"MetaData\":{\"ConfigurationVersion\":{\"MajorVersion\":");
    oecm_put_u64(b, v);
    oecm_puts(b, ",\"MinorVersion\":");
    oecm_put_u64(b, v);
    oecm_puts(b, "},\"DataSetClassId\":\"");
    oecm_put_uuid(b, s->class_id[j]);
    oecm_puts(b, "\",\"EnumDataTypes\":[],\"Fields\":[");
    for (size_t i = 0; i < d->field_count; i++) {
        if (i)
            oecm_putc(b, ',');
        put_field_meta(b, s, j, &d->fields[i]);
    }
    oecm_puts(b, "],\"Name\":");
    oecm_put_json_cstr(b, d->tag);
    oecm_puts(b, ",\"Namespaces\":[],\"SimpleDataTypes\":[],\"StructureDataTypes\":[]");
    if (d->description) {
        oecm_puts(b, ",\"Description\":");
        oecm_put_localized(b, d->description);
    }
    oecm_puts(b, "}}");
}

static void put_own_topic(oecm_buf *b, const oecm_state *s, const char *res, const char *f1,
                          const char *f2) {
    oecm_put_pub_prefix(b, s);
    oecm_puts(b, res);
    oecm_putc(b, '/');
    oecm_put_b64url(b, (const uint8_t *)s->il, s->il_len);
    oecm_putc(b, '/');
    oecm_puts(b, f1);
    if (f2) {
        oecm_putc(b, '/');
        oecm_puts(b, f2);
    }
    oecm_putc(b, '\0');
}

oecm_err oecm_publish_metadata(oecm_state *s, size_t data, oecm_span reply_to) {
    int64_t now = oecm_wall(s);
    if (s->meta_ms == 0)
        s->meta_ms = now; /* first sending (REQ-MSG-032) */
    oecm_buf b;
    oecm_buf_init(&b, s->tx, s->tx_cap);
    put_own_topic(&b, s, "Metadata", s->cfg->data[data].tag, NULL);
    size_t at = b.len;
    oecm_next_message_id(s, now);
    put_metadata(&b, s, data, reply_to);
    if (b.full)
        return OECM_E_NOSPACE;
    return oecm_publish(s, at, b.len - at);
}

oecm_err oecm_publish_status(oecm_state *s, const oecm_status *st, oecm_span reply_to) {
    bool good = st->number == 0;
    const char *level = good ? "Good" : "Bad";
    int64_t now = oecm_wall(s);
    oecm_buf b;
    oecm_buf_init(&b, s->tx, s->tx_cap);
    put_own_topic(&b, s, "Event", "Status", level);
    size_t at = b.len;
    oecm_next_message_id(s, now);
    oecm_put_nm(&b, s, "ua-data", OECM_CLASS_STATUS, NULL, reply_to);
    oecm_puts(&b, ",\"Messages\":[");
    oecm_put_dsm(&b, s, good ? OECM_WRITER_STATUS : OECM_WRITER_STATUS + 2u, "Status", level, now,
                 0, "ua-event");
    oecm_puts(&b, "{\"Number\":");
    oecm_put_u64(&b, st->number);
    oecm_puts(&b, ",\"Category\":\"CAT_STATUS_1\",\"Description\":");
    oecm_put_json_cstr(&b, st->description);
    oecm_puts(&b, ",\"Details\":{\"SymbolicId\":");
    oecm_put_json_cstr(&b, st->symbol);
    oecm_puts(&b, "}}}]}");
    if (b.full)
        return OECM_E_NOSPACE;
    return oecm_publish(s, at, b.len - at);
}

/* --- Connect, change, Interval -------------------------------------------------- */

static oecm_err subscribe(oecm_state *s, const char *method) {
    oecm_buf b;
    oecm_buf_init(&b, s->tx, s->tx_cap);
    oecm_puts(&b, "Oi4v2/");
    oecm_puts(&b, oecm_service_type_name(s->cfg->service_type));
    oecm_putc(&b, '/');
    oecm_put_b64url(&b, (const uint8_t *)s->il, s->il_len);
    oecm_putc(&b, '/');
    oecm_puts(&b, method);
    oecm_puts(&b, "/#");
    oecm_putc(&b, '\0');
    if (b.full)
        return OECM_E_NOSPACE;
    return s->pal->mqtt_subscribe(s->pal->ctx, (const char *)s->tx) == OECM_OK ? OECM_OK
                                                                               : OECM_E_PAL;
}

oecm_err oecm_l2_subscribe(oecm_state *s) {
    static const char *const methods[] = {"Get", "Set", "Del"};
    oecm_err err = OECM_OK;
    s->subs_due = true;
    for (size_t i = 0; i < 3 && err == OECM_OK; i++)
        err = subscribe(s, methods[i]);
    if (err == OECM_OK)
        s->subs_due = false;
    return err;
}

static oecm_err connected(oecm_state *s) {
    const oecm_config *c = s->cfg;
    oecm_err err = s->subs_due ? oecm_l2_subscribe(s) : OECM_OK;
    /* the order of oec-core provider.rs on_connected: Profile,
     * PublicationList, SubscriptionList, Config, Metadata, Data */
    if (err == OECM_OK) {
        oecm_answer a = {.res = OECM_R_PROFILE, .topic_res = "Profile", .topic_source = true,
                         .n = 1};
        err = oecm_answer_publish(s, &a);
    }
    if (err == OECM_OK)
        err = publish_list(s, OECM_R_PUBLIST, "PublicationList", oecm_publist_count(s));
    if (err == OECM_OK && oecm_sublist_count(s) != 0)
        err = publish_list(s, OECM_R_SUBLIST, "SubscriptionList", oecm_sublist_count(s));
    for (size_t i = 0; i < c->config_count && err == OECM_OK; i++) {
        if (s->config_ms[i] == 0)
            s->config_ms[i] = oecm_wall(s);
        err = oecm_publish_config(s, i, NO_REPLY);
    }
    for (size_t j = 0; j < c->data_count && err == OECM_OK; j++)
        err = oecm_publish_metadata(s, j, NO_REPLY);
    uint32_t now = oecm_mono(s);
    for (size_t j = 0; j < c->data_count && err == OECM_OK; j++) {
        err = oecm_publish_data(s, j, NO_REPLY);
        s->data_ms[j] = now;
    }
    if (err != OECM_OK) {
        s->lists_due = true;
        return err;
    }
    s->lists_due = false;
    s->data_due = 0;
    s->config_due = 0;
    return OECM_OK;
}

oecm_err oecm_l2_connected(oecm_state *s) { return connected(s); }

oecm_err oecm_l2_publish_due(oecm_state *s) {
    if (s->lists_due)
        return connected(s);
    const oecm_config *c = s->cfg;
    for (size_t i = 0; i < c->config_count; i++) {
        if ((s->config_due & 1u << i) == 0)
            continue;
        oecm_err err = oecm_publish_config(s, i, NO_REPLY);
        if (err != OECM_OK)
            return err;
        s->config_due = (uint8_t)(s->config_due & ~(1u << i));
    }
    uint32_t now = oecm_mono(s);
    for (size_t j = 0; j < c->data_count; j++) {
        uint32_t iv = c->data[j].interval_ms;
        bool due = (s->data_due & 1u << j) != 0 || (iv != 0 && now - s->data_ms[j] >= iv);
        if (!due)
            continue;
        oecm_err err = oecm_publish_data(s, j, NO_REPLY);
        if (err != OECM_OK)
            return err;
        s->data_due = (uint8_t)(s->data_due & ~(1u << j));
        s->data_ms[j] = now;
    }
    return OECM_OK;
}

uint32_t oecm_l2_wait_ms(const oecm_state *s, uint32_t wait) {
    if (s->lists_due || s->data_due || s->config_due)
        return 0;
    uint32_t now = oecm_mono(s);
    for (size_t j = 0; j < s->cfg->data_count; j++) {
        uint32_t iv = s->cfg->data[j].interval_ms;
        if (iv == 0)
            continue;
        uint32_t since = now - s->data_ms[j];
        if (since >= iv)
            return 0;
        if (iv - since < wait)
            wait = iv - since;
    }
    return wait;
}

/* --- Definitions (oecm_init) ---------------------------------------------------- */

static bool text_ok(const char *s) { return s == NULL || oecm_utf8_valid(s, strlen(s)); }

/* Tag / FieldName / ConfigName: letters, digits, '_' (and '-' if dash). */
static bool name_ok(const char *s, bool dash) {
    if (s == NULL || s[0] == '\0' || strlen(s) > 64u)
        return false;
    for (const char *p = s; *p; p++) {
        char c = *p;
        if (!((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
              c == '_' || (dash && c == '-')))
            return false;
    }
    return true;
}

/* GroupName / ConfigName (oec-core config.rs valid_name, REQ-RES-CONFIG-004):
 * not starting with a digit or '-'. */
static bool cfg_name_ok(const char *s) {
    return name_ok(s, true) && !(s[0] == '-' || (s[0] >= '0' && s[0] <= '9'));
}

static bool number_ok(const char *s) {
    return s == NULL || oecm_is_json_number(s, strlen(s));
}

static bool field_ok(const oecm_field *f) {
    if (!name_ok(f->name, false) || f->value == NULL || (unsigned)f->type > OECM_TYPE_STRING)
        return false;
    if (f->type == OECM_TYPE_STRING && f->cap == 0)
        return false;
    if (f->decimals > 9 || !text_ok(f->description) || !text_ok(f->unit))
        return false;
    if ((f->eu_low == NULL) != (f->eu_high == NULL) || !number_ok(f->eu_low) ||
        !number_ok(f->eu_high))
        return false;
    return true;
}

static bool data_ok(const oecm_config *c, size_t j) {
    const oecm_data *d = &c->data[j];
    if (!name_ok(d->tag, true) || !text_ok(d->description) || d->fields == NULL ||
        d->field_count == 0 || d->field_count > OECM_FIELDS_MAX)
        return false;
    for (size_t k = 0; k < j; k++)
        if (same(c->data[k].tag, d->tag))
            return false;
    for (size_t i = 0; i < d->field_count; i++) {
        if (!field_ok(&d->fields[i]))
            return false;
        for (size_t k = 0; k < i; k++)
            if (same(d->fields[k].name, d->fields[i].name))
                return false;
    }
    return true;
}

static bool object_ok(const oecm_cfg_object *o) {
    if (!cfg_name_ok(o->key) || same(o->key, "Name") || same(o->key, "Description") ||
        o->name == NULL || !text_ok(o->name) ||
        !text_ok(o->description) || !text_ok(o->unit) || !text_ok(o->default_value) ||
        (unsigned)o->type > OECM_CFG_STRING || o->value == NULL || o->cap == 0)
        return false;
    size_t n = bounded_len(o->value, o->cap);
    if (n == o->cap || !oecm_utf8_valid(o->value, n))
        return false;
    if (!number_ok(o->min) || !number_ok(o->max) || (o->values == NULL && o->value_count != 0))
        return false;
    for (size_t v = 0; v < o->value_count; v++)
        if (o->values[v] == NULL || !text_ok(o->values[v]))
            return false;
    return true;
}

/* A Config context Filter: one topic level of printable ASCII. */
static bool context_name_ok(const char *s) {
    if (s == NULL || s[0] == '\0' || strlen(s) > 64u)
        return false;
    for (const char *p = s; *p; p++)
        if (*p <= ' ' || *p > '~' || *p == '/' || *p == '+' || *p == '#')
            return false;
    return true;
}

static bool context_ok(const oecm_config *c, size_t i) {
    const oecm_cfg_context *x = &c->config[i];
    if (!context_name_ok(x->name) || x->title == NULL || !text_ok(x->title) ||
        !text_ok(x->description) || (x->groups == NULL && x->count != 0))
        return false;
    for (size_t k = 0; k < i; k++)
        if (same(c->config[k].name, x->name))
            return false;
    for (size_t g = 0; g < x->count; g++) {
        const oecm_cfg_group *grp = &x->groups[g];
        if (!cfg_name_ok(grp->key) || same(grp->key, "Context") || grp->name == NULL ||
            !text_ok(grp->name) || !text_ok(grp->description) ||
            (grp->objects == NULL && grp->count != 0))
            return false;
        for (size_t k = 0; k < g; k++)
            if (same(x->groups[k].key, grp->key))
                return false;
        for (size_t o = 0; o < grp->count; o++) {
            if (!object_ok(&grp->objects[o]))
                return false;
            for (size_t k = 0; k < o; k++)
                if (same(grp->objects[k].key, grp->objects[o].key))
                    return false;
        }
    }
    return true;
}

/* The DataSetClassId: UUIDv5 of ["<Tag>",[["<Name>",<BuiltInType>,-1],…]]
 * (oec-core data.rs class_id). */
static void derive_class(oecm_state *s, size_t j) {
    const oecm_data *d = &s->cfg->data[j];
    oecm_buf b;
    oecm_buf_init(&b, s->tx, s->tx_cap);
    oecm_putc(&b, '[');
    oecm_put_json_cstr(&b, d->tag);
    oecm_puts(&b, ",[");
    for (size_t i = 0; i < d->field_count; i++) {
        if (i)
            oecm_putc(&b, ',');
        oecm_putc(&b, '[');
        oecm_put_json_cstr(&b, d->fields[i].name);
        oecm_putc(&b, ',');
        oecm_put_u64(&b, builtin[d->fields[i].type]);
        oecm_puts(&b, ",-1]");
    }
    oecm_puts(&b, "]]");
    oecm_uuid5(class_ns, s->tx, b.len, s->class_id[j]);
}

/* Whether one item fits a NetworkMessage of its own on its own topic. */
static bool item_fits(oecm_state *s, enum oecm_res res, size_t i, const char *name,
                      const char *filter) {
    oecm_answer a = {.res = res, .topic_res = name, .topic_source = true, .topic_filter = filter,
                     .n = 1};
    a.items[0] = (uint8_t)i;
    oecm_buf b;
    oecm_buf_count(&b);
    put_answer_topic(&b, s, &a);
    size_t topic = b.len;
    oecm_buf_count(&b);
    put_item(&b, s, res, i, INT64_C(253402300799999));
    return counted_page(s, &a, true, 0) + b.len + 1u <= oecm_payload_limit(s, topic);
}

oecm_err oecm_l2_init(oecm_state *s) {
    const oecm_config *c = s->cfg;
    if (c->data_count > OECM_DATA_MAX || c->config_count > OECM_CONFIG_MAX ||
        (c->data == NULL && c->data_count != 0) || (c->config == NULL && c->config_count != 0))
        return OECM_E_ARG;
    for (size_t j = 0; j < c->data_count; j++)
        if (!data_ok(c, j))
            return OECM_E_ARG;
    for (size_t i = 0; i < c->config_count; i++)
        if (!context_ok(c, i))
            return OECM_E_ARG;
    if (oecm_publist_count(s) > OECM_ITEMS_MAX || oecm_sublist_count(s) > OECM_ITEMS_MAX)
        return OECM_E_ARG;
    for (size_t j = 0; j < c->data_count; j++)
        derive_class(s, j);
    /* every DataSetMessage fits; the Metadata with the longest MessageId */
    for (size_t j = 0; j < c->data_count; j++) {
        if (!item_fits(s, OECM_R_DATA, j, "Data", c->data[j].tag))
            return OECM_E_NOSPACE;
        oecm_buf b;
        oecm_buf_count(&b);
        put_own_topic(&b, s, "Metadata", c->data[j].tag, NULL);
        size_t topic = b.len - 1u;
        uint64_t ms = s->id_ms;
        uint32_t counter = s->id_counter;
        s->id_ms = UINT64_MAX;
        s->id_counter = UINT32_MAX;
        s->meta_ms = INT64_C(253402300799999);
        oecm_buf_count(&b);
        put_metadata(&b, s, j, NO_REPLY);
        s->id_ms = ms;
        s->id_counter = counter;
        s->meta_ms = 0;
        if (b.len + 64u > oecm_payload_limit(s, topic))
            return OECM_E_NOSPACE;
    }
    for (size_t i = 0; i < c->config_count; i++) {
        s->config_ms[i] = INT64_C(253402300799999);
        bool ok = item_fits(s, OECM_R_CONFIG, i, "Config", c->config[i].name);
        s->config_ms[i] = 0;
        if (!ok)
            return OECM_E_NOSPACE;
    }
    return OECM_OK;
}
