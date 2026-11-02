/*
 * Internal interfaces of the micro SDK core (ADR-0030). Not part of the
 * API; the host tests include it to test the parts directly.
 */
#ifndef OECM_INTERNAL_H
#define OECM_INTERNAL_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include "oecm.h"

/* --- Identification Link (MG 4, REQ-IL-*) -------------------------------- */

#define OECM_IL_MAX 255u         /* REQ-IL-019 */
#define OECM_IL_ENCODED_MAX 340u /* base64url of 255 octets */

/* Violation bits; the order of oecm_il_violation_codes is the order of the
 * shared vectors (oec-core il.rs). */
enum {
    OECM_IL_EMPTY = 1u << 0,
    OECM_IL_TOO_LONG = 1u << 1,
    OECM_IL_FORBIDDEN = 1u << 2,
    OECM_IL_EMPTY_AUTHORITY = 1u << 3,
    OECM_IL_UPPERCASE_AUTHORITY = 1u << 4,
    OECM_IL_ENCODING = 1u << 5,
    OECM_IL_MANUFACTURER_URI = 1u << 6
};
#define OECM_IL_VIOLATION_KINDS 7u
/* Code of violation bit i (e.g. "forbidden-character"). */
const char *oecm_il_violation_code(unsigned bit_index);

/* The machine-checkable violations of an Identification Link (REQ-IL-008,
 * 009, 016, 018, 019); 0: valid. */
unsigned oecm_il_check(const char *s, size_t n);

/* Violations of a decoded AppId/Source topic element: OECM_IL_ENCODING if
 * not canonical unpadded base64url, else those of the decoded link.
 * out (cap >= OECM_IL_MAX + 1) receives the decoded octets and a NUL. */
unsigned oecm_il_decode(const char *element, size_t n, char *out, size_t cap, size_t *out_len);

/* ManufacturerUri rule (REQ-IL-024, REQ-RES-MAM-004), case-sensitive. */
bool oecm_manufacturer_uri_valid(const char *s, size_t n);

/* Builds the Oi4Identifier (REQ-IL-007, 022 … 034) into out (NUL-terminated);
 * the ManufacturerUri is lower-cased first. Returns the violations; 0:
 * valid. NULL elements are empty ("nd"). */
unsigned oecm_oi4identifier(const char *manufacturer_uri, const char *model,
                            const char *product_code, const char *serial_number, char *out,
                            size_t cap, size_t *out_len);

bool oecm_utf8_valid(const char *s, size_t n);

/* --- Output buffer --------------------------------------------------------- */

typedef struct oecm_buf {
    uint8_t *p;
    size_t cap;
    size_t len;
    bool full; /* something did not fit; the content is incomplete */
} oecm_buf;

/* p NULL: counting mode, only len advances (sizes for the pagination). */
void oecm_buf_init(oecm_buf *b, uint8_t *p, size_t cap);
void oecm_buf_count(oecm_buf *b);
void oecm_put(oecm_buf *b, const void *s, size_t n);
void oecm_puts(oecm_buf *b, const char *s);
void oecm_putc(oecm_buf *b, char c);
void oecm_put_u64(oecm_buf *b, uint64_t v);
void oecm_put_i64(oecm_buf *b, int64_t v);
void oecm_put_b64url(oecm_buf *b, const uint8_t *s, size_t n);
void oecm_put_dnp(oecm_buf *b, const char *s, size_t n);
/* A JSON string with quotes; s must be valid UTF-8. */
void oecm_put_json_str(oecm_buf *b, const char *s, size_t n);
/* The escaped content of a JSON string, without quotes. */
void oecm_put_json_chars(oecm_buf *b, const char *s, size_t n);

size_t oecm_b64url_len(size_t n);

/* --- Numbers, time, UUIDs (oecm_num.c) ---------------------------------- */

/* A finite double with `decimals` digits after the point; values beyond
 * 1e15 in scientific notation with 15 significant digits. */
