/* The simulated temperature sensor of the micro SDK examples from Level 2
 * (C-094; shared by the host runner and the ESP-IDF firmware since C-096): Data "Temperature" (Value read-only in
 * practice, Setpoint written by Set Data) and the Config context "Sensor"
 * (group "General": Interval of the simulation, Label). Every tick moves
 * the Value a tenth of the way towards the Setpoint. */
#include "oecm_sensor.h"

static double value = 21.0, setpoint = 21.0;
static char label[33] = "Temperature sensor";
static char interval[16] = "5";

static const oecm_field fields[] = {
    {.name = "Value", .description = "Measured temperature", .type = OECM_TYPE_DOUBLE,
     .value = &value, .decimals = 2, .unit = "°C", .eu_low = "-40", .eu_high = "125"},
    {.name = "Setpoint", .description = "Temperature the simulation moves to",
     .type = OECM_TYPE_DOUBLE, .value = &setpoint, .decimals = 1, .unit = "°C",
     .eu_low = "-40", .eu_high = "125"},
    {.name = "Label", .description = "Label of the sensor", .type = OECM_TYPE_STRING,
     .value = label, .cap = sizeof label},
};

static const oecm_data data[] = {{
    .tag = "Temperature",
    .description = "Simulated temperature of the micro SDK example",
    .fields = fields,
    .field_count = sizeof fields / sizeof fields[0],
    .interval_ms = 60000,
    .writable = true,
}};

static const oecm_cfg_object general[] = {
    {.key = "Interval", .name = "Interval", .description = "Seconds between two simulation steps",
     .type = OECM_CFG_NUMBER, .value = interval, .cap = sizeof interval, .default_value = "5",
     .unit = "s", .mandatory = true, .min = "1", .max = "3600"},
    {.key = "Label", .name = "Label", .description = "Label of the sensor",
     .type = OECM_CFG_STRING, .value = label, .cap = sizeof label, .default_value = "Temperature sensor",
     .length = 32},
};

static const oecm_cfg_group groups[] = {{
    .key = "General",
    .name = "General",
    .objects = general,
    .count = sizeof general / sizeof general[0],
}};

static const oecm_cfg_context contexts[] = {{
    .name = "Sensor",
    .title = "Sensor",
    .description = "Settings of the simulated sensor",
    .groups = groups,
    .count = 1,
}};

static oecm_device *device;

/* A changed Label is also a field of the Data (shared buffer). */
static void on_config(void *ctx, size_t context) {
    (void)ctx;
    (void)context;
    (void)oecm_data_changed(device, 0);
}

void oecm_sensor_setup(oecm_config *cfg, oecm_device *dev) {
    device = dev;
    cfg->data = data;
    cfg->data_count = 1;
    cfg->config = contexts;
    cfg->config_count = 1;
    cfg->on_config = on_config;
}

unsigned oecm_sensor_interval_s(void) {
    unsigned s = 0;
    for (const char *p = interval; *p >= '0' && *p <= '9'; p++)
        s = s * 10u + (unsigned)(*p - '0');
    return s == 0 ? 5u : s;
}

void oecm_sensor_tick(void) {
    double next = value + (setpoint - value) / 10.0;
    if (next - setpoint < 0.005 && setpoint - next < 0.005)
        next = setpoint;
    if (next == value)
        return;
    value = next;
    (void)oecm_data_changed(device, 0);
}

double oecm_sensor_value(void) { return value; }

double oecm_sensor_setpoint(void) { return setpoint; }
