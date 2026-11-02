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

void oecm_buf_init(oecm_buf *b, uint8_t *p, size_t cap);
void oecm_put(oecm_buf *b, const void *s, size_t n);
void oecm_puts(oecm_buf *b, const char *s);
void oecm_putc(oecm_buf *b, char c);
void oecm_put_u64(oecm_buf *b, uint64_t v);
void oecm_put_i64(oecm_buf *b, int64_t v);
void oecm_put_b64url(oecm_buf *b, const uint8_t *s, size_t n);
void oecm_put_dnp(oecm_buf *b, const char *s, size_t n);
/* A JSON string with quotes; s must be valid UTF-8. */
void oecm_put_json_str(oecm_buf *b, const char *s, size_t n);

size_t oecm_b64url_len(size_t n);

/* --- Names ----------------------------------------------------------------- */

const char *oecm_service_type_name(oecm_service_type st);
const char *oecm_health_name(oecm_health h);

/* --- Messages (MG 7, MG 8) ------------------------------------------------- */

#define OECM_CLASS_MAM "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58"
#define OECM_CLASS_HEALTH "d8e7b6df-42ba-448a-975a-199f59e8ffeb"

/* DataSetWriterIds of the device's own publications (ADR-0030 §9). */
#define OECM_WRITER_MAM 10u
#define OECM_WRITER_HEALTH 11u

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
} oecm_state;

/* Encodes into tx: the topic, a NUL, then the payload. Returns the payload
 * offset and sets *payload_len; 0 if it did not fit. */
size_t oecm_encode_birth(oecm_state *s, int64_t unix_ms, size_t *payload_len);
size_t oecm_encode_health(oecm_state *s, int64_t unix_ms, oecm_health h, int score,
                          size_t *payload_len);

#endif /* OECM_INTERNAL_H */