void oecm_put_f64(oecm_buf *b, double v, unsigned decimals);
/* RFC 3339 UTC with milliseconds, e.g. 2026-11-10T08:00:00.000Z. */
void oecm_put_time(oecm_buf *b, int64_t unix_ms);
/* A JSON number token (validated grammar) as double; false if not one. */
bool oecm_parse_f64(const char *p, size_t n, double *out);
/* Whether p[0..n) is exactly one JSON number token. */
bool oecm_is_json_number(const char *p, size_t n);

typedef struct oecm_sha1 {
    uint32_t h[5];
    uint8_t block[64];
    uint64_t len;
} oecm_sha1;
void oecm_sha1_init(oecm_sha1 *c);
void oecm_sha1_update(oecm_sha1 *c, const void *p, size_t n);
void oecm_sha1_final(oecm_sha1 *c, uint8_t out[20]);
/* RFC 9562 UUID version 5 of name in namespace ns. */
void oecm_uuid5(const uint8_t ns[16], const void *name, size_t n, uint8_t out[16]);
void oecm_put_uuid(oecm_buf *b, const uint8_t u[16]);

/* --- JSON reader (oecm_json.c) ------------------------------------------- */

#define OECM_JSON_DEPTH 16u

typedef struct oecm_span {
    const char *p;
    size_t n;
} oecm_span;

typedef enum oecm_jkind {
    OECM_J_INVALID,
    OECM_J_NULL,
    OECM_J_BOOL,
    OECM_J_NUMBER,
    OECM_J_STRING,
    OECM_J_ARRAY,
    OECM_J_OBJECT
} oecm_jkind;

/* Validates one RFC 8259 document (UTF-8, no lone surrogates, nesting at
 * most OECM_JSON_DEPTH) without recursion; *root is its value. Every other
 * reader function expects spans of a validated document. */
bool oecm_json_parse(const char *p, size_t n, oecm_span *root);
oecm_jkind oecm_json_kind(oecm_span v);

typedef struct oecm_jit {
    const char *p;
    const char *end;
} oecm_jit;
/* Iterates the members of an object or the elements of an array. */
void oecm_json_iter(oecm_span container, oecm_jit *it);
/* The next member (key: the string token) or element (key may be NULL). */
bool oecm_json_next(oecm_jit *it, oecm_span *key, oecm_span *value);
/* The value of key in obj; the last of duplicate keys (as serde_json). */
bool oecm_json_get(oecm_span obj, const char *key, oecm_span *value);
/* A string token whose decoded text is s[0..n). */
bool oecm_json_str_eq(oecm_span v, const char *s, size_t n);
bool oecm_json_streq(oecm_span v, const char *s);
/* Decodes a string token into out (NUL-terminated); false if it is not a
 * string or does not fit. */
bool oecm_json_str(oecm_span v, char *out, size_t cap, size_t *len);
/* Decoded length in octets of a string token; *nul: it contains U+0000. */
size_t oecm_json_str_bytes(oecm_span v, bool *nul);
/* Decoded length in code points of a string token. */
size_t oecm_json_str_chars(oecm_span v);
bool oecm_json_i64(oecm_span v, int64_t *out);
bool oecm_json_f64(oecm_span v, double *out);
bool oecm_json_bool(oecm_span v, bool *out);

/* --- Topics (oecm_topic.c) ----------------------------------------------- */

/* Oi4v2/<ServiceType>/<AppId>/<Method>/<Resource>[/<Source>[/<Filter>]];
 * absent parts have n == 0. The Filter may span several levels. */
typedef struct oecm_topic {
    oecm_span st, app, method, resource, source, filter;
} oecm_topic;
/* False for fewer than 5 levels, an empty level, a wildcard or a NUL. */
bool oecm_topic_parse(const char *t, size_t n, oecm_topic *out);
/* Whether e is the (canonical, unpadded) base64url encoding of s[0..n). */
bool oecm_b64url_eq(oecm_span e, const char *s, size_t n);
bool oecm_span_eq(oecm_span a, const char *s);

/* --- Names ----------------------------------------------------------------- */

const char *oecm_service_type_name(oecm_service_type st);
const char *oecm_health_name(oecm_health h);

/* --- Messages (MG 7, MG 8) ------------------------------------------------- */

