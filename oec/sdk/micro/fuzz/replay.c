/* Replays a corpus directory through a fuzz target (ADR-0030 §6): the
 * seeds run under ASan/UBSan in ctest without libFuzzer. Usage:
 * fuzz_X_replay DIR... */
#include <dirent.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int LLVMFuzzerTestOneInput(const uint8_t *data, size_t size);

static int replay(const char *path) {
    FILE *f = fopen(path, "rb");
    if (!f)
        return 1;
    static uint8_t buf[1 << 16];
    size_t n = fread(buf, 1, sizeof buf, f);
    int bad = ferror(f) || !feof(f);
    fclose(f);
    if (bad) {
        fprintf(stderr, "%s: read error or larger than %zu octets\n", path, sizeof buf);
        return 1;
    }
    /* a copy of exactly n octets, so ASan sees reads past the input */
    uint8_t *p = malloc(n ? n : 1);
    if (!p)
        return 1;
    memcpy(p, buf, n);
    LLVMFuzzerTestOneInput(p, n);
    free(p);
    return 0;
}

int main(int argc, char **argv) {
    int files = 0, rc = 0;
    for (int i = 1; i < argc; i++) {
        DIR *d = opendir(argv[i]);
        if (!d) {
            perror(argv[i]);
            return 1;
        }
        struct dirent *e;
        while ((e = readdir(d)) != NULL) {
            if (e->d_name[0] == '.')
                continue;
            char path[4096];
            snprintf(path, sizeof path, "%s/%s", argv[i], e->d_name);
            rc |= replay(path);
            files++;
        }
        closedir(d);
    }
    printf("%d input(s) replayed\n", files);
    return rc || files == 0;
}
