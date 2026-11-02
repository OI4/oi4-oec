/* Fuzz target: the topic parser (oecm_topic.c). The parts of an accepted
 * topic are non-empty levels inside the input, without wildcards. */
#include <stddef.h>
#include <stdint.h>
#include <string.h>

#include "oecm_internal.h"

int LLVMFuzzerTestOneInput(const uint8_t *data, size_t size);

static void inside(oecm_span s, const char *t, size_t n, bool required) {
    if (s.n == 0) {
        if (required)
            __builtin_trap();
        return;
    }
    if (s.p < t || s.p + s.n > t + n)
        __builtin_trap();
    for (size_t i = 0; i < s.n; i++)
        if (s.p[i] == '+' || s.p[i] == '#' || s.p[i] == '\0')
            __builtin_trap();
}

int LLVMFuzzerTestOneInput(const uint8_t *data, size_t size) {
    const char *t = (const char *)data;
    oecm_topic p;
    if (!oecm_topic_parse(t, size, &p))
        return 0;
    inside(p.st, t, size, true);
    inside(p.app, t, size, true);
    inside(p.method, t, size, true);
    inside(p.resource, t, size, true);
    inside(p.source, t, size, false);
    inside(p.filter, t, size, false);
    if (memchr(p.app.p, '/', p.app.n) || (p.source.n && memchr(p.source.p, '/', p.source.n)))
        __builtin_trap();
    (void)oecm_b64url_eq(p.app, "provider.com/S/1", 16);
    (void)oecm_b64url_eq(p.source, t, size < 64 ? size : 64);
    return 0;
}