#define OECM_CLASS_MAM "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58"
#define OECM_CLASS_HEALTH "d8e7b6df-42ba-448a-975a-199f59e8ffeb"

#define OECM_CLASS_PROFILE "48017c6a-05c8-48d7-9d85-4b08bbb707f3"
#define OECM_CLASS_PUBLIST "217434d6-6e1e-4230-b907-f52bc9ffe152"
#define OECM_CLASS_SUBLIST "e5d68c47-c276-4929-8ab9-4c1090cac785"
#define OECM_CLASS_CONFIG "9d5983db-440d-4474-9fd7-1cd7a6c8b6c2"
#define OECM_CLASS_STATUS "543ae05e-b6d9-4161-a0a3-350a0fac5976"
#define OECM_CLASS_REFDES "27a75019-164a-496d-a38b-90e8a55c2cfa"

/* DataSetWriterIds of the device's own publications (ADR-0030 §9), as
 * the Rust provider assigns them: Config contexts from 18, then Data
 * (Metadata shares the id of its Data). */
#define OECM_WRITER_MAM 10u
#define OECM_WRITER_HEALTH 11u
#define OECM_WRITER_PROFILE 12u
#define OECM_WRITER_PUBLIST 13u
#define OECM_WRITER_STATUS 14u /* Good; Uncertain 15, Bad 16 */
#define OECM_WRITER_SUBLIST 17u
#define OECM_WRITER_FIRST 18u
#define OECM_WRITER_PAGINATION 2u

/* The FNV-1a 64 hash, for the derived ClientId. */
uint64_t oecm_fnv1a64(const char *s, size_t n);

/* The core's device state; must fit oecm_device (static assertion). */
typedef struct oecm_state {
    const oecm_config *cfg;
    const oecm_pal *pal;
    uint8_t *tx;
    size_t tx_cap;
    const char *il;
    size_t il_len;
    char client_id[24];
    uint8_t phase; /* 0: not initialized */
    uint8_t health;
    int8_t score; /* -1: none */
    bool health_due;
    bool birth_due;
    bool unavailable; /* the last own Health was FAILURE_1/0 */
    uint32_t last_health_ms;
    uint32_t phase_since_ms;
    uint32_t backoff_ms;
    uint64_t id_ms;      /* MessageId generator (REQ-MSG-007/008) */
    uint32_t id_counter; /* 0: no counter */
    char il_buf[OECM_IL_MAX + 1];
    /* Level 2 */
    bool subs_due;       /* Get, Set, Del subscriptions after a connect */
    bool lists_due;      /* Config, lists, Metadata, Data after a connect */
    uint8_t data_due;    /* bit per Data: publish on change */
    uint8_t config_due;  /* bit per Config context: publish on change */
    int64_t meta_ms;     /* first sending of the Metadata (REQ-MSG-032) */
    int64_t config_ms[OECM_CONFIG_MAX]; /* last change of a context */
    uint32_t data_ms[OECM_DATA_MAX];    /* last periodic publication */
    uint8_t class_id[OECM_DATA_MAX][16];
    /* Level 3 */
    bool refdes_due;        /* publish the ReferenceDesignation on change */
    uint16_t wid_count;     /* NewDataSetWriterId reservations in the window */
    uint32_t wid_window_ms; /* start of the window (oecm_mono) */
    uint32_t wid_next;      /* next DataSetWriterId to reserve */
    uint32_t wid_limit;     /* persisted: no id below it was reserved before */
    size_t refdes_len;      /* the stored object in cfg->refdes_buf; 0: none */
    size_t refdes_max;      /* the largest object that fits every answer */
} oecm_state;

/* --- Level 2 (oecm_res.c, oecm_req.c) ------------------------------------ */

typedef struct oecm_status {
    uint32_t number;
    const char *symbol;
    const char *description;
} oecm_status;
extern const oecm_status oecm_good, oecm_bad_decoding, oecm_bad_not_writable,
    oecm_bad_out_of_range, oecm_bad_not_supported, oecm_bad_not_found, oecm_bad_type_mismatch,
    oecm_bad_invalid_argument, oecm_bad_internal_error, oecm_bad_resource_unavailable;

