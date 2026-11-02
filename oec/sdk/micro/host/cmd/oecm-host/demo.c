/* The application of the host runner from Level 2 (ADR-0030 §7, C-094):
 * a simulated temperature sensor. Data "Temperature" (Value read-only in
 * practice, Setpoint written by Set Data) and the Config context "Sensor"
 * (group "General": Interval of the simulation, Label). Every tick moves
 * the Value a tenth of the way towards the Setpoint. */
#include "oecm.h"

static double value = 21.0, setpoint = 21.0;
static char label[33] = "Host sensor";
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
    .description = "Simulated temperature of the micro SDK host runner",
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
     .type = OECM_CFG_STRING, .value = label, .cap = sizeof label, .default_value = "Host sensor",
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

void oecm_host_demo(oecm_config *cfg, oecm_device *dev);
void oecm_host_demo(oecm_config *cfg, oecm_device *dev) {
    device = dev;
    cfg->data = data;
    cfg->data_count = 1;
    cfg->config = contexts;
    cfg->config_count = 1;
    cfg->on_config = on_config;
}

/* Seconds between two steps (the Config value, 1 … 3600). */
unsigned oecm_host_demo_interval(void);
unsigned oecm_host_demo_interval(void) {
    unsigned s = 0;
    for (const char *p = interval; *p >= '0' && *p <= '9'; p++)
        s = s * 10u + (unsigned)(*p - '0');
    return s == 0 ? 5u : s;
}

/* One simulation step; publishes the Data if the Value changed. */
void oecm_host_demo_tick(void);
void oecm_host_demo_tick(void) {
    double next = value + (setpoint - value) / 10.0;
    if (next - setpoint < 0.005 && setpoint - next < 0.005)
        next = setpoint;
    if (next == value)
        return;
    value = next;
    (void)oecm_data_changed(device, 0);
}
