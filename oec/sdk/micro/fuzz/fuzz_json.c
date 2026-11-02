/* Fuzz target: the JSON reader (oecm_json.c). Every accepted document is
 * walked completely: kinds, members, string decoding, numbers. */
#include <stddef.h>
#include <stdint.h>
#include <string.h>

#include "oecm_internal.h"

int LLVMFuzzerTestOneInput(const uint8_t *data, size_t size);

static char text[256];

static void walk(oecm_span root) {
    /* without recursion, as the reader: a stack of iterators */
    oecm_jit stack[OECM_JSON_DEPTH + 1];
    size_t depth = 0;
    oecm_jkind k = oecm_json_kind(root);
    if (k != OECM_J_OBJECT && k != OECM_J_ARRAY)
        return;
    oecm_json_iter(root, &stack[depth++]);
    while (depth > 0) {
        oecm_span key, v;
        if (!oecm_json_next(&stack[depth - 1], &key, &v)) {
            depth--;
            continue;
        }
        size_t n;
        bool nul;
        if (key.p != NULL) {
            (void)oecm_json_str(key, text, sizeof text, &n);
            (void)oecm_json_str_chars(key);
        }
        switch (oecm_json_kind(v)) {
        case OECM_J_STRING:
            (void)oecm_json_str(v, text, sizeof text, &n);
            (void)oecm_json_str_bytes(v, &nul);
            (void)oecm_json_str_chars(v);
            (void)oecm_json_streq(v, "x");
            break;
        case OECM_J_NUMBER: {
            int64_t i;
            double d;
            (void)oecm_json_i64(v, &i);
            (void)oecm_json_f64(v, &d);
            break;
        }
        case OECM_J_BOOL: {
            bool b;
            (void)oecm_json_bool(v, &b);
            break;
        }
        case OECM_J_OBJECT:
            (void)oecm_json_get(v, "Payload", &key);
            /* fall through */
        case OECM_J_ARRAY:
            if (depth > OECM_JSON_DEPTH)
                __builtin_trap(); /* deeper than the reader accepts */
            oecm_json_iter(v, &stack[depth++]);
            break;
        default:
            break;
        }
    }
}

int LLVMFuzzerTestOneInput(const uint8_t *data, size_t size) {
    oecm_span root;
    if (oecm_json_parse((const char *)data, size, &root))
        walk(root);
    return 0;
}
