/* The history of the logger demo (A-303, ADR-0028): a ring buffer of the
 * last records (as JSON) and of the numeric values of Data and Health
 * (time series per AppId, Source, Filter and field) for the Web UI of
 * increment 5. Thread-safe: the SDK thread adds, other threads read. */
#ifndef LOGGER_HISTORY_H
#define LOGGER_HISTORY_H

#include <stdint.h>

#include "core.h"

/* Points per series and the maximum number of series. */
#define LG_SERIES_POINTS 240
#define LG_MAX_SERIES 256

typedef struct lg_history lg_history;

/* A history of capacity records (LG_HISTORY_MIN … LG_HISTORY_MAX); NULL
 * if out of memory. */
lg_history *lg_history_new(unsigned capacity);
void lg_history_free(lg_history *h);

/* Changes the capacity and keeps the newest records; false if out of
 * memory (the history is unchanged). */
bool lg_history_resize(lg_history *h, unsigned capacity);

/* Adds the record of a logged message (JSON, as lg_write_record writes
 * it) and its numeric values: the top-level numbers of a Data Payload,
 * HealthScore of a Health Payload. */
void lg_history_add(lg_history *h, const lg_msg *m, int level);

/* The number of records held, the number of series and the series that
 * found no place (LG_MAX_SERIES). */
unsigned lg_history_count(lg_history *h);
unsigned lg_history_series_count(lg_history *h);
uint64_t lg_history_series_dropped(lg_history *h);

/* Calls f for every record, the oldest first; seq counts from 1 since the
 * start. Under the lock: f must not call back. */
typedef void (*lg_record_fn)(void *user_data, uint64_t seq, const char *record_json);
void lg_history_each(lg_history *h, lg_record_fn f, void *user_data);

/* One point of a series. */
typedef struct lg_point {
    int64_t time_ms;
    double value;
} lg_point;

/* A series: the topic parts (source and filter may be NULL), the field and
 * its points, the oldest first. */
typedef struct lg_series_view {
    lg_resource resource;
    const char *app;
    const char *source;
    const char *filter;
    const char *field;
    const lg_point *points;
    unsigned n;
} lg_series_view;

/* Calls f for every series in the order of their first value. Under the
 * lock: f must not call back. */
typedef void (*lg_series_fn)(void *user_data, const lg_series_view *s);
void lg_history_series_each(lg_history *h, lg_series_fn f, void *user_data);

#endif