/* Common parts of the encoders (oecm_device.c). */
void oecm_put_publisher(oecm_buf *b, const oecm_state *s);
/* Advances the MessageId generator (s->id_ms, s->id_counter). */
void oecm_next_message_id(oecm_state *s, int64_t unix_ms);
/* A MessageId (without quotes). */
void oecm_put_id(oecm_buf *b, const oecm_state *s, uint64_t ms, uint32_t counter);
/* A NetworkMessage with the current MessageId up to (without) its
 * Messages; class_id or class_uuid NULL: no DataSetClassId; reply_to: the
 * JSON string token of the ReplyTo, n == 0: "". */
void oecm_put_nm(oecm_buf *b, const oecm_state *s, const char *message_type, const char *class_id,
                 const uint8_t *class_uuid, oecm_span reply_to);
/* A DataSetMessage up to its Payload value. WriterGroupName is group or
 * group/group_tail; timestamp_ms < 0, version 0, message_type NULL:
 * omitted. */
void oecm_put_dsm(oecm_buf *b, const oecm_state *s, unsigned writer, const char *group,
                  const char *group_tail, int64_t timestamp_ms, uint32_t version,
                  const char *message_type);
void oecm_put_mam(oecm_buf *b, const oecm_state *s);
void oecm_put_health(oecm_buf *b, oecm_health h, int score);
/* "Oi4v2/<ST>/<AppId>/Pub/" */
void oecm_put_pub_prefix(oecm_buf *b, const oecm_state *s);
void oecm_put_localized(oecm_buf *b, const char *text);
void oecm_put_json_cstr(oecm_buf *b, const char *s);
oecm_err oecm_publish(oecm_state *s, size_t at, size_t len);
int64_t oecm_wall(const oecm_state *s);
uint32_t oecm_mono(const oecm_state *s);
/* The largest payload: MaxPacketSize and the room in tx after the topic. */
size_t oecm_payload_limit(const oecm_state *s, size_t topic_len);

/* Validates the Level 2 definitions and derives the DataSetClassIds;
 * uses tx as scratch. */
oecm_err oecm_l2_init(oecm_state *s);
/* The subscriptions after a connect, before the Birth (as the Rust
 * runtime: a request answering the Birth is received); retried by
 * oecm_l2_publish_due if one failed. */
oecm_err oecm_l2_subscribe(oecm_state *s);
/* The publications after a connect (Config, lists, Metadata, Data) and
 * the due Data/Config; OECM_OK if nothing failed. */
oecm_err oecm_l2_connected(oecm_state *s);
oecm_err oecm_l2_publish_due(oecm_state *s);
uint32_t oecm_l2_wait_ms(const oecm_state *s, uint32_t wait);
void oecm_l2_message(oecm_state *s, const char *topic, const uint8_t *payload, size_t len);

/* The answers (oecm_res.c), used by oecm_req.c. */
enum oecm_res {
    OECM_R_MAM,
    OECM_R_HEALTH,
    OECM_R_PROFILE,
    OECM_R_PUBLIST,
    OECM_R_SUBLIST,
    OECM_R_CONFIG,
    OECM_R_REFDES,
    OECM_R_DATA
};
#define OECM_ITEMS_MAX 32u
typedef struct oecm_answer {
    enum oecm_res res;
    /* n > 0: the topic is Pub/ + the request topic from its Resource level
     * on (MG 8.2.2); else Pub/<topic_res>[/<own Source>[/<topic_filter>]]. */
    oecm_span topic_rest;
    const char *topic_res;
    bool topic_source;
    const char *topic_filter;
    oecm_span reply_to; /* JSON string token; n == 0: "" */
    bool paginate;      /* a PaginationRequest was given (MG 8.2.3) */
    uint32_t page, per_page;
    size_t n;           /* items: indices of the resource's DataSetMessages */
    uint8_t items[OECM_ITEMS_MAX];
} oecm_answer;
/* Publishes the answer as one or more NetworkMessages (MG 8.2.3). */
oecm_err oecm_answer_publish(oecm_state *s, const oecm_answer *a);

