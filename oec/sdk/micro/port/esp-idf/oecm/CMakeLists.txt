# The OEC Micro SDK as an ESP-IDF component (C-096): the portable core of
# src/ and the ESP-IDF PAL (oecm_esp.c, esp-mqtt + NVS).
file(GLOB OECM_CORE ${CMAKE_CURRENT_LIST_DIR}/../../../src/*.c)
idf_component_register(SRCS ${OECM_CORE} "oecm_esp.c"
    INCLUDE_DIRS "../../../include" "include"
    PRIV_INCLUDE_DIRS "../../../src"
    REQUIRES mqtt
    PRIV_REQUIRES nvs_flash esp_timer log esp_ringbuf)
