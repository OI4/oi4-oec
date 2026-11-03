/*
 * OECMicro.h - the entry header of the OEC Micro SDK as an Arduino library
 * (ADR-0024, C-097): the core API (oecm.h) and the ESP-IDF port
 * (oecm_esp.h) on the esp-mqtt and NVS of the Arduino-ESP32 core.
 *
 * The sketch owns the network (WiFi.begin) and the time (configTime): the
 * core needs a plausible wall clock before oecm_start (REQ-MSG-007).
 * nvs_flash_init is done by the Arduino-ESP32 core.
 */
#ifndef OECMICRO_H
#define OECMICRO_H

#include "oecm.h"
#include "oecm_esp.h"

#endif /* OECMICRO_H */
