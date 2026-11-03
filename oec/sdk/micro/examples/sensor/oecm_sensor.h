/* The simulated temperature sensor of the micro SDK examples (Level 2+):
 * Data "Temperature" (Value, writable Setpoint, Label) and the Config
 * context "Sensor" (Interval, Label). Used by the host runner and the
 * ESP-IDF firmware. */
#ifndef OECM_SENSOR_H
#define OECM_SENSOR_H

#include "oecm.h"

/* Sets the Data and Config definitions of cfg; dev is the device the
 * sensor reports its changes to. */
void oecm_sensor_setup(oecm_config *cfg, oecm_device *dev);
/* Seconds between two simulation steps (the Config value, 1 … 3600). */
unsigned oecm_sensor_interval_s(void);
/* One simulation step; marks the Data changed if the Value changed. */
void oecm_sensor_tick(void);

#endif