typedef struct oecm_pub_entry {
    const char *resource; /* "MAM", "Event", "Config", … */
    const char *filter;   /* "Status/Good", the context or Tag; NULL: none */
    unsigned writer;
    const char *mode;
} oecm_pub_entry;
size_t oecm_publist_count(const oecm_state *s);
void oecm_publist_entry(const oecm_state *s, size_t i, oecm_pub_entry *e);
/* SubscriptionList entries: the Set topics of the Config contexts and of
 * the writable Data (resource "Config"/"Data", the context or Tag). */
size_t oecm_sublist_count(const oecm_state *s);
void oecm_sublist_entry(const oecm_state *s, size_t i, const char **resource, const char **tag);
void oecm_put_sublist_path(oecm_buf *b, const oecm_state *s, size_t i);
/* Status Event of a Set/Del or of an unknown Source (MG 8.3.6.1). */
oecm_err oecm_publish_status(oecm_state *s, const oecm_status *st, oecm_span reply_to);
oecm_err oecm_publish_metadata(oecm_state *s, size_t data, oecm_span reply_to);
/* Data / Config with ReplyTo (n == 0: none) on the own topic. */
oecm_err oecm_publish_data(oecm_state *s, size_t data, oecm_span reply_to);
oecm_err oecm_publish_config(oecm_state *s, size_t ctx, oecm_span reply_to);
unsigned oecm_data_writer(const oecm_state *s, size_t data);
unsigned oecm_config_writer(const oecm_state *s, size_t ctx);
/* `^[0-9]+-<ServiceType>/.+` (REQ-MSG-007), on the JSON string token. */
bool oecm_message_id_ok(oecm_span v);

/* --- Level 3 (oecm_l3.c; MG 8.3.10, 8.4.1, 9.1.10, 9.2.1) --------------- */

/* The DataSetWriterId of the own ReferenceDesignation: after the Config
 * and Data DataSets; NewDataSetWriterId reserves the ids above it. */
unsigned oecm_refdes_writer(const oecm_state *s);
/* The room for the ReferenceDesignation object in an answer on
 * Pub/ReferenceDesignation/<own Source> (oecm_res.c). */
size_t oecm_refdes_room(oecm_state *s);
/* Checks the Level 3 configuration, derives refdes_max and loads the
 * stored ReferenceDesignation and the DataSetWriterId limit. */
oecm_err oecm_l3_init(oecm_state *s);
/* The answer to a Get of the own ReferenceDesignation (REQ-RES-REFDES-012
 * … 015): -1 no answer (an invalid Query), 0 an empty answer, 1 the
 * object. root: the request; with_source: the topic has a Source. */
int oecm_l3_refdes_get(oecm_state *s, oecm_span root, bool with_source);
/* Set with the Payload object of the one DataSetMessage: publishes the
 * object with the ReplyTo and returns the Status (REQ-RES-REFDES-006 …
 * 010). */
const oecm_status *oecm_l3_refdes_set(oecm_state *s, oecm_span payload, oecm_span reply_to);
const oecm_status *oecm_l3_refdes_del(oecm_state *s);
/* Canonical form of a ReferenceDesignation object into out; scratch holds
 * one decoded string. 0: valid; 1: invalid (BadDecodingError); 2: does
 * not fit out (BadOutOfRange). */
int oecm_refdes_encode(oecm_span obj, oecm_buf *out, char *scratch, size_t scratch_cap);
/* RFC 3339 DateTime of the schema (oec-core value.rs). */
bool oecm_date_time_ok(const char *s, size_t n);
/* Call NewDataSetWriterId (MG 9.2.1, REQ-ACT-*); topic is the Call topic. */
void oecm_l3_call(oecm_state *s, const char *topic, const oecm_topic *t, const uint8_t *payload,
                  size_t len);

/* Encodes into tx: the topic, a NUL, then the payload. Returns the payload
 * offset and sets *payload_len; 0 if it did not fit. */
size_t oecm_encode_birth(oecm_state *s, int64_t unix_ms, size_t *payload_len);
size_t oecm_encode_health(oecm_state *s, int64_t unix_ms, oecm_health h, int score,
                          size_t *payload_len);

#endif /* OECM_INTERNAL_H */
